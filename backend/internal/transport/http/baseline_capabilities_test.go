package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	identityhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestAdmittedUserWithoutBusinessGrantsCanUseFixedBaselineCapabilities(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.RBACMode = config.RBACModeEnforce
	})
	user := app.createUser(t, 71, "default-group-user", "user", "")
	workspace := identitygorm.Workspace{ID: 72, Name: "默认工作区", Slug: "default", Enabled: true, IsDefault: true}
	if err := app.db.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.Workspace{ID: 74, Name: "未参与工作区", Slug: "foreign", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: workspace.ID, UserID: user.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	team := identitygorm.Team{ID: 73, WorkspaceID: workspace.ID, Name: "研发团队"}
	if err := app.db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.TeamMember{
		WorkspaceID: workspace.ID, TeamID: team.ID, UserID: user.ID, Role: "member",
	}).Error; err != nil {
		t.Fatal(err)
	}

	directory, err := identity.NewBaselineDirectoryService(identitygorm.NewBaselineDirectoryRepository(app.db))
	if err != nil {
		t.Fatal(err)
	}
	directoryHandler := identityhttp.NewBaselineDirectoryHandler(directory)
	authorizer := identity.NewAccessService(identitygorm.NewAccessRepository(app.db), nil)
	app.engine = NewWithDependencies(app.cfg, app.db, Dependencies{
		Authorizer:                    authorizer,
		MountBaselineDirectoryRoutes:  directoryHandler.RegisterAuthenticatedRoutes,
		MountWorkspaceDirectoryRoutes: directoryHandler.RegisterWorkspaceRoutes,
	})
	token := app.token(t, user)

	checks := []struct {
		path        string
		workspaceID uint
	}{
		{path: "/api/v1/core/auth/me/workspaces"},
		{path: "/api/v1/core/directory/users", workspaceID: workspace.ID},
		{path: "/api/v1/core/directory/teams", workspaceID: workspace.ID},
		{path: "/api/v1/core/directory/teams/73/members", workspaceID: workspace.ID},
		{path: "/api/v1/platform/project-requirement-scope-options", workspaceID: workspace.ID},
	}
	for _, check := range checks {
		response := app.request(t, http.MethodGet, check.path, token, check.workspaceID, nil)
		requireStatus(t, response, http.StatusOK)
		if check.path == "/api/v1/core/auth/me/workspaces" {
			ids := availableWorkspaceIDs(t, response)
			if len(ids) != 1 || ids[0] != workspace.ID {
				t.Fatalf("ordinary user workspaces=%v", ids)
			}
		}
	}

	for _, path := range []string{
		"/api/v1/core/workspaces",
		"/api/v1/core/users",
		"/api/v1/core/teams",
	} {
		response := app.request(t, http.MethodGet, path, token, workspace.ID, nil)
		requireStatus(t, response, http.StatusForbidden)
	}
}

func TestAccessAdministratorCanDiscoverEveryEnabledWorkspace(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.RBACMode = config.RBACModeEnforce
	})
	administrator := app.createUser(t, 81, "workspace-root", identity.LegacySuperAdminRole, "")
	if err := app.db.Create(&[]identitygorm.Workspace{
		{ID: 82, Name: "研发", Slug: "engineering", Enabled: true},
		{ID: 83, Name: "产品", Slug: "product", Enabled: true},
		{ID: 84, Name: "停用", Slug: "disabled", Enabled: false},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&identitygorm.Workspace{}).Where("id = ?", 84).UpdateColumn("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	role := identitygorm.AccessRole{
		Code: identity.AccessAdministratorRoleCode, Name: "Access administrator", Enabled: true, SystemManaged: true,
	}
	if err := app.db.Where("code = ?", role.Code).FirstOrCreate(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.FirstOrCreate(&identitygorm.AccessUserRole{UserID: administrator.ID, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}
	directory, err := identity.NewBaselineDirectoryService(identitygorm.NewBaselineDirectoryRepository(app.db))
	if err != nil {
		t.Fatal(err)
	}
	directoryHandler := identityhttp.NewBaselineDirectoryHandler(directory)
	app.engine = NewWithDependencies(app.cfg, app.db, Dependencies{
		Authorizer:                   identity.NewAccessService(identitygorm.NewAccessRepository(app.db), nil),
		MountBaselineDirectoryRoutes: directoryHandler.RegisterAuthenticatedRoutes,
	})

	response := app.request(
		t, http.MethodGet, "/api/v1/core/auth/me/workspaces", app.token(t, administrator), 0, nil,
	)
	requireStatus(t, response, http.StatusOK)
	ids := availableWorkspaceIDs(t, response)
	if len(ids) != 2 || ids[0] != 82 || ids[1] != 83 {
		t.Fatalf("administrator workspaces=%v", ids)
	}
}

func availableWorkspaceIDs(t *testing.T, response *httptest.ResponseRecorder) []uint {
	t.Helper()
	var envelope struct {
		Data struct {
			Items []identity.AvailableWorkspace `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode workspaces: %v body=%s", err, response.Body.String())
	}
	ids := make([]uint, 0, len(envelope.Data.Items))
	for _, workspace := range envelope.Data.Items {
		ids = append(ids, workspace.ID)
	}
	return ids
}
