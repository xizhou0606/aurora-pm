package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

func TestCreateTaskAcceptsActivePMUsersWhenActorIsWorkspaceMember(t *testing.T) {
	app := newTestApp(t, nil)
	actor := app.createUser(t, 45, "icefrog", "user", "")
	app.createUser(t, 31, "task-assignee", "user", "")
	rows := []any{
		&identitygorm.Workspace{ID: 2, Name: "Workspace 2", Slug: "workspace-2", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: actor.ID, Role: "member", Enabled: true},
		&identitygorm.Team{ID: 1, WorkspaceID: 2, Name: "Team 1"},
		&projectgorm.ProjectSet{ID: 5, WorkspaceID: 2, TeamID: 1, Name: "Test", Code: "TEST", Status: "active"},
		&projectgorm.PlanUnit{ID: 24, WorkspaceID: 2, TeamID: 1, ProjectSetID: 5, Title: "Version 24", Type: "version", Status: "active"},
		&requirementgorm.Requirement{ID: 142, WorkspaceID: 2, TeamID: 1, ProjectSetID: 5, PlanUnitID: 24, Title: "Member profile", Status: "backlog"},
	}
	for _, row := range rows {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	response := app.request(t, http.MethodPost, "/api/v1/platform/tasks", app.token(t, actor), 2,
		map[string]any{
			"requirement_id":   142,
			"kind":             "task",
			"title":            "Build profile",
			"status":           "draft",
			"priority":         "normal",
			"reporter_user_id": 45,
			"assignee_id":      31,
			"starts_at":        "2026-08-10T16:00:00.000Z",
			"due_at":           "2026-08-13T15:59:59.000Z",
		})
	requireStatus(t, response, http.StatusCreated)
	data := decodeData(t, response).(map[string]any)
	if data["requirement_id"] != float64(142) || data["assignee_id"] != float64(31) ||
		data["reporter_user_id"] != float64(45) {
		encoded, _ := json.Marshal(data)
		t.Fatalf("unexpected created task=%s", encoded)
	}
}

func TestBIScreenUsesLocalPMUserRegistry(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "bi-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", 13).
		Update("product_owner_user_id", admin.ID).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen", app.token(t, admin), 1, nil)
	requireStatus(t, response, http.StatusOK)
}
