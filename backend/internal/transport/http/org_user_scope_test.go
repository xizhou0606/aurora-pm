package httpapi

import (
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestListUsersUsesGlobalActivePMUserRegistry(t *testing.T) {
	app := newTestApp(t, nil)
	owner := app.createUser(t, 1, "owner", "user", "")
	teamMember := app.createUser(t, 2, "team-member", "user", "")
	projectMember := app.createUser(t, 3, "project-member", "user", "")
	taskParticipant := app.createUser(t, 4, "task-participant", "user", "")
	outsider := app.createUser(t, 5, "outsider", "user", "")
	foreign := app.createUser(t, 6, "foreign", "user", "")
	superAdmin := app.createUser(t, 8, "super-admin", "super_admin", "")
	for _, row := range []any{
		&identitygorm.User{ID: 7, ExternalSubject: "shared:77", ExternalIssuer: "https://identity.example",
			Username: "shared-77", DisplayName: "Shared 77", Role: "user", Active: true},
		&identitygorm.Workspace{ID: 1, Name: "Workspace A", Slug: "workspace-a", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "Workspace B", Slug: "workspace-b", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: owner.ID, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: teamMember.ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: projectMember.ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: taskParticipant.ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: outsider.ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: 7, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: foreign.ID, Role: "owner", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Team A"},
		&identitygorm.Team{ID: 11, WorkspaceID: 1, Name: "Team B"},
		&identitygorm.Team{ID: 20, WorkspaceID: 2, Name: "Foreign Team"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: teamMember.ID, Role: "member"},
		&projectgorm.ProjectSet{ID: 100, WorkspaceID: 1, TeamID: 10, Name: "Scoped project"},
		&projectgorm.PlanUnit{ID: 200, WorkspaceID: 1, TeamID: 10, ProjectSetID: 100, Title: "V1"},
		&requirementgorm.Requirement{ID: 300, WorkspaceID: 1, TeamID: 10, ProjectSetID: 100, PlanUnitID: 200, Title: "Requirement"},
		&projectgorm.ProjectPermission{WorkspaceID: 1, TeamID: 10, ProjectSetID: 100, UserID: projectMember.ID, AccessLevel: "member"},
		&projectgorm.ProjectPermission{WorkspaceID: 1, TeamID: 10, ProjectSetID: 100, UserID: 7, AccessLevel: "viewer"},
		&projectgorm.ProjectSet{ID: 101, WorkspaceID: 1, TeamID: 10, Name: "Fallback project", OwnerUserID: &owner.ID},
		&projectgorm.PlanUnit{ID: 201, WorkspaceID: 1, TeamID: 10, ProjectSetID: 101, Title: "V2", OwnerUserID: &outsider.ID},
		&projectgorm.ProjectSet{ID: 500, WorkspaceID: 2, TeamID: 20, Name: "Foreign project"},
		&projectgorm.PlanUnit{ID: 600, WorkspaceID: 2, TeamID: 20, ProjectSetID: 500, Title: "Foreign V1"},
		&taskgorm.WorkTask{ID: 400, WorkspaceID: 1, TeamID: 10, ProjectSetID: 100, ProjectID: 200,
			RequirementID: 300, Kind: taskgorm.WorkTaskKindTask, TaskKey: "SCOPE-1", Title: "Scoped task",
			AssigneeID: &taskParticipant.ID, ReporterUserID: &projectMember.ID},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	ownerToken := app.token(t, owner)

	activeUsers := []uint{1, 2, 3, 4, 5, 6, 7, 8}
	assertUserScopeIDs(t, app, ownerToken, "/api/v1/core/users?team_id=10", activeUsers)
	assertUserScopeIDs(t, app, ownerToken, "/api/v1/core/users?project_id=200", activeUsers)
	assertUserScopeIDs(t, app, ownerToken, "/api/v1/core/users?milestone_id=200", activeUsers)
	assertUserScopeIDs(t, app, ownerToken, "/api/v1/core/users?ids=2,6,7", []uint{2, 6, 7})
	assertUserScopeIDs(t, app, app.token(t, superAdmin), "/api/v1/core/users?team_id=10",
		activeUsers)

	outsiderToken := app.token(t, outsider)
	assertUserScopeIDs(t, app, outsiderToken, "/api/v1/core/users?team_id=10", activeUsers)
	assertUserScopeIDs(t, app, outsiderToken, "/api/v1/core/users?project_id=200", activeUsers)
}

func assertUserScopeIDs(t *testing.T, app *testApp, token, path string, want []uint) {
	t.Helper()
	rec := app.request(t, http.MethodGet, path, token, 1, nil)
	requireStatus(t, rec, http.StatusOK)
	rows := decodeData(t, rec).([]any)
	if len(rows) != len(want) {
		t.Fatalf("path=%s rows=%v want=%v", path, rows, want)
	}
	for index, id := range want {
		row := rows[index].(map[string]any)
		if row["id"] != float64(id) {
			t.Fatalf("path=%s row[%d]=%v want id=%d", path, index, row, id)
		}
	}
}
