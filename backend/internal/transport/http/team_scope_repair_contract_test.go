package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/maintenance/teamscoperepair"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

func TestLegacyZeroTeamScopeRepairRestoresTaskCreation(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	projectMember := app.createUser(t, 2, "project-member", "user", "")
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "Workspace", Slug: "workspace", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Default Team"},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: admin.ID, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: projectMember.ID, Role: "member", Enabled: true},
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 0, Name: "Legacy Project", Code: "LEG"},
		&projectgorm.PlanUnit{ID: 12, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11, Title: "V1"},
		&requirementgorm.Requirement{ID: 13, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11, PlanUnitID: 12, Title: "Requirement"},
		&projectgorm.ProjectPermission{ID: 14, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11,
			UserID: projectMember.ID, AccessLevel: "member"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, admin)
	beforeUsers := app.request(t, http.MethodGet, "/api/v1/core/users?team_id=10", token, 1, nil)
	requireStatus(t, beforeUsers, http.StatusOK)
	if rows := decodeData(t, beforeUsers).([]any); len(rows) != 2 ||
		rows[1].(map[string]any)["id"] != float64(projectMember.ID) {
		t.Fatalf("unexpected pre-repair team candidates: %s", beforeUsers.Body.String())
	}
	create := func(title string) *httptest.ResponseRecorder {
		return app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, 1,
			map[string]any{"requirement_id": 13, "kind": "task", "title": title})
	}

	failed := create("Before repair")
	requireStatus(t, failed, http.StatusBadRequest)
	if !strings.Contains(failed.Body.String(), "project team does not belong to workspace") {
		t.Fatalf("unexpected original failure: %s", failed.Body.String())
	}

	dryRunner, err := teamscoperepair.New(app.db, teamscoperepair.Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report, err := dryRunner.Run(context.Background()); err != nil || !report.Ready {
		t.Fatalf("dry-run report=%+v err=%v", report, err)
	}
	requireStatus(t, create("After dry-run"), http.StatusBadRequest)

	applyRunner, err := teamscoperepair.New(app.db, teamscoperepair.Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report, err := applyRunner.Run(context.Background()); err != nil || !report.Applied {
		t.Fatalf("apply report=%+v err=%v", report, err)
	}
	requireStatus(t, create("After repair"), http.StatusCreated)
	afterUsers := app.request(t, http.MethodGet, "/api/v1/core/users?team_id=10", token, 1, nil)
	requireStatus(t, afterUsers, http.StatusOK)
	rows := decodeData(t, afterUsers).([]any)
	if len(rows) != 2 || rows[1].(map[string]any)["id"] != float64(projectMember.ID) {
		t.Fatalf("unexpected repaired team candidates: %s", afterUsers.Body.String())
	}
}
