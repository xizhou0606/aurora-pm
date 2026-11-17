package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"net/http"
	"testing"
)

func TestWorkspaceMembershipBoundaryAndTeamValidation(t *testing.T) {
	app := newTestApp(t, nil)
	user1 := app.createUser(t, 1, "w1-user", "user", "")
	user2 := app.createUser(t, 2, "w2-user", "user", "")
	for _, workspace := range []identitygorm.Workspace{
		{ID: 1, Name: "W1", Slug: "w1", Enabled: true},
		{ID: 2, Name: "W2", Slug: "w2", Enabled: true},
	} {
		if err := app.db.Create(&workspace).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, team := range []identitygorm.Team{
		{ID: 10, WorkspaceID: 1, Name: "T1"},
		{ID: 20, WorkspaceID: 2, Name: "T2"},
	} {
		if err := app.db.Create(&team).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: 1, UserID: user1.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	project2 := projectgorm.ProjectSet{
		ID: 22, WorkspaceID: 2, TeamID: 20, Name: "Secret", Status: "planning",
		OwnerUserID: &user2.ID,
	}
	if err := app.db.Create(&project2).Error; err != nil {
		t.Fatal(err)
	}
	token1 := app.token(t, user1)

	crossWorkspace := app.request(t, http.MethodGet, "/api/v1/platform/project-sets", token1, 2, nil)
	requireStatus(t, crossWorkspace, http.StatusNotFound)
	crossProject := app.request(t, http.MethodGet,
		"/api/v1/platform/project-management-board?project_new_id=22", token1, 1, nil)
	requireStatus(t, crossProject, http.StatusNotFound)
	badTeam := app.request(t, http.MethodPost, "/api/v1/platform/project-sets", token1, 1, map[string]any{
		"team_id": 20, "name": "Wrong scope",
	})
	requireStatus(t, badTeam, http.StatusBadRequest)
	users := app.request(t, http.MethodGet, "/api/v1/core/users", token1, 1, nil)
	requireStatus(t, users, http.StatusOK)
	rows := decodeData(t, users).([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["username"] != "w1-user" ||
		rows[1].(map[string]any)["username"] != "w2-user" {
		t.Fatalf("global PM user registry=%v", rows)
	}
}
