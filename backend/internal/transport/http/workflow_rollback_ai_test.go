package httpapi

import (
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestRollbackStillCancelsAffectedActiveAIExecution(t *testing.T) {
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "录入"},
			{"id": "ai", "type": "ai_check", "name": "AI 风险检查", "capability": "delivery.risk_monitor"},
			{"id": "recovery", "type": "human_task", "name": "风险处理"},
			{"id": "review", "type": "approval", "name": "评审"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "ai"},
			{"id": "e3", "source": "ai", "target": "review", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "recovery", "kind": "failure"},
			{"id": "e5", "source": "recovery", "target": "review"},
			{"id": "e6", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "AI 回退取消", "subject_type": "plan_unit", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdUnit := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "AI 回退取消", "type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, createdUnit, http.StatusCreated)
	data := decodeData(t, createdUnit).(map[string]any)
	instanceID := uint(data["workflow_instance_id"].(float64))
	subjectID := uint(data["id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	var aiRun aigorm.AIRun
	var nodeRun workflowgorm.WorkflowNodeRun
	var job workflowgorm.WorkflowJob
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "plan_unit", subjectID).Order("id DESC").First(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "ai", "waiting").First(&nodeRun).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_run_id = ?", workspace.ID, instanceID, nodeRun.ID).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	rolledBack := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "rollback", "node_id": "ai", "target_node_id": "input", "comment": "输入需重做",
	})
	requireStatus(t, rolledBack, http.StatusOK)
	assertWorkflowCurrentNode(t, decodeData(t, rolledBack).(map[string]any), "input")
	if err := app.db.First(&aiRun, aiRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&nodeRun, nodeRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&job, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if aiRun.Status != "cancelled" || nodeRun.Status != "cancelled" || job.Status != "cancelled" {
		t.Fatalf("affected AI was not cancelled: ai=%q node_run=%q job=%q", aiRun.Status, nodeRun.Status, job.Status)
	}
}
