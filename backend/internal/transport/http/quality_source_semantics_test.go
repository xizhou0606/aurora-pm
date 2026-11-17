package httpapi

import (
	"fmt"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
	"time"
)

func TestQualityBugCreateMatchesDeployedKeyAndActivityContract(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	actor := fix.users["manager"]
	token := fix.app.token(t, actor)

	create := func(requirementID uint, title string) map[string]any {
		t.Helper()
		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs", token, fix.workspaceID, map[string]any{
			"requirement_id": requirementID,
			"title":          title,
			"severity_key":   "p2",
		})
		requireStatus(t, response, http.StatusCreated)
		return decodeData(t, response).(map[string]any)
	}

	first := create(fix.requirement.ID, "First deployed-contract bug")
	firstID := uint(first["id"].(float64))
	assertQualityBugKey(t, first, firstID)

	var stored qualitygorm.QualityBug
	if err := fix.app.db.First(&stored, firstID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.BugKey != fmt.Sprintf("BUG-%d", firstID) {
		t.Fatalf("stored bug_key=%q want BUG-%d", stored.BugKey, firstID)
	}

	activities := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/activities", firstID),
		token, fix.workspaceID, nil)
	requireStatus(t, activities, http.StatusOK)
	activityRows := decodeData(t, activities).(map[string]any)["items"].([]any)
	if len(activityRows) != 1 {
		t.Fatalf("activities=%s", activities.Body.String())
	}
	activity := activityRows[0].(map[string]any)
	if activity["action"] != "created" || activity["summary"] != "提交缺陷" ||
		activity["field_key"] != "" || activity["old_value"] != "" || activity["new_value"] != "" ||
		activity["user_id"] != float64(actor.ID) {
		t.Fatalf("create activity contract=%v", activity)
	}

	secondProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		Name: "Second project", Code: "SECOND",
	}
	secondPlan := projectgorm.PlanUnit{
		ID: 22, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: secondProject.ID, Title: "Second plan",
	}
	secondRequirement := requirementgorm.Requirement{
		ID: 23, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: secondProject.ID, PlanUnitID: secondPlan.ID, Title: "Second requirement",
	}
	for _, row := range []any{&secondProject, &secondPlan, &secondRequirement} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	second := create(secondRequirement.ID, "Second deployed-contract bug")
	secondID := uint(second["id"].(float64))
	assertQualityBugKey(t, second, secondID)
	if first["bug_key"] == second["bug_key"] {
		t.Fatalf("cross-project bug keys collided: first=%v second=%v", first, second)
	}

	legacy := qualitygorm.QualityBug{
		ID: 900, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: fix.requirement.ID,
		BugKey: "", Title: "Legacy empty key", Status: "triage", Priority: "medium", SeverityKey: "p2",
	}
	if err := fix.app.db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	legacyResponse := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d", legacy.ID),
		token, fix.workspaceID, nil)
	requireStatus(t, legacyResponse, http.StatusOK)
	if got := decodeData(t, legacyResponse).(map[string]any)["bug_key"]; got != "BUG-900" {
		t.Fatalf("legacy fallback bug_key=%v want BUG-900", got)
	}

	for _, legacySeverity := range []string{"fatal", "critical", "blocker", "major", "minor"} {
		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs", token, fix.workspaceID, map[string]any{
			"requirement_id": fix.requirement.ID,
			"title":          "Legacy severity is read-only",
			"severity_key":   legacySeverity,
		})
		requireStatus(t, response, http.StatusBadRequest)
	}
}

