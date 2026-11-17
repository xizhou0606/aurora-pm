package httpapi

import (
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
	"time"
)

func TestWorkflowRejectReturnsToConfiguredNodeAndRecordsActor(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
			{"id": "e4", "source": "review", "target": "input", "kind": "reject", "loop": true},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "可驳回需求流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)

	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "驳回回退需求"})
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "advance", "node_id": "input", "comment": "录入信息已补齐",
	}), http.StatusOK)
	rejected := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "reject", "node_id": "review", "comment": "请补充边界条件",
	})
	requireStatus(t, rejected, http.StatusOK)
	rejectedData := decodeData(t, rejected).(map[string]any)
	if rejectedData["status"] != "running" {
		t.Fatalf("reject terminated workflow: %v", rejectedData["status"])
	}
	current := rejectedData["current_nodes"].([]any)
	if len(current) != 1 || current[0].(map[string]any)["id"] != "input" {
		t.Fatalf("reject did not follow configured edge: %v", current)
	}

	loaded := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, loaded, http.StatusOK)
	history := decodeData(t, loaded).(map[string]any)["transition_history"].([]any)
	if len(history) != 3 {
		t.Fatalf("transition history length=%d: %v", len(history), history)
	}
	inputEntry := history[1].(map[string]any)
	if inputEntry["node_id"] != "input" || inputEntry["operator_name"] != "owner" || inputEntry["comment"] != "录入信息已补齐" || inputEntry["target_node_id"] != "review" || inputEntry["target_node_name"] != "产品评审" {
		t.Fatalf("human/form completion history contract=%v", inputEntry)
	}
	last := history[len(history)-1].(map[string]any)
	if last["node_name"] != "产品评审" || last["action"] != "rejected" || last["status"] != "rejected" || last["operator_name"] != "owner" || last["comment"] != "请补充边界条件" || last["target_node_id"] != "input" || last["target_node_name"] != "需求录入" {
		t.Fatalf("rejection history contract=%v", last)
	}
}

func TestLegacyRejectedWorkflowCanResumeAlongPublishedRejectPath(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
			{"id": "e4", "source": "review", "target": "input", "kind": "reject", "loop": true},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "历史恢复需求流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "历史驳回恢复需求"})
	requirementID := uint(createdRequirement["id"].(float64))
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "advance", "node_id": "input",
	}), http.StatusOK)

	// Reproduce the state written by the legacy reject implementation: the rejection
	// history exists, but the instance is terminal and has no active node.
	now := time.Now().UTC()
	legacyRun := workflowgorm.WorkflowNodeRun{
		WorkspaceID: workspace.ID, InstanceID: instanceID, NodeID: "review", NodeType: "approval",
		Status: "rejected", OutputJSON: encodeWorkflowRunActor(1), StartedAt: now, CompletedAt: &now,
	}
	if err := app.db.Create(&legacyRun).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").
		Update("status", "rejected").Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("workspace_id = ? AND id = ?", workspace.ID, instanceID).
		Updates(map[string]any{"status": "rejected", "current_node_ids": "[]", "completed_at": now}).Error; err != nil {
		t.Fatal(err)
	}

	resumed := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "resume"})
	requireStatus(t, resumed, http.StatusOK)
	data := decodeData(t, resumed).(map[string]any)
	if data["status"] != "running" || data["completed_at"] != nil {
		t.Fatalf("legacy instance was not reopened: %v", data)
	}
	current := data["current_nodes"].([]any)
	if len(current) != 1 || current[0].(map[string]any)["id"] != "input" {
		t.Fatalf("legacy instance did not resume to reject target: %v", current)
	}
	var activeTokens int64
	if err := app.db.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "active").
		Count(&activeTokens).Error; err != nil || activeTokens != 1 {
		t.Fatalf("active return token count=%d err=%v", activeTokens, err)
	}
	var restored requirementgorm.Requirement
	if err := app.db.Where("workspace_id = ? AND id = ?", workspace.ID, requirementID).First(&restored).Error; err != nil {
		t.Fatal(err)
	}
	if restored.Status != "backlog" {
		t.Fatalf("requirement status after resume=%q", restored.Status)
	}
	var rejectionRuns int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).
		Where("workspace_id = ? AND instance_id = ? AND status = ?", workspace.ID, instanceID, "rejected").
		Count(&rejectionRuns).Error; err != nil || rejectionRuns != 1 {
		t.Fatalf("rejection history was changed: count=%d err=%v", rejectionRuns, err)
	}
	repeated := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "resume"})
	requireStatus(t, repeated, http.StatusConflict)
}

