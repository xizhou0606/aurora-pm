package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestNativeWorkbenchContractsAndWorkspaceIsolation(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	now := time.Now().UTC()
	rows := []any{
		&taskgorm.WorkTask{ID: 101, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-1", Title: "Workspace A todo",
			Status: "draft", Priority: "high", AssigneeID: &admin.ID, DueAt: timePtr(now.Add(48 * time.Hour))},
		&taskgorm.WorkTask{ID: 102, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "subtask", TaskKey: "A-2", Title: "Workspace A done",
			Status: "completed", Priority: "normal", AssigneeID: &admin.ID},
		&taskgorm.WorkTask{ID: 201, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
			RequirementID: 23, Kind: "task", TaskKey: "B-1", Title: "Workspace B secret",
			Status: "draft", Priority: "urgent", AssigneeID: &admin.ID},
		&taskgorm.WorkTask{ID: 103, WorkspaceID: 1, TeamID: 10, ProjectSetID: 21, ProjectID: 22,
			RequirementID: 23, Kind: "task", TaskKey: "BAD-1", Title: "Corrupt cross hierarchy",
			Status: "draft", Priority: "urgent", AssigneeID: &admin.ID},
		&qualitygorm.QualityBug{ID: 301, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, BugKey: "A-SEED-1", Title: "Workspace A bug", Status: "review",
			Priority: "high", SeverityKey: "p1", ReporterUserID: &admin.ID,
			FoundVersion: "v1", SourceExecutionID: uintPtr(901), SourceTestID: uintPtr(902)},
		&qualitygorm.QualityBug{ID: 401, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
			RequirementID: 23, BugKey: "BB-1", Title: "Workspace B secret bug", Status: "review",
			Priority: "urgent", SeverityKey: "p0", ReporterUserID: &admin.ID},
		&qualitygorm.QualityBug{ID: 302, WorkspaceID: 1, TeamID: 10, ProjectSetID: 21, ProjectID: 22,
			RequirementID: 23, BugKey: "BADB-1", Title: "Corrupt cross bug", Status: "review",
			Priority: "urgent", SeverityKey: "p0", ReporterUserID: &admin.ID},
	}
	for _, row := range rows {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	list := app.request(t, http.MethodGet, "/api/v1/platform/tasks?project_set_ids=11&project_ids=12&requirement_ids=13", token, 1, nil)
	requireStatus(t, list, http.StatusOK)
	taskList := decodeData(t, list).(map[string]any)
	if taskList["total"] != float64(2) || len(taskList["items"].([]any)) != 2 || strings.Contains(list.Body.String(), "secret") {
		t.Fatalf("task list leaked or mismatched: %s", list.Body.String())
	}
	dueSoon := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?filter_kinds=task&due_bucket=due_soon", token, 1, nil)
	requireStatus(t, dueSoon, http.StatusOK)
	dueSoonList := decodeData(t, dueSoon).(map[string]any)
	if dueSoonList["total"] != float64(1) ||
		dueSoonList["items"].([]any)[0].(map[string]any)["id"] != float64(101) {
		t.Fatalf("task BI drilldown filters=%s", dueSoon.Body.String())
	}
	stats := app.request(t, http.MethodGet, "/api/v1/platform/tasks/stats?due_soon_days=3", token, 1, nil)
	requireStatus(t, stats, http.StatusOK)
	stat := decodeData(t, stats).(map[string]any)
	if stat["total"] != float64(2) || stat["completed"] != float64(1) ||
		stat["incomplete"] != float64(1) || stat["due_soon"] != float64(1) {
		t.Fatalf("task stats contract=%v", stat)
	}
	foreignTask := app.request(t, http.MethodGet, "/api/v1/platform/tasks/201", token, 1, nil)
	requireStatus(t, foreignTask, http.StatusNotFound)

	quality := app.request(t, http.MethodGet, "/api/v1/platform/quality/bugs", token, 1, nil)
	requireStatus(t, quality, http.StatusOK)
	qualityList := decodeData(t, quality).(map[string]any)
	if qualityList["total"] != float64(1) || len(qualityList["items"].([]any)) != 1 ||
		strings.Contains(quality.Body.String(), "secret") {
		t.Fatalf("quality list leaked or mismatched: %s", quality.Body.String())
	}
	bugDTO := qualityList["items"].([]any)[0].(map[string]any)
	if bugDTO["project_new_id"] != float64(12) || bugDTO["found_version"] != "v1" ||
		bugDTO["source_execution_id"] != float64(901) || bugDTO["current_owner_user_id"] != float64(1) {
		t.Fatalf("quality compatibility fields=%v", bugDTO)
	}

	created := app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs", token, 1, map[string]any{
		"requirement_id": 13, "title": "Created bug", "due_date": "2026-08-20",
		"found_version": "v2", "source_execution_id": 903, "source_test_id": 904,
		"defect_owner_user_id": 1,
	})
	requireStatus(t, created, http.StatusCreated)
	createdBug := decodeData(t, created).(map[string]any)
	createdID := uint(createdBug["id"].(float64))
	missingReason := app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d", createdID), token, 1, map[string]any{"status": "review"})
	requireStatus(t, missingReason, http.StatusBadRequest)
	patched := app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d", createdID), token, 1,
		map[string]any{"status": "review", "resolution_reason": "fixed", "resolution_note": "fixed in v2"})
	requireStatus(t, patched, http.StatusOK)
	activities := app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/activities", createdID), token, 1, nil)
	requireStatus(t, activities, http.StatusOK)
	activityPage := decodeData(t, activities).(map[string]any)
	activityRows := activityPage["items"].([]any)
	if len(activityRows) != 4 || int(activityPage["total"].(float64)) != 4 {
		t.Fatalf("quality activities=%s", activities.Body.String())
	}
	wantActivities := []struct {
		action, field, value string
	}{
		{"resolved", "status", "review"},
		{"resolution_decided", "resolution_reason", "fixed"},
		{"assigned", "assignee_user_id", "1"},
		{"created", "", ""},
	}
	for index, want := range wantActivities {
		row := activityRows[index].(map[string]any)
		if row["action"] != want.action || row["field_key"] != want.field || row["new_value"] != want.value {
			t.Fatalf("quality activity[%d]=%v want=%+v", index, row, want)
		}
	}
	deleted := app.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d", createdID), token, 1, nil)
	requireStatus(t, deleted, http.StatusOK)
	var activityCount int64
	app.db.Model(&qualitygorm.QualityBugActivity{}).Where("quality_bug_id = ?", createdID).Count(&activityCount)
	if activityCount != 0 {
		t.Fatalf("deleted quality activities=%d", activityCount)
	}

	todo := app.request(t, http.MethodGet, "/api/v1/platform/workbench/todos/summary?domains=bug", token, 1, nil)
	requireStatus(t, todo, http.StatusOK)
	summary := decodeData(t, todo).(map[string]any)["summary"].(map[string]any)
	if summary["tasks"] != float64(0) || summary["bugs"] != float64(1) || summary["review_bugs"] != float64(1) {
		t.Fatalf("todo summary=%v", summary)
	}
	feed := app.request(t, http.MethodGet, "/api/v1/platform/workbench/todos/feed?type=bug", token, 1, nil)
	requireStatus(t, feed, http.StatusOK)
	var feedBody map[string]any
	if err := json.Unmarshal(feed.Body.Bytes(), &feedBody); err != nil {
		t.Fatal(err)
	}
	if feedBody["total"] != float64(1) || strings.Contains(feed.Body.String(), "secret") {
		t.Fatalf("todo feed leaked or mismatched: %s", feed.Body.String())
	}

	bi := app.request(t, http.MethodGet, "/api/v1/platform/executive-dashboard/bi-screen", token, 1, nil)
	requireStatus(t, bi, http.StatusOK)
	biData := decodeData(t, bi).(map[string]any)
	overview := biData["overview_metrics"].(map[string]any)
	if overview["total_tasks"] != float64(2) || overview["open_bugs"] != float64(1) ||
		biData["summary"] == nil || biData["kpi"] == nil || strings.Contains(bi.Body.String(), "secret") {
		t.Fatalf("BI contract leaked or mismatched: %s", bi.Body.String())
	}
	if _, exists := overview["story_points_total"]; exists {
		t.Fatalf("BI contract still exposes retired story points: %s", bi.Body.String())
	}
}

