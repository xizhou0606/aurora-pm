package httpapi

import (
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestRequirementWorkflowNodePlansBatchUpdatesDatesAndAssignees(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	reviewer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reviewer")
	manager := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 3, "manager")
	managerToken := app.token(t, manager)
	reviewerToken := app.token(t, reviewer)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "录入"},
			{"id": "review", "type": "approval", "name": "评审"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "批量分工需求流程", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "批量分工需求"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"

	updated := app.request(t, http.MethodPatch, path, managerToken, workspace.ID, map[string]any{
		"confirm_subject_schedule": true,
		"nodes": []map[string]any{
			{"node_id": "input", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-12", "expected_planned_end_at": nil, "assignee_user_id": reviewer.ID},
			{"node_id": "review", "planned_start_at": "2026-08-13", "expected_planned_start_at": nil, "planned_end_at": "2026-08-14", "expected_planned_end_at": nil, "assignee_user_id": uint(1)},
		},
	})
	requireStatus(t, updated, http.StatusOK)
	data := decodeData(t, updated).(map[string]any)
	if data["can_manage_node_plans"] != true {
		t.Fatalf("batch response omitted management capability: %v", data)
	}
	plans := data["node_plans"].(map[string]any)
	if plans["input"].(map[string]any)["assignee_user_id"] != float64(reviewer.ID) ||
		plans["review"].(map[string]any)["assignee_user_id"] != float64(1) {
		t.Fatalf("batch assignee overrides=%v", plans)
	}
	assertWorkflowTimelineAssignee(t, data["timeline_nodes"].([]any), "input", reviewer.ID, reviewer.DisplayName)
	assertWorkflowTimelineAssignee(t, data["timeline_nodes"].([]any), "review", 1, "owner")

	var inputToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "active").First(&inputToken).Error; err != nil {
		t.Fatal(err)
	}
	if inputToken.AssigneeUserID == nil || *inputToken.AssigneeUserID != reviewer.ID {
		t.Fatalf("active token not reassigned atomically: %+v", inputToken)
	}
	requireStatus(t, app.workflowAction(t, workspace.ID, ownerToken, instanceID, map[string]any{"action": "complete", "node_id": "input"}), http.StatusOK)
	requireStatus(t, app.workflowAction(t, workspace.ID, reviewerToken, instanceID, map[string]any{"action": "complete", "node_id": "input"}), http.StatusConflict)
	requireStatus(t, app.workflowAction(t, workspace.ID, reviewerToken, instanceID, map[string]any{"action": "approve", "node_id": "review"}), http.StatusOK)
	requireStatus(t, app.workflowAction(t, workspace.ID, ownerToken, instanceID, map[string]any{"action": "approve", "node_id": "review"}), http.StatusConflict)
}

func TestPlanUnitWorkflowNodePlansBatchUpdatesActiveAssignee(t *testing.T) {
	app, ownerToken, workspace, project, unit := newAIWorkflowFixture(t)
	deliveryOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "delivery-owner")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "delivery", "type": "human_task", "name": "交付"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{{"id": "e1", "source": "start", "target": "delivery"}, {"id": "e2", "source": "delivery", "target": "end"}},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", ownerToken, workspace.ID, map[string]any{
		"name": "批量分工项目流程", "subject_type": "plan_unit", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", ownerToken, workspace.ID, nil), http.StatusOK)
	created := app.request(t, http.MethodPost, "/api/v1/platform/project-new", ownerToken, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "批量分工项目", "type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, created, http.StatusCreated)
	instanceID := uint(decodeData(t, created).(map[string]any)["workflow_instance_id"].(float64))
	updated := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans", ownerToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "delivery", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil, "assignee_user_id": deliveryOwner.ID}},
	})
	requireStatus(t, updated, http.StatusOK)
	assertWorkflowTimelineAssignee(t, decodeData(t, updated).(map[string]any)["timeline_nodes"].([]any), "delivery", deliveryOwner.ID, deliveryOwner.DisplayName)
	requireStatus(t, app.workflowAction(t, workspace.ID, app.token(t, deliveryOwner), instanceID, map[string]any{
		"action": "complete", "node_id": "delivery",
	}), http.StatusOK)
}