func TestPlanUnitWorkflowAINodeQueuesRun(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "split", "type": "parallel_split", "name": "执行阶段并行分支"},
			{"id": "develop", "type": "human_task", "name": "研发执行"},
			{"id": "risk", "type": "ai_check", "name": "AI 风险智能巡检", "capability": "delivery.risk_monitor"},
			{"id": "handle", "type": "human_task", "name": "风险应对"},
			{"id": "join", "type": "parallel_join", "name": "执行阶段汇聚"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "split", "kind": "approve"},
			{"id": "e4", "source": "split", "target": "develop", "loop": true},
			{"id": "e5", "source": "split", "target": "risk", "loop": true},
			{"id": "e6", "source": "risk", "target": "handle", "kind": "success", "loop": true},
			{"id": "e7", "source": "risk", "target": "handle", "kind": "failure", "loop": true},
			{"id": "e8", "source": "develop", "target": "join", "loop": true},
			{"id": "e9", "source": "handle", "target": "join", "loop": true},
			{"id": "e10", "source": "join", "target": "end", "loop": true},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "项目风险流", "subject_type": "plan_unit", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	started := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": unit.ProjectSetID, "title": "AI 风险项目", "type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, started, http.StatusCreated)
	startedData := decodeData(t, started).(map[string]any)
	unit.ID = uint(startedData["id"].(float64))
	instanceID := uint(startedData["workflow_instance_id"].(float64))
	// 需求录入完成现在要求范围非空（空范围防呆），先录入一条需求。
	entered := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: unit.TeamID, ProjectSetID: unit.ProjectSetID,
		ScheduledPlanUnitID: &unit.ID, Title: "已录入需求", Status: "backlog", Version: 1,
	}
	if err := app.db.Create(&entered).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "advance", "node_id": "input", "confirm_requirement_lock": true}), http.StatusOK)
	split := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "approve", "node_id": "review"})
	requireStatus(t, split, http.StatusOK)
	current := decodeData(t, split).(map[string]any)["current_node_ids"].([]any)
	if len(current) != 2 {
		t.Fatalf("parallel split current nodes=%v", current)
	}
	// Entering an AI node now creates the run immediately. A second action while
	// that run is pending must not enqueue a duplicate or advance the workflow.
	data := decodeData(t, split).(map[string]any)
	if data["status"] != "running" {
		t.Fatalf("AI queue changed instance status: %v", data)
	}
	var runs int64
	if err := app.db.Model(&aigorm.AIRun{}).Where("workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?", workspace.ID, "plan_unit", unit.ID, "delivery.risk_monitor").Count(&runs).Error; err != nil || runs != 1 {
		t.Fatalf("plan-unit AI run count=%d err=%v", runs, err)
	}
	var first aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "plan_unit", unit.ID).Order("id DESC").First(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&first).Updates(map[string]any{"status": "failed", "error_message": "provider format error"}).Error; err != nil {
		t.Fatal(err)
	}
	retried := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "retry_ai", "node_id": "risk",
	})
	requireStatus(t, retried, http.StatusOK)
	if err := app.db.Model(&aigorm.AIRun{}).Where("workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?", workspace.ID, "plan_unit", unit.ID, "delivery.risk_monitor").Count(&runs).Error; err != nil || runs != 2 {
		t.Fatalf("plan-unit AI retry run count=%d err=%v", runs, err)
	}
	retryCurrent := decodeData(t, retried).(map[string]any)["current_node_ids"].([]any)
	if !containsAny(retryCurrent, "risk") {
		t.Fatalf("AI retry moved the workflow token: %v", retryCurrent)
	}
	var second aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "plan_unit", unit.ID).Order("id DESC").First(&second).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&second).Updates(map[string]any{"status": "completed", "output_json": `{"summary":"ok"}`}).Error; err != nil {
		t.Fatal(err)
	}
	advanced := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "risk",
	})
	requireStatus(t, advanced, http.StatusOK)
	advancedCurrent := decodeData(t, advanced).(map[string]any)["current_node_ids"].([]any)
	if containsAny(advancedCurrent, "risk") {
		t.Fatalf("completed AI node remained active: %v", advancedCurrent)
	}
	rechecked := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "recheck_ai", "node_id": "risk",
	})
	requireStatus(t, rechecked, http.StatusOK)
	if err := app.db.Model(&aigorm.AIRun{}).Where("workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?", workspace.ID, "plan_unit", unit.ID, "delivery.risk_monitor").Count(&runs).Error; err != nil || runs != 3 {
		t.Fatalf("historical AI recheck run count=%d err=%v", runs, err)
	}
	recheckedCurrent := decodeData(t, rechecked).(map[string]any)["current_node_ids"].([]any)
	if containsAny(recheckedCurrent, "risk") || len(recheckedCurrent) != len(advancedCurrent) {
		t.Fatalf("historical AI recheck changed workflow tokens: before=%v after=%v", advancedCurrent, recheckedCurrent)
	}
	joined := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "handle",
	})
	requireStatus(t, joined, http.StatusOK)
	joinedData := decodeData(t, joined).(map[string]any)
	if !containsAny(joinedData["current_node_ids"].([]any), "join") || !containsAny(joinedData["current_node_ids"].([]any), "develop") {
		t.Fatalf("parallel join did not wait for remaining branch: %v", joinedData["current_node_ids"])
	}
	for _, raw := range joinedData["timeline_nodes"].([]any) {
		node := raw.(map[string]any)
		if node["id"] == "join" && (node["status"] != "waiting" || node["parallel_completed"] != float64(1) || node["parallel_total"] != float64(2)) {
			t.Fatalf("parallel join progress=%v", node)
		}
	}
	finished := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "develop",
	})
	requireStatus(t, finished, http.StatusOK)
	finishedData := decodeData(t, finished).(map[string]any)
	if finishedData["status"] != "completed" || len(finishedData["current_node_ids"].([]any)) != 0 {
		t.Fatalf("parallel join did not auto-complete: %v", finishedData)
	}
}

func containsAny(values []any, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
