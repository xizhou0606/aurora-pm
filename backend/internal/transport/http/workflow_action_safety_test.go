package httpapi

import (
	"encoding/json"
	"time"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestWorkflowActionWithoutExecutableEdgeFailsClosed(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	invalidDefinition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "end", "kind": "no"},
		},
	}
	rejected := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "错误动作流", "subject_type": "requirement", "definition": invalidDefinition,
	})
	requireStatus(t, rejected, http.StatusUnprocessableEntity)

	validDefinition := map[string]any{
		"nodes": invalidDefinition["nodes"],
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "历史动作流", "subject_type": "requirement", "definition": validDefinition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)

	legacyRaw, err := json.Marshal(invalidDefinition)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowTemplateVersion{}).
		Where("workspace_id = ? AND template_id = ?", workspace.ID, templateID).
		Update("definition", string(legacyRaw)).Error; err != nil {
		t.Fatal(err)
	}
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "历史错误路径"})
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	action := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	})
	requireStatus(t, action, http.StatusConflict)

	var instance workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "running" || !containsString(currentWorkflowNodes(instance.CurrentNodeIDs), "input") {
		t.Fatalf("invalid transition changed instance: %+v", instance)
	}
	var terminalRuns, inputRuns int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "end").Count(&terminalRuns).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "input").Count(&inputRuns).Error; err != nil {
		t.Fatal(err)
	}
	if terminalRuns != 0 || inputRuns != 0 {
		t.Fatalf("invalid transition persisted runs: input=%d end=%d", inputRuns, terminalRuns)
	}
}

func TestWorkflowConditionWithoutMatchingCELPathFailsClosed(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "choice", "type": "condition", "name": "分支选择"},
			{"id": "left", "type": "human_task", "name": "左分支"},
			{"id": "right", "type": "human_task", "name": "右分支"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "choice"},
			{"id": "e2", "source": "choice", "target": "left", "condition": "context.decision == true"},
			{"id": "e3", "source": "choice", "target": "right", "condition": "context.decision == true"},
			{"id": "e4", "source": "left", "target": "end"},
			{"id": "e5", "source": "right", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "CEL 无匹配分支", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "CEL 无匹配"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	result := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "condition_no", "node_id": "choice",
	})
	requireStatus(t, result, http.StatusConflict)
	var instance workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "running" || !containsString(currentWorkflowNodes(instance.CurrentNodeIDs), "choice") {
		t.Fatalf("unmatched condition changed instance: %+v", instance)
	}
	var choiceRuns int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "choice").Count(&choiceRuns).Error; err != nil || choiceRuns != 0 {
		t.Fatalf("unmatched condition runs=%d err=%v", choiceRuns, err)
	}
}

func TestWorkflowNodeActionClaimsActiveTokenOnce(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "并发审批"})
	instanceID := uint(created["workflow_instance_id"].(float64))

	if err := app.db.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").
		Update("status", "processing").Error; err != nil {
		t.Fatal(err)
	}
	colliding := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "重复请求",
	})
	requireStatus(t, colliding, http.StatusConflict)
	if err := app.db.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "processing").
		Update("status", "active").Error; err != nil {
		t.Fatal(err)
	}

	approved := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "通过",
	})
	requireStatus(t, approved, http.StatusOK)
	repeated := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "再次通过",
	})
	requireStatus(t, repeated, http.StatusConflict)

	var runs, approvals int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "review").Count(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowApproval{}).Where("workspace_id = ? AND instance_id = ?", workspace.ID, instanceID).Count(&approvals).Error; err != nil {
		t.Fatal(err)
	}
	if runs != 1 || approvals != 1 {
		t.Fatalf("duplicate action rows: runs=%d approvals=%d", runs, approvals)
	}
}

