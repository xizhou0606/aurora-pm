package httpapi

import (
	"encoding/json"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestApprovalRejectReturnsToPreviousAIAndReusesResultUntilManualRecheck(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "ai", "type": "ai_check", "name": "完整性检查", "capability": "requirement.completeness"},
			{"id": "review", "type": "approval", "name": "需求预审审批", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "ai"},
			{"id": "e3", "source": "ai", "target": "review", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "input", "kind": "failure", "loop": true},
			{"id": "e5", "source": "review", "target": "end", "kind": "approve"},
			// Compatibility metadata may still point farther back; runtime reject
			// returns to the actual previous business node instead.
			{"id": "e6", "source": "review", "target": "input", "kind": "reject", "loop": true},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "驳回复用 AI 结果", definition)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "结算页需求"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requirementID := uint(requirement["id"].(float64))

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	var firstRun aigorm.AIRun
	if err := app.db.Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?",
		workspace.ID, "requirement", requirementID, "requirement.completeness",
	).Order("id DESC").First(&firstRun).Error; err != nil {
		t.Fatal(err)
	}
	output, _ := json.Marshal(workflowCompletenessOutput(true, firstRun.SubjectVersion, 90))
	if err := app.db.Model(&firstRun).Updates(map[string]any{"status": "completed", "output_json": string(output)}).Error; err != nil {
		t.Fatal(err)
	}
	advanced := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "ai",
	})
	requireStatus(t, advanced, http.StatusOK)
	advancedData := decodeData(t, advanced).(map[string]any)
	assertWorkflowCurrentNode(t, advancedData, "review")
	var aiTransition map[string]any
	for _, raw := range advancedData["transition_history"].([]any) {
		item := raw.(map[string]any)
		if item["node_id"] == "ai" && item["action"] == "ai_completed" {
			aiTransition = item
		}
	}
	if aiTransition == nil || aiTransition["target_node_id"] != "review" || aiTransition["target_node_name"] != "需求预审审批" {
		t.Fatalf("AI transition target contract=%v", aiTransition)
	}

	rejected := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "reject", "node_id": "review", "comment": "请重新确认完整性报告",
	})
	requireStatus(t, rejected, http.StatusOK)
	rejectedData := decodeData(t, rejected).(map[string]any)
	assertWorkflowCurrentNode(t, rejectedData, "ai")
	aiNode := workflowTimelineNode(t, rejectedData, "ai")
	execution, ok := aiNode["ai_execution"].(map[string]any)
	if !ok || execution["run_id"] != float64(firstRun.ID) || execution["status"] != "completed" {
		t.Fatalf("returned AI node did not expose previous result: %v", aiNode)
	}
	if !containsString(actionStrings(aiNode["allowed_actions"]), "complete") || !containsString(actionStrings(aiNode["allowed_actions"]), "recheck_ai") {
		t.Fatalf("returned AI actions=%v", aiNode["allowed_actions"])
	}
	assertWorkflowAIRunCount(t, app, workspace.ID, requirementID, 1)
	var waiting int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		workspace.ID, instanceID, "ai", "waiting",
	).Count(&waiting).Error; err != nil || waiting != 0 {
		t.Fatalf("reject automatically queued AI: waiting=%d err=%v", waiting, err)
	}
	reused := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "ai",
	})
	requireStatus(t, reused, http.StatusOK)
	assertWorkflowCurrentNode(t, decodeData(t, reused).(map[string]any), "review")
	assertWorkflowAIRunCount(t, app, workspace.ID, requirementID, 1)

	rejectedAgain := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "reject", "node_id": "review", "comment": "再次确认报告",
	})
	requireStatus(t, rejectedAgain, http.StatusOK)
	assertWorkflowCurrentNode(t, decodeData(t, rejectedAgain).(map[string]any), "ai")
	assertWorkflowAIRunCount(t, app, workspace.ID, requirementID, 1)

	rechecked := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "recheck_ai", "node_id": "ai",
	})
	requireStatus(t, rechecked, http.StatusOK)
	recheckedNode := workflowTimelineNode(t, decodeData(t, rechecked).(map[string]any), "ai")
	recheckedExecution, ok := recheckedNode["ai_execution"].(map[string]any)
	if !ok || recheckedExecution["status"] != "queued" || recheckedExecution["run_id"] == float64(firstRun.ID) {
		t.Fatalf("manual recheck did not queue a fresh result: %v", recheckedNode)
	}
	assertWorkflowAIRunCount(t, app, workspace.ID, requirementID, 2)
	var activeToken workflowgorm.WorkflowToken
	if err := app.db.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ?",
		workspace.ID, instanceID, "ai",
	).Order("id DESC").First(&activeToken).Error; err != nil || activeToken.Status != "active" {
		t.Fatalf("manual recheck did not restore active token: token=%+v err=%v", activeToken, err)
	}
}

func assertWorkflowAIRunCount(t *testing.T, app *testApp, workspaceID, requirementID uint, want int64) {
	t.Helper()
	var count int64
	if err := app.db.Model(&aigorm.AIRun{}).Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?",
		workspaceID, "requirement", requirementID, "requirement.completeness",
	).Count(&count).Error; err != nil || count != want {
		t.Fatalf("AI run count=%d want=%d err=%v", count, want, err)
	}
}
