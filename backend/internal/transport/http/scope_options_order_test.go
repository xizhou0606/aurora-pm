package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
	"time"
)

func TestScopeOptionsMatchesSourceOrderingAndVisibility(t *testing.T) {
	app := newTestApp(t, nil)
	viewer := app.createUser(t, 1, "viewer", "user", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "Workspace A", Slug: "workspace-a", Enabled: true}
	otherWorkspace := identitygorm.Workspace{ID: 2, Name: "Workspace B", Slug: "workspace-b", Enabled: true}
	visibleTeam := identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Visible"}
	hiddenTeam := identitygorm.Team{ID: 20, WorkspaceID: 1, Name: "Hidden"}
	foreignTeam := identitygorm.Team{ID: 30, WorkspaceID: 2, Name: "Foreign"}
	for _, row := range []any{
		&workspace, &otherWorkspace, &visibleTeam, &hiddenTeam, &foreignTeam,
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: viewer.ID, Role: "member", Enabled: true},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: visibleTeam.ID, UserID: viewer.ID, Role: "member"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	newer := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
	older := newer.Add(-24 * time.Hour)
	projects := []projectgorm.ProjectSet{
		{ID: 10, WorkspaceID: 1, TeamID: 10, Name: "Visible 10", UpdatedAt: newer},
		{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "Visible 11", UpdatedAt: newer},
		{ID: 12, WorkspaceID: 1, TeamID: 10, Name: "Visible 12", UpdatedAt: older},
		{ID: 20, WorkspaceID: 1, TeamID: 20, Name: "Hidden", UpdatedAt: newer.Add(time.Hour)},
		{ID: 30, WorkspaceID: 2, TeamID: 30, Name: "Foreign", UpdatedAt: newer.Add(2 * time.Hour)},
	}
	for i := range projects {
		if err := app.db.Create(&projects[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	plans := []projectgorm.PlanUnit{
		{ID: 100, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, Title: "10 / one", SortOrder: 1},
		{ID: 101, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, Title: "10 / two", SortOrder: 1},
		{ID: 102, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, Title: "10 / zero", SortOrder: 0},
		{ID: 110, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "11 / zero", SortOrder: 0},
		{ID: 120, WorkspaceID: 1, TeamID: 10, ProjectSetID: 12, Title: "12 / zero", SortOrder: 0},
		{ID: 200, WorkspaceID: 1, TeamID: 20, ProjectSetID: 20, Title: "Hidden", SortOrder: 0},
		{ID: 300, WorkspaceID: 2, TeamID: 30, ProjectSetID: 30, Title: "Foreign", SortOrder: 0},
	}
	for i := range plans {
		if err := app.db.Create(&plans[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	requirements := []requirementgorm.Requirement{
		{ID: 1000, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, PlanUnitID: 100, Title: "10 / 100 / one"},
		{ID: 1001, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, PlanUnitID: 100, Title: "10 / 100 / two"},
		{ID: 1020, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, PlanUnitID: 102, Title: "10 / 102"},
		{ID: 1100, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, PlanUnitID: 110, Title: "11 / 110"},
		{ID: 1200, WorkspaceID: 1, TeamID: 10, ProjectSetID: 12, PlanUnitID: 120, Title: "12 / 120"},
		{ID: 2000, WorkspaceID: 1, TeamID: 20, ProjectSetID: 20, PlanUnitID: 200, Title: "Hidden"},
		{ID: 3000, WorkspaceID: 2, TeamID: 30, ProjectSetID: 30, PlanUnitID: 300, Title: "Foreign"},
		// Requirement-pool rows and corrupt legacy hierarchy references remain
		// queryable from the requirement pool, but cannot form scope options.
		{ID: 1002, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, Title: "Unscheduled"},
		{ID: 1003, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, PlanUnitID: 999, Title: "Missing plan unit"},
		{ID: 1004, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, PlanUnitID: 110, Title: "Cross-project plan unit"},
		{ID: 1005, WorkspaceID: 1, TeamID: 10, ProjectSetID: 10, PlanUnitID: 300, Title: "Cross-workspace plan unit"},
		{ID: 1006, WorkspaceID: 1, TeamID: 20, ProjectSetID: 10, PlanUnitID: 100, Title: "Cross-team plan unit"},
	}
	for i := range requirements {
		if err := app.db.Create(&requirements[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	rec := app.request(t, http.MethodGet,
		"/api/v1/platform/project-requirement-scope-options", app.token(t, viewer), 1, nil)
	requireStatus(t, rec, http.StatusOK)
	data := decodeData(t, rec).(map[string]any)
	assertJSONRowIDs(t, data["project_sets"], []uint{20, 11, 10, 12})
	assertJSONRowIDs(t, data["projects"], []uint{102, 101, 100, 110, 120, 200})
	assertJSONRowIDs(t, data["requirements"], []uint{1001, 1000, 1020, 1100, 1200, 2000})
	assertJSONRowShape(t, data["project_sets"], "id", "team_id", "name", "code")
	assertJSONRowShape(t, data["projects"], "id", "team_id", "project_new_id", "title", "requirements_locked_at")
	assertJSONRowShape(t, data["requirements"], "id", "team_id", "project_new_id", "version_task_new_id", "title")
}

func assertJSONRowIDs(t *testing.T, raw any, want []uint) {
	t.Helper()
	rows, ok := raw.([]any)
	if !ok || len(rows) != len(want) {
		t.Fatalf("rows=%v want ids=%v", raw, want)
	}
	for index, id := range want {
		row, ok := rows[index].(map[string]any)
		if !ok || row["id"] != float64(id) {
			t.Fatalf("row[%d]=%v want id=%d", index, rows[index], id)
		}
	}
}

func assertJSONRowShape(t *testing.T, raw any, keys ...string) {
	t.Helper()
	rows, ok := raw.([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("rows=%v want a non-empty list", raw)
	}
	row, ok := rows[0].(map[string]any)
	if !ok || len(row) != len(keys) {
		t.Fatalf("row=%v want exactly keys=%v", rows[0], keys)
	}
	for _, key := range keys {
		if _, exists := row[key]; !exists {
			t.Fatalf("row=%v missing key=%s", row, key)
		}
	}
}