func TestWorkflowNodePlansBatchRejectsInvalidItemAtomically(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "原子批量排期"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	var beforeInstance workflowgorm.WorkflowInstance
	var beforeToken workflowgorm.WorkflowToken
	if err := app.db.First(&beforeInstance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND status = ?", workspace.ID, instanceID, "active").First(&beforeToken).Error; err != nil {
		t.Fatal(err)
	}

	invalid := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{
			{"node_id": "review", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil, "assignee_user_id": uint(1)},
			{"node_id": "end", "planned_end_at": "2026-08-21", "expected_planned_end_at": nil},
		},
	})
	requireStatus(t, invalid, http.StatusBadRequest)
	var afterInstance workflowgorm.WorkflowInstance
	var afterToken workflowgorm.WorkflowToken
	if err := app.db.First(&afterInstance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&afterToken, beforeToken.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterInstance.NodePlansJSON != beforeInstance.NodePlansJSON || !sameWorkflowAssignee(beforeToken.AssigneeUserID, afterToken.AssigneeUserID) {
		t.Fatalf("invalid batch partially persisted: plans=%q token=%+v", afterInstance.NodePlansJSON, afterToken)
	}

	requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "review"}},
	}), http.StatusBadRequest)
	requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "review", "assignee_user_id": nil}},
	}), http.StatusBadRequest)
}

func TestCompletedNodeAssigneeOverrideOnlyAffectsFutureRollbackToken(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	nextOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "next-owner")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "录入"},
			{"id": "review", "type": "approval", "name": "评审"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "回退分工流程", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "回退后新负责人"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, ownerToken, instanceID, map[string]any{"action": "complete", "node_id": "input"}), http.StatusOK)

	updated := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans", ownerToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "input", "assignee_user_id": nextOwner.ID}},
	})
	requireStatus(t, updated, http.StatusOK)
	var historical workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "completed").Order("id DESC").First(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if historical.AssigneeUserID == nil || *historical.AssigneeUserID != 1 {
		t.Fatalf("completed token history was rewritten: %+v", historical)
	}

	rolledBack := app.workflowAction(t, workspace.ID, ownerToken, instanceID, map[string]any{
		"action": "rollback", "node_id": "review", "target_node_id": "input", "comment": "重新录入",
	})
	requireStatus(t, rolledBack, http.StatusOK)
	var fresh workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "active").Order("id DESC").First(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.AssigneeUserID == nil || *fresh.AssigneeUserID != nextOwner.ID {
		t.Fatalf("rollback token did not inherit instance override: %+v", fresh)
	}
}

func TestWorkflowNodePlanCorruptionFailsClosed(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "损坏排期"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).Update("node_plans_json", "{").Error; err != nil {
		t.Fatal(err)
	}
	response := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans", token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "review", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil}},
	})
	requireStatus(t, response, http.StatusConflict)
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if instance.NodePlansJSON != "{" {
		t.Fatalf("corrupt node plans were silently replaced: %q", instance.NodePlansJSON)
	}
	var active workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND status = ?", workspace.ID, instanceID, "active").First(&active).Error; err != nil {
		t.Fatal(err)
	}
	var version workflowgorm.WorkflowTemplateVersion
	if err := app.db.First(&version, instance.TemplateVersionID).Error; err != nil {
		t.Fatal(err)
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	if len(issues) > 0 {
		t.Fatalf("fixture definition invalid: %v", issues)
	}
	if _, err := workflowTokenEffectiveAssigneeTx(app.db, instance, definition, active); err == nil {
		t.Fatal("corrupt node plans did not fail closed during authorization")
	}
}

func assertWorkflowTimelineAssignee(t *testing.T, nodes []any, nodeID string, userID uint, name string) {
	t.Helper()
	for _, raw := range nodes {
		node := raw.(map[string]any)
		if node["id"] == nodeID {
			if node["assignee_user_id"] != float64(userID) || node["assignee_name"] != name {
				t.Fatalf("node %s assignee=%v want=%d/%s", nodeID, node, userID, name)
			}
			return
		}
	}
	t.Fatalf("timeline node %s not found: %v", nodeID, nodes)
}

func sameWorkflowAssignee(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