func TestSharedIdentityInternalIDsRoundTripThroughAssigneeAndTodo(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/core/workspaces":
			fmt.Fprint(w, `{"data":{"items":[{"id":9,"name":"Shared Workspace","enabled":true}]}}`)
		case "/api/v1/core/users":
			fmt.Fprint(w, `{"data":[{"id":77,"username":"shared-77","display_name":"Shared 77",`+
				`"avatar_url":"https://example.test/shared-77.png","role":"developer"}]}`)
		case "/internal/api/v1/internal-rpc/base/users/batch":
			fmt.Fprint(w, `{"data":{"items":[{"id":77,"username":"shared-77",`+
				`"display_name":"Shared 77","avatar_url":"https://example.test/shared-77.png"}]}}`)
		case "/api/v1/core/workspaces/9/members":
			fmt.Fprint(w, `{"data":{"items":[{"user_id":77,"role":"member","enabled":true}]}}`)
		default:
			fmt.Fprint(w, `{"data":{"id":77,"username":"shared-77","display_name":"Shared 77",`+
				`"role":"developer"}}`)
		}
	}))
	defer upstream.Close()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AuthMode = "shared_jwt"
		cfg.SharedJWTIssuer = "one2all"
		cfg.SharedIntrospectionURL = upstream.URL + "/api/v1/core/auth/me"
		cfg.SharedIntrospectionTimeout = time.Second
	})
	if err := app.db.Create(&identitygorm.Workspace{ID: 9, Name: "Shared Workspace", Slug: "shared-9", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.User{
		ID: 701, ExternalIssuer: "one2all", ExternalSubject: "one2all|77", Username: "shared-77", DisplayName: "Shared 77", Active: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	users := app.request(t, http.MethodGet, "/api/v1/core/users", "shared-token", 9, nil)
	requireStatus(t, users, http.StatusOK)
	userRows := decodeData(t, users).([]any)
	if len(userRows) != 1 || userRows[0].(map[string]any)["id"] != float64(701) {
		t.Fatalf("internal user id not exposed: %s", users.Body.String())
	}
	for _, row := range []any{
		&identitygorm.WorkspaceMember{WorkspaceID: 9, UserID: 701, Role: "member", Enabled: true},
		&identitygorm.Team{ID: 90, WorkspaceID: 9, Name: "Shared Team"},
		&identitygorm.TeamMember{WorkspaceID: 9, TeamID: 90, UserID: 701, Role: "admin"},
		&projectgorm.ProjectSet{ID: 91, WorkspaceID: 9, TeamID: 90, Name: "Shared PM", Code: "SH", OwnerUserID: uintPtr(701)},
		&projectgorm.PlanUnit{ID: 92, WorkspaceID: 9, TeamID: 90, ProjectSetID: 91, Title: "V1", OwnerUserID: uintPtr(701)},
		&requirementgorm.Requirement{ID: 93, WorkspaceID: 9, TeamID: 90, ProjectSetID: 91, PlanUnitID: 92,
			Title: "Shared requirement", ProductOwnerUserID: uintPtr(701)},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	members := app.request(t, http.MethodGet, "/api/v1/core/teams/90/members", "shared-token", 9, nil)
	requireStatus(t, members, http.StatusOK)
	if !strings.Contains(members.Body.String(), `"user_id":701`) {
		t.Fatalf("team member did not expose internal id: %s", members.Body.String())
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", "shared-token", 9, map[string]any{
		"requirement_id": 93, "title": "Shared assigned task", "assignee_id": 701,
	})
	requireStatus(t, created, http.StatusCreated)
	createdTask := decodeData(t, created).(map[string]any)
	if createdTask["assignee_id"] != float64(701) {
		t.Fatalf("shared assignee was rewritten: %v", createdTask)
	}
	todo := app.request(t, http.MethodGet, "/api/v1/platform/workbench/todos/summary?domains=task", "shared-token", 9, nil)
	requireStatus(t, todo, http.StatusOK)
	summary := decodeData(t, todo).(map[string]any)["summary"].(map[string]any)
	if summary["tasks"] != float64(1) {
		t.Fatalf("shared fact assignee did not match todo: %s", todo.Body.String())
	}
}

func seedNativeWorkbenchScopes(t *testing.T, app *testApp, admin identitygorm.User) {
	t.Helper()
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "A", Slug: "a-workspace", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "B", Slug: "b-workspace", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: admin.ID, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: admin.ID, Role: "owner", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Team A"},
		&identitygorm.Team{ID: 20, WorkspaceID: 2, Name: "Team B"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: admin.ID, Role: "admin"},
		&identitygorm.TeamMember{WorkspaceID: 2, TeamID: 20, UserID: admin.ID, Role: "admin"},
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "Project A", Code: "A", OwnerUserID: &admin.ID},
		&projectgorm.ProjectSet{ID: 21, WorkspaceID: 2, TeamID: 20, Name: "Project B", Code: "B", OwnerUserID: &admin.ID},
		&projectgorm.PlanUnit{ID: 12, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "Version A"},
		&projectgorm.PlanUnit{ID: 22, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, Title: "Version B"},
		&requirementgorm.Requirement{ID: 13, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, PlanUnitID: 12, Title: "Requirement A"},
		&requirementgorm.Requirement{ID: 23, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, PlanUnitID: 22, Title: "Requirement B"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func timePtr(value time.Time) *time.Time { return &value }
func uintPtr(value uint) *uint           { return &value }