func TestRequirementWithWorkflowHistoryCanBeDeleted(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	requirementTemplateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, requirementTemplateID, map[string]any{"title": "带流程历史的需求"})
	requirementID := uint(requirement["id"].(float64))
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.request(t, http.MethodDelete, "/api/v1/platform/product-requirements-new/"+uintString(requirementID), token, workspace.ID, nil), http.StatusOK)
	var requirementCount, instanceCount int64
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).Count(&requirementCount).Error; err != nil || requirementCount != 0 {
		t.Fatalf("requirement count=%d err=%v", requirementCount, err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).Count(&instanceCount).Error; err != nil || instanceCount != 0 {
		t.Fatalf("workflow instance count=%d err=%v", instanceCount, err)
	}
}

func TestPlanUnitWithConfiguredWorkflowCanBeDeleted(t *testing.T) {
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	planTemplateID := createPublishedPlanUnitWorkflow(t, app, token, workspace.ID)
	createdUnit := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "仅配置流程的版本", "type": "version", "workflow_template_id": planTemplateID,
	})
	requireStatus(t, createdUnit, http.StatusCreated)
	data := decodeData(t, createdUnit).(map[string]any)
	unitID := uint(data["id"].(float64))
	instanceID := uint(data["workflow_instance_id"].(float64))

	requireStatus(t, app.request(t, http.MethodDelete, "/api/v1/platform/project-new/"+uintString(unitID), token, workspace.ID, nil), http.StatusOK)
	for name, check := range map[string]struct {
		model any
		where string
		args  []any
	}{
		"plan unit":         {&projectgorm.PlanUnit{}, "id = ? AND workspace_id = ?", []any{unitID, workspace.ID}},
		"workflow instance": {&workflowgorm.WorkflowInstance{}, "id = ? AND workspace_id = ?", []any{instanceID, workspace.ID}},
		"workflow token":    {&workflowgorm.WorkflowToken{}, "instance_id = ? AND workspace_id = ?", []any{instanceID, workspace.ID}},
		"workflow run":      {&workflowgorm.WorkflowNodeRun{}, "instance_id = ? AND workspace_id = ?", []any{instanceID, workspace.ID}},
	} {
		var count int64
		if err := app.db.Model(check.model).Where(check.where, check.args...).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s count=%d err=%v", name, count, err)
		}
	}
}

func TestPlanUnitWithExecutedWorkflowCannotBeDeleted(t *testing.T) {
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	planTemplateID := createPublishedPlanUnitWorkflow(t, app, token, workspace.ID)
	createdUnit := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "已有流程执行历史的版本", "type": "version", "workflow_template_id": planTemplateID,
	})
	requireStatus(t, createdUnit, http.StatusCreated)
	data := decodeData(t, createdUnit).(map[string]any)
	unitID := uint(data["id"].(float64))
	instanceID := uint(data["workflow_instance_id"].(float64))
	now := time.Now().UTC()
	if err := app.db.Create(&workflowgorm.WorkflowNodeRun{
		WorkspaceID: workspace.ID, InstanceID: instanceID,
		NodeID: "delivery", NodeType: "human_task", Status: "completed",
		StartedAt: now, CompletedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodDelete, "/api/v1/platform/project-new/"+uintString(unitID), token, workspace.ID, nil), http.StatusConflict)
	var unitCount int64
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ? AND workspace_id = ?", unitID, workspace.ID).Count(&unitCount).Error; err != nil || unitCount != 1 {
		t.Fatalf("plan unit count=%d err=%v", unitCount, err)
	}
}

func TestOrphanWorkflowInstanceFailsClosed(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "模拟历史孤儿流程"})
	requirementID := uint(requirement["id"].(float64))
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if err := app.db.Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).Delete(&requirementgorm.Requirement{}).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil), http.StatusConflict)
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "approve", "node_id": "review",
	}), http.StatusConflict)
	var reviewRuns int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "review").Count(&reviewRuns).Error; err != nil || reviewRuns != 0 {
		t.Fatalf("orphan workflow review runs=%d err=%v", reviewRuns, err)
	}
}
