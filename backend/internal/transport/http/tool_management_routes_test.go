package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestToolManagementRoutesRequireCanonicalActionAndWorkspace(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.RBACMode = config.RBACModeEnforce
	})
	user := app.createUser(t, 31, "tooling-user", "user", "")
	workspace := identitygorm.Workspace{ID: 41, Name: "Tooling", Slug: "tooling", Enabled: true}
	if err := app.db.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: workspace.ID, UserID: user.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	access := identity.NewAccessService(identitygorm.NewAccessRepository(app.db), nil)
	inventory := NewRouteInventory()
	engine := NewWithDependencies(app.cfg, app.db, Dependencies{
		Authorizer: access,
		MountToolManagementRoutes: func(group *gin.RouterGroup) {
			group.GET("/tool-runtime/catalog", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			group.GET("/external-tooling/configuration", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			group.GET("/external-tooling/effective-access", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			group.PUT("/external-tooling/configuration", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
		},
	})
	inventory.Load(engine.Routes())
	access.SetServiceInterfaceValidator(inventory)
	app.engine = engine

	missingSession := app.request(t, http.MethodGet, "/api/v1/platform/tool-runtime/catalog", "", workspace.ID, nil)
	requireStatus(t, missingSession, http.StatusUnauthorized)

	withoutGrant := app.request(t, http.MethodGet, "/api/v1/platform/tool-runtime/catalog", app.token(t, user), workspace.ID, nil)
	requireStatus(t, withoutGrant, http.StatusForbidden)
	withoutConfigurationGrant := app.request(t, http.MethodGet, "/api/v1/platform/external-tooling/configuration", app.token(t, user), workspace.ID, nil)
	requireStatus(t, withoutConfigurationGrant, http.StatusForbidden)
	selfEffectiveAccess := app.request(t, http.MethodGet, "/api/v1/platform/external-tooling/effective-access", app.token(t, user), workspace.ID, nil)
	requireStatus(t, selfEffectiveAccess, http.StatusNoContent)

	toolCatalogRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code:          "route.tool-catalog-test",
		ActionCode:    string(accesscatalog.ActionExternalToolingCatalogView),
		HTTPMethod:    http.MethodGet,
		RouteTemplate: "/api/v1/platform/tool-runtime/catalog",
		Name:          "Tool catalog test route",
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	role, err := access.CreateRole(context.Background(), identity.RoleInput{Name: "Tool catalog viewer", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{toolCatalogRoute.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}

	missingWorkspace := app.request(t, http.MethodGet, "/api/v1/platform/tool-runtime/catalog", app.token(t, user), 0, nil)
	requireStatus(t, missingWorkspace, http.StatusBadRequest)

	allowed := app.request(t, http.MethodGet, "/api/v1/platform/tool-runtime/catalog", app.token(t, user), workspace.ID, nil)
	requireStatus(t, allowed, http.StatusNoContent)

	configurationViewRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code:          "route.tool-configuration-view-test",
		ActionCode:    string(accesscatalog.ActionExternalToolingConfigurationView),
		HTTPMethod:    http.MethodGet,
		RouteTemplate: "/api/v1/platform/external-tooling/configuration",
		Name:          "View Tool configuration test route",
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{toolCatalogRoute.ID, configurationViewRoute.ID}); err != nil {
		t.Fatal(err)
	}
	configurationView := app.request(t, http.MethodGet, "/api/v1/platform/external-tooling/configuration", app.token(t, user), workspace.ID, nil)
	requireStatus(t, configurationView, http.StatusNoContent)
	configurationUpdateDenied := app.request(t, http.MethodPut, "/api/v1/platform/external-tooling/configuration", app.token(t, user), workspace.ID, nil)
	requireStatus(t, configurationUpdateDenied, http.StatusForbidden)

	configurationUpdateRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code:          "route.tool-configuration-update-test",
		ActionCode:    string(accesscatalog.ActionExternalToolingConfigurationUpdate),
		HTTPMethod:    http.MethodPut,
		RouteTemplate: "/api/v1/platform/external-tooling/configuration",
		Name:          "Update Tool configuration test route",
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{
		toolCatalogRoute.ID, configurationViewRoute.ID, configurationUpdateRoute.ID,
	}); err != nil {
		t.Fatal(err)
	}
	configurationUpdate := app.request(t, http.MethodPut, "/api/v1/platform/external-tooling/configuration", app.token(t, user), workspace.ID, nil)
	requireStatus(t, configurationUpdate, http.StatusNoContent)
}
