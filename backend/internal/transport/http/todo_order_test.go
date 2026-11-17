package httpapi

import (
	"encoding/json"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWorkbenchTodoFeedsUseOperationalPriorityOrder(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	now := time.Now().UTC()
	assignee := admin.ID
	tasks := []taskgorm.WorkTask{
		{ID: 501, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			Kind: "task", TaskKey: "A-501", Title: "normal", Status: "draft", Priority: "normal", AssigneeID: &assignee, DueAt: timePtr(now.Add(24 * time.Hour))},
		{ID: 502, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			Kind: "task", TaskKey: "A-502", Title: "high", Status: "draft", Priority: "high", AssigneeID: &assignee, DueAt: timePtr(now.Add(48 * time.Hour))},
		{ID: 503, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			Kind: "task", TaskKey: "A-503", Title: "urgent", Status: "draft", Priority: "urgent", AssigneeID: &assignee, DueAt: timePtr(now.Add(72 * time.Hour))},
		{ID: 504, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			Kind: "task", TaskKey: "A-504", Title: "blocked", Status: "blocked", Priority: "normal", AssigneeID: &assignee, DueAt: timePtr(now.Add(96 * time.Hour))},
		{ID: 505, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			Kind: "task", TaskKey: "A-505", Title: "overdue", Status: "draft", Priority: "normal", AssigneeID: &assignee, DueAt: timePtr(now.Add(-time.Hour))},
	}
	if err := app.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	bugs := []qualitygorm.QualityBug{
		{ID: 601, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-B601", Title: "normal", Status: "todo", Priority: "medium", SeverityKey: "p2", AssigneeUserID: &assignee, DueAt: timePtr(now.Add(24 * time.Hour))},
		{ID: 602, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-B602", Title: "high", Status: "todo", Priority: "high", SeverityKey: "p2", AssigneeUserID: &assignee, DueAt: timePtr(now.Add(120 * time.Hour))},
		{ID: 603, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-B603", Title: "severe", Status: "todo", Priority: "medium", SeverityKey: "p1", AssigneeUserID: &assignee, DueAt: timePtr(now.Add(96 * time.Hour))},
		{ID: 604, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-B604", Title: "review", Status: "review", Priority: "medium", SeverityKey: "p2", ReporterUserID: &assignee, DueAt: timePtr(now.Add(72 * time.Hour))},
		{ID: 605, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-B605", Title: "overdue", Status: "todo", Priority: "medium", SeverityKey: "p2", AssigneeUserID: &assignee, DueAt: timePtr(now.Add(-time.Hour))},
	}
	if err := app.db.Create(&bugs).Error; err != nil {
		t.Fatal(err)
	}
	assertTodoFeedIDs(t, app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/feed?type=task&limit=10", token, 1, nil),
		[]float64{505, 504, 503, 502, 501})
	assertTodoFeedIDs(t, app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/feed?type=bug&limit=10", token, 1, nil),
		[]float64{605, 604, 603, 602, 601})
}

func assertTodoFeedIDs(t *testing.T, rec *httptest.ResponseRecorder, expected []float64) {
	t.Helper()
	requireStatus(t, rec, http.StatusOK)
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	actual := make([]float64, 0, len(envelope.Data))
	for _, row := range envelope.Data {
		actual = append(actual, row["id"].(float64))
	}
	if len(actual) != len(expected) {
		t.Fatalf("todo order ids=%v want=%v body=%s", actual, expected, rec.Body.String())
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("todo order ids=%v want=%v", actual, expected)
		}
	}
}
