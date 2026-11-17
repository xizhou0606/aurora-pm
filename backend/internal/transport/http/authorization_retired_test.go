package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
)

func TestActiveWorkspaceMemberUsesBusinessDataContextWithoutLegacyPermissions(t *testing.T) {
	app := newTestApp(t, nil)
	member := app.createUser(t, 1, "workspace-member", "readonly", "")
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "Workspace A", Slug: "workspace-a", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "Workspace B", Slug: "workspace-b", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: member.ID, Role: "member", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Team A"},
		&identitygorm.Team{ID: 20, WorkspaceID: 2, Name: "Team B"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: member.ID, Role: "member"},
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "Project A", Code: "A"},
		&projectgorm.ProjectSet{ID: 21, WorkspaceID: 2, TeamID: 20, Name: "Project B", Code: "B"},
		&projectgorm.PlanUnit{ID: 12, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "V1"},
		&projectgorm.PlanUnit{ID: 22, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, Title: "V2"},
		&requirementgorm.Requirement{ID: 13, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, PlanUnitID: 12, Title: "Requirement A"},
		&requirementgorm.Requirement{ID: 23, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, PlanUnitID: 22, Title: "Requirement B"},
		&projectgorm.ProjectPermission{WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, UserID: member.ID, AccessLevel: "viewer"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	token := app.token(t, member)
	requirementTemplateID := createPublishedRequirementWorkflow(t, app, token, 1)
	boardRec := app.request(t, http.MethodGet,
		"/api/v1/platform/project-management-board?project_new_id=11", token, 1, nil)
	requireStatus(t, boardRec, http.StatusOK)
	board := decodeData(t, boardRec).(map[string]any)
	project := board["project"].(map[string]any)
	for _, key := range []string{"can_update", "can_delete", "can_manage_members", "can_transfer_owner", "can_create_plan_unit", "current_user_access_level"} {
		if _, exists := project[key]; exists {
			t.Fatalf("retired project authorization field %s leaked: %v", key, project)
		}
	}
	if _, exists := board["project_permissions"]; exists {
		t.Fatalf("retired project permission resource leaked through board: %v", board)
	}
	if _, exists := board["effective_permissions"]; exists {
		t.Fatalf("retired effective permission resource leaked through board: %v", board)
	}

	patched := app.request(t, http.MethodPatch, "/api/v1/platform/project-sets/11", token, 1,
		map[string]any{"name": "Project A updated"})
	requireStatus(t, patched, http.StatusOK)
	createdTask := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, 1,
		map[string]any{"requirement_id": 13, "title": "Workspace-wide task"})
	requireStatus(t, createdTask, http.StatusCreated)
	createdBug := app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs", token, 1,
		map[string]any{"requirement_id": 13, "title": "Workspace-wide bug"})
	requireStatus(t, createdBug, http.StatusCreated)

	createdRequirement := app.request(t, http.MethodPost,
		"/api/v1/platform/product-requirements-new", token, 1, map[string]any{
			"priority":              "p1",
			"version_task_new_id":   12,
			"title":                 "No permission synchronization",
			"product_owner_user_id": member.ID,
			"workflow_template_id":  requirementTemplateID,
		})
	requireStatus(t, createdRequirement, http.StatusCreated)
	var permissionCount, roleCount int64
	app.db.Model(&projectgorm.ProjectPermission{}).Count(&permissionCount)
	app.db.Model(&projectgorm.ProjectMemberRole{}).Count(&roleCount)
	if permissionCount != 1 || roleCount != 0 {
		t.Fatalf("requirement write changed retired permission data: permissions=%d roles=%d", permissionCount, roleCount)
	}

	retired := app.request(t, http.MethodGet,
		"/api/v1/platform/project-set-permissions?project_new_id=11", token, 1, nil)
	requireStatus(t, retired, http.StatusNotFound)
	for _, path := range []string{"/api/v1/core/auth/caps", "/api/v1/core/auth/rbac-effective"} {
		retired := app.request(t, http.MethodGet, path, token, 1, nil)
		requireStatus(t, retired, http.StatusNotFound)
	}

	foreignProject := app.request(t, http.MethodGet,
		"/api/v1/platform/project-management-board?project_new_id=21", token, 1, nil)
	requireStatus(t, foreignProject, http.StatusNotFound)
}