func TestQualityBugListMatchesDeployedOrderingOwnershipAndSeverityContract(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	targetID := fix.users["reviewer"].ID
	otherID := fix.users["assignee"].ID
	now := time.Now().UTC()
	rows := []qualitygorm.QualityBug{
		qualitySourceBug(fix, 601, "triage target", "triage", "p0", &targetID, &otherID, &otherID, now.Add(24*time.Hour)),
		qualitySourceBug(fix, 602, "review target", "review", "p1", &otherID, &otherID, &targetID, now.Add(18*time.Hour)),
		qualitySourceBug(fix, 603, "done target", "done", "blocker", &otherID, &otherID, &targetID, now.Add(12*time.Hour)),
		qualitySourceBug(fix, 604, "resolved reporter fallback", "resolved", "critical", &otherID, &targetID, nil, now.Add(6*time.Hour)),
		qualitySourceBug(fix, 605, "closed reporter fallback", "closed", "p2", &otherID, &targetID, nil, now),
		qualitySourceBug(fix, 606, "completed target", "completed", "p3", &otherID, &otherID, &targetID, now.Add(-6*time.Hour)),
		qualitySourceBug(fix, 607, "done assignee is not owner", "done", "p2", &targetID, &otherID, &otherID, now.Add(-12*time.Hour)),
		qualitySourceBug(fix, 608, "triage reviewer is not owner", "triage", "p2", &otherID, &otherID, &targetID, now.Add(-24*time.Hour)),
	}
	for index := range rows {
		if err := fix.app.db.Create(&rows[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	seedForeignQualityBug(t, fix.app)

	token := fix.app.token(t, fix.users["manager"])
	list := fix.app.request(t, http.MethodGet, "/api/v1/platform/quality/bugs", token, fix.workspaceID, nil)
	requireStatus(t, list, http.StatusOK)
	data := decodeData(t, list).(map[string]any)
	if data["total"] != float64(8) {
		t.Fatalf("workspace-scoped total=%v body=%s", data["total"], list.Body.String())
	}
	assertQualityBugIDs(t, data["items"].([]any), []uint{608, 607, 606, 605, 604, 603, 602, 601})
	statistics := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs/statistics", token, fix.workspaceID, nil)
	requireStatus(t, statistics, http.StatusOK)
	summary := decodeData(t, statistics).(map[string]any)["summary"].(map[string]any)
	if summary["total"] != float64(8) || summary["open"] != float64(4) ||
		summary["review"] != float64(2) || summary["closed"] != float64(4) ||
		summary["high_severity"] != float64(2) {
		t.Fatalf("list summary=%v; legacy statuses and p0/p1 must match the source", summary)
	}

	page := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs?limit=2&offset=1", token, fix.workspaceID, nil)
	requireStatus(t, page, http.StatusOK)
	assertQualityBugIDs(t, decodeData(t, page).(map[string]any)["items"].([]any), []uint{607, 606})

	owners := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs?current_owner_user_id=%d", targetID),
		token, fix.workspaceID, nil)
	requireStatus(t, owners, http.StatusOK)
	ownerData := decodeData(t, owners).(map[string]any)
	if ownerData["total"] != float64(6) {
		t.Fatalf("owner total=%v body=%s", ownerData["total"], owners.Body.String())
	}
	ownerRows := ownerData["items"].([]any)
	assertQualityBugIDs(t, ownerRows, []uint{606, 605, 604, 603, 602, 601})
	wantStatuses := []string{"done", "done", "review", "done", "review", "triage"}
	for index, raw := range ownerRows {
		row := raw.(map[string]any)
		if row["current_owner_user_id"] != float64(targetID) {
			t.Fatalf("current owner mismatch row=%v target=%d", row, targetID)
		}
		if row["status"] != wantStatuses[index] {
			t.Fatalf("normalized status=%v want=%s row=%v", row["status"], wantStatuses[index], row)
		}
	}

	done := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs?status=done", token, fix.workspaceID, nil)
	requireStatus(t, done, http.StatusOK)
	assertQualityBugIDs(t, decodeData(t, done).(map[string]any)["items"].([]any), []uint{607, 606, 605, 603})
	review := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs?status=review", token, fix.workspaceID, nil)
	requireStatus(t, review, http.StatusOK)
	assertQualityBugIDs(t, decodeData(t, review).(map[string]any)["items"].([]any), []uint{604, 602})

	unclosed := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs?exclude_statuses=done,canceled", token, fix.workspaceID, nil)
	requireStatus(t, unclosed, http.StatusOK)
	unclosedData := decodeData(t, unclosed).(map[string]any)
	if unclosedData["total"] != float64(4) {
		t.Fatalf("unclosed total=%v body=%s", unclosedData["total"], unclosed.Body.String())
	}
	for _, raw := range unclosedData["items"].([]any) {
		status := raw.(map[string]any)["status"]
		if status == "done" || status == "canceled" {
			t.Fatalf("unclosed filter returned terminal status: %v", raw)
		}
	}
}

func assertQualityBugKey(t *testing.T, row map[string]any, id uint) {
	t.Helper()
	want := fmt.Sprintf("BUG-%d", id)
	if row["bug_key"] != want {
		t.Fatalf("bug_key=%v want=%s row=%v", row["bug_key"], want, row)
	}
}

func qualitySourceBug(
	fix qualityPermissionFixture,
	id uint,
	title, status, severity string,
	assigneeID, reporterID, reviewerID *uint,
	updatedAt time.Time,
) qualitygorm.QualityBug {
	return qualitygorm.QualityBug{
		ID: id, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: fix.requirement.ID,
		BugKey: fmt.Sprintf("SOURCE-%d", id), Title: title, Status: status,
		Priority: "medium", SeverityKey: severity, AssigneeUserID: assigneeID,
		ReporterUserID: reporterID, ReviewerUserID: reviewerID, UpdatedAt: updatedAt,
	}
}

func assertQualityBugIDs(t *testing.T, rows []any, want []uint) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("row count=%d want=%d rows=%v", len(rows), len(want), rows)
	}
	for index, raw := range rows {
		got := uint(raw.(map[string]any)["id"].(float64))
		if got != want[index] {
			t.Fatalf("row[%d].id=%d want=%d rows=%v", index, got, want[index], rows)
		}
	}
}

func seedForeignQualityBug(t *testing.T, app *testApp) {
	t.Helper()
	workspace := identitygorm.Workspace{ID: 2, Name: "Foreign quality workspace", Slug: "foreign-quality", Enabled: true}
	team := identitygorm.Team{ID: 20, WorkspaceID: workspace.ID, Name: "Foreign quality team"}
	project := projectgorm.ProjectSet{ID: 31, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "Foreign quality project", Code: "FQ"}
	plan := projectgorm.PlanUnit{ID: 32, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "Foreign plan"}
	requirement := requirementgorm.Requirement{ID: 33, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, PlanUnitID: plan.ID, Title: "Foreign requirement"}
	bug := qualitygorm.QualityBug{
		ID: 999, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID,
		ProjectID: plan.ID, RequirementID: requirement.ID, BugKey: "FOREIGN-999",
		Title: "Foreign p0 must not leak", Status: "triage", Priority: "urgent", SeverityKey: "p0",
	}
	for _, row := range []any{&workspace, &team, &project, &plan, &requirement, &bug} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}
