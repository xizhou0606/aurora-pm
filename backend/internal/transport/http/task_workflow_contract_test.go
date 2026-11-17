package httpapi

import (
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"testing"
)

func TestTaskManagementStandardWorkflowContracts(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "workflow-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)

	statuses := app.request(t, http.MethodGet,
		"/api/v1/platform/issue-statuses?team_id=10&project_id=12&kind=task", token, 1, nil)
	requireStatus(t, statuses, http.StatusOK)
	statusRows := decodeData(t, statuses).([]any)
	expectedColors := map[string]string{
		"draft": "#b45309", "in_dev": "#2563eb", "pending_test": "#b45309",
		"completed": "#15803d", "cancelled": "#64748b",
	}
	if len(statusRows) != len(expectedColors) {
		t.Fatalf("status catalog=%s", statuses.Body.String())
	}
	for _, raw := range statusRows {
		row := raw.(map[string]any)
		key := row["status_key"].(string)
		if row["color"] != expectedColors[key] {
			t.Fatalf("status %q color=%v want=%q", key, row["color"], expectedColors[key])
		}
	}

	workflow := app.request(t, http.MethodGet,
		"/api/v1/platform/issue-workflow-transitions?team_id=10&project_id=12&kind=subtask", token, 1, nil)
	requireStatus(t, workflow, http.StatusOK)
	workflowData := decodeData(t, workflow).(map[string]any)
	if workflowData["enforce"] != false || workflowData["standard"] != true ||
		workflowData["workflow_profile"] != "standard_builtin" ||
		len(workflowData["transitions"].([]any)) != 10 {
		t.Fatalf("workflow=%s", workflow.Body.String())
	}

	task := taskgorm.WorkTask{
		ID: 801, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-801", Title: "Workflow task",
		Status: "draft", Priority: "normal",
	}
	if err := app.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	detail := app.request(t, http.MethodGet, "/api/v1/platform/tasks/801", token, 1, nil)
	requireStatus(t, detail, http.StatusOK)
	allowed := decodeData(t, detail).(map[string]any)["allowed_status_transitions"].([]any)
	if len(allowed) != 5 || allowed[4] != "cancelled" {
		t.Fatalf("allowed transitions=%s", detail.Body.String())
	}

	foreign := app.request(t, http.MethodGet,
		"/api/v1/platform/issue-statuses?team_id=20&project_id=22&kind=task", token, 1, nil)
	requireStatus(t, foreign, http.StatusNotFound)
	invalid := app.request(t, http.MethodGet,
		"/api/v1/platform/issue-statuses?kind=vuln", token, 1, nil)
	requireStatus(t, invalid, http.StatusBadRequest)
}
