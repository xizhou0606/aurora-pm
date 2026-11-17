package httpapi

import (
	"net/http"
	"testing"

	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func TestQualityBugDashboardUsesListFiltersAndLifecycleSemantics(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	assigneeID := fix.users["assignee"].ID
	reviewerID := fix.users["reviewer"].ID
	reporterID := fix.users["reporter"].ID
	bugs := []qualitygorm.QualityBug{
		{ID: 910, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
			ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: fix.requirement.ID,
			BugKey: "QP-910", Title: "Triage", Status: "triage", SeverityKey: "p0",
			AssigneeUserID: &assigneeID, ReporterUserID: &reporterID},
		{ID: 911, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
			ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: fix.requirement.ID,
			BugKey: "QP-911", Title: "Review", Status: "review", SeverityKey: "p1",
			AssigneeUserID: &assigneeID, ReporterUserID: &reporterID, ReviewerUserID: &reviewerID},
		{ID: 912, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
			ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: fix.requirement.ID,
			BugKey: "QP-912", Title: "Done", Status: "closed", SeverityKey: "p2",
			AssigneeUserID: &assigneeID, ReporterUserID: &reporterID, ReviewerUserID: &reviewerID},
		{ID: 913, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
			ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: fix.requirement.ID,
			BugKey: "QP-913", Title: "Canceled", Status: "canceled", SeverityKey: "p3",
			AssigneeUserID: &assigneeID, ReporterUserID: &reporterID},
	}
	for index := range bugs {
		if err := fix.app.db.Create(&bugs[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := fix.app.token(t, fix.users["manager"])
	response := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs/dashboard", token, fix.workspaceID, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	summary := data["summary"].(map[string]any)
	assertDashboardCount(t, summary, "submitted", 4)
	assertDashboardCount(t, summary, "resolved", 2)
	assertDashboardCount(t, summary, "closed", 2)
	assertDashboardCount(t, summary, "backlog", 2)

	people := data["by_person"].([]any)
	if len(people) != 2 {
		t.Fatalf("by_person=%v body=%s", people, response.Body.String())
	}
	statuses := dashboardDimensionCounts(data["by_status"].([]any))
	if statuses["triage"] != 1 || statuses["review"] != 1 || statuses["done"] != 1 {
		t.Fatalf("by_status=%v body=%s", statuses, response.Body.String())
	}
	if _, exists := statuses["canceled"]; exists {
		t.Fatalf("retired canceled status should not be exposed: by_status=%v", statuses)
	}

	filtered := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs/dashboard?severity_key=p0", token, fix.workspaceID, nil)
	requireStatus(t, filtered, http.StatusOK)
	filteredSummary := decodeData(t, filtered).(map[string]any)["summary"].(map[string]any)
	assertDashboardCount(t, filteredSummary, "submitted", 1)
	assertDashboardCount(t, filteredSummary, "backlog", 1)
}

func assertDashboardCount(t *testing.T, summary map[string]any, key string, want float64) {
	t.Helper()
	if summary[key] != want {
		t.Fatalf("%s=%v want=%v", key, summary[key], want)
	}
}

func dashboardDimensionCounts(rows []any) map[string]float64 {
	counts := map[string]float64{}
	for _, value := range rows {
		row := value.(map[string]any)
		counts[row["key"].(string)] = row["count"].(float64)
	}
	return counts
}
