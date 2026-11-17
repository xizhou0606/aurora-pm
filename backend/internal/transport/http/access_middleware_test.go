package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	identityhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/http"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workbenchhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestAccessMiddlewareUsesCanonicalRouteAndFailsClosed(t *testing.T) {
	app, access, inventory := newAccessMiddlewareFixture(t)
	user := app.createUser(t, 17, "access-user", "user", "")
	token := app.token(t, user)

	menus := app.request(t, http.MethodGet, "/api/v1/core/access/me/menus", token, 0, nil)
	requireStatus(t, menus, http.StatusOK)
	assertAccessResponseCode(t, menus, "")
	actions := app.request(t, http.MethodGet, "/api/v1/core/access/me/action-permissions", token, 0, nil)
	requireStatus(t, actions, http.StatusOK)
	assertAccessResponseCode(t, actions, "")

	unregistered := app.request(t, http.MethodGet, "/api/v1/core/access/admin/roles", token, 0, nil)
	requireStatus(t, unregistered, http.StatusForbidden)
	assertAccessResponseCode(t, unregistered, apiErrorAccessInterfaceUnregistered)

	if _, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "access.roles.list-test", HTTPMethod: http.MethodGet, RouteTemplate: "/core/access/admin/roles", Name: "Alias", Enabled: true,
	}); !errors.Is(err, identity.ErrAccessRouteNotRegistered) {
		t.Fatalf("alias registration err=%v", err)
	}
	rolesRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "access.roles.list-test", HTTPMethod: http.MethodGet, RouteTemplate: "/api/v1/core/access/admin/roles", Name: "List roles", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	forbidden := app.request(t, http.MethodGet, "/api/v1/core/access/admin/roles", token, 0, nil)
	requireStatus(t, forbidden, http.StatusForbidden)
	assertAccessResponseCode(t, forbidden, apiErrorAccessInterfaceForbidden)

	role, err := access.CreateRole(context.Background(), identity.RoleInput{Code: "access-test-role", Name: "Access test role", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{rolesRoute.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	canonical := app.request(t, http.MethodGet, "/api/v1/core/access/admin/roles", token, 0, nil)
	requireStatus(t, canonical, http.StatusOK)
	alias := app.request(t, http.MethodGet, "/core/access/admin/roles", token, 0, nil)
	requireStatus(t, alias, http.StatusOK)

	roleDetailRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "access.roles.get-test", HTTPMethod: http.MethodGet, RouteTemplate: "/api/v1/core/access/admin/roles/:id", Name: "Get role", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{rolesRoute.ID, roleDetailRoute.ID}); err != nil {
		t.Fatal(err)
	}
	detail := app.request(t, http.MethodGet, "/core/access/admin/roles/987", token, 0, nil)
	requireStatus(t, detail, http.StatusNotFound)

	if err := inventory.ValidateServiceInterface(context.Background(), http.MethodPost, "/api/v1/core/auth/login"); !errors.Is(err, identity.ErrAccessRouteNotRegistered) {
		t.Fatalf("public login route registered as candidate err=%v", err)
	}
}

func TestTestingLibraryCompatibilityReportRequiresVersionReportAction(t *testing.T) {
	app, access, _ := newAccessMiddlewareFixture(t)
	user := app.createUser(t, 19, "testing-access-user", "user", "")
	if err := app.db.Create(&identitygorm.Workspace{ID: 1, Name: "Testing", Slug: "testing-access", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: user.ID, Role: "member", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	token := app.token(t, user)

	libraryRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "testing.library.compat-list", ActionCode: string(accesscatalog.ActionTestingLibraryView),
		HTTPMethod: http.MethodGet, RouteTemplate: "/api/v1/platform/pm/testing/items", Name: "List testing library", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	reportRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "testing.version-reports.list", ActionCode: string(accesscatalog.ActionTestingVersionReportsView),
		HTTPMethod: http.MethodGet, RouteTemplate: versionReportListRoute, Name: "List version test reports", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	libraryUploadRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "testing.library.compat-upload", ActionCode: string(accesscatalog.ActionTestingLibraryUpload),
		HTTPMethod: http.MethodPost, RouteTemplate: "/api/v1/platform/pm/testing/items", Name: "Upload testing library item", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	reportUploadRoute, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "testing.version-reports.upload", ActionCode: string(accesscatalog.ActionTestingVersionReportsUpload),
		HTTPMethod: http.MethodPost, RouteTemplate: versionReportUploadRoute, Name: "Upload version test report", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	role, err := access.CreateRole(context.Background(), identity.RoleInput{Code: "testing-reader", Name: "Testing reader", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{libraryRoute.ID}); err != nil {
		t.Fatal(err)
	}

	compatForbidden := app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_report", token, 1, nil)
	requireStatus(t, compatForbidden, http.StatusForbidden)
	assertAccessResponseCode(t, compatForbidden, apiErrorAccessInterfaceForbidden)
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{libraryRoute.ID, libraryUploadRoute.ID}); err != nil {
		t.Fatal(err)
	}
	compatUploadForbidden := app.request(t, http.MethodPost, "/api/v1/platform/pm/testing/items?kind=test_report", token, 1, nil)
	requireStatus(t, compatUploadForbidden, http.StatusForbidden)
	assertAccessResponseCode(t, compatUploadForbidden, apiErrorAccessInterfaceForbidden)

	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{libraryRoute.ID, reportRoute.ID, libraryUploadRoute.ID, reportUploadRoute.ID}); err != nil {
		t.Fatal(err)
	}
	compatAllowed := app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_report", token, 1, nil)
	requireStatus(t, compatAllowed, http.StatusOK)
	compatUploadAllowed := app.request(t, http.MethodPost, "/api/v1/platform/pm/testing/items?kind=test_report", token, 1, nil)
	requireStatus(t, compatUploadAllowed, http.StatusBadRequest)

	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{reportRoute.ID, reportUploadRoute.ID}); err != nil {
		t.Fatal(err)
	}
	canonicalAllowed := app.request(t, http.MethodGet, versionReportListRoute, token, 1, nil)
	requireStatus(t, canonicalAllowed, http.StatusOK)
	canonicalUploadAllowed := app.request(t, http.MethodPost, versionReportUploadRoute, token, 1, nil)
	requireStatus(t, canonicalUploadAllowed, http.StatusBadRequest)
	libraryForbidden := app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_case", token, 1, nil)
	requireStatus(t, libraryForbidden, http.StatusForbidden)
}

func TestPersonalAccountRoutesAreAuthenticatedSystemRoutes(t *testing.T) {
	t.Parallel()
	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/core/auth/me/account"},
		{http.MethodPatch, "/api/v1/core/auth/me/profile"},
		{http.MethodPut, "/api/v1/core/auth/me/password"},
		{http.MethodPost, "/api/v1/core/auth/me/identities/:provider/bind/start"},
		{http.MethodDelete, "/api/v1/core/auth/me/identities/:provider"},
		{http.MethodGet, "/api/v1/platform/external-tooling/effective-access"},
	} {
		route := route
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()
			if !isAuthenticatedSystemRoute(route.method, route.path) {
				t.Fatalf("personal account route is not an authenticated system route")
			}
		})
	}
}

func TestAccessMiddlewareDoesNotEnforceRolesInPrepareMode(t *testing.T) {
	app := newTestApp(t, nil)
	user := app.createUser(t, 18, "prepare-user", "user", "")
	server := &Server{
		cfg: app.cfg, db: app.db, tokens: app.tokens, authCache: map[string]cachedIdentity{},
	}
	engine := gin.New()
	protected := engine.Group("")
	protected.Use(server.authenticate, server.authorizeAccess)
	protected.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	app.engine = engine

	response := app.request(t, http.MethodGet, "/protected", app.token(t, user), 0, nil)
	requireStatus(t, response, http.StatusNoContent)
}

func newAccessMiddlewareFixture(t *testing.T) (*testApp, *identity.AccessService, *RouteInventory) {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.RBACMode = config.RBACModeEnforce
	})
	access := identity.NewAccessService(identitygorm.NewAccessRepository(app.db), nil)
	menus := workbench.NewMenuService(workbenchgorm.NewAccessMenuRepository(app.db), access)
	access.SetMenuGrantValidator(menus)
	inventory := NewRouteInventory()
	accessHandler := identityhttp.NewAccessHandler(access, inventory)
	menuHandler := workbenchhttp.NewMenuHandler(menus)
	engine := NewWithDependencies(app.cfg, app.db, Dependencies{
		Authorizer: access,
		MountAccessRoutes: func(group *gin.RouterGroup) {
			accessHandler.RegisterRoutes(group)
		},
		MountMenuRoutes: func(group *gin.RouterGroup) {
			menuHandler.RegisterRoutes(group)
		},
	})
	inventory.Load(engine.Routes())
	access.SetServiceInterfaceValidator(inventory)
	app.engine = engine
	return app, access, inventory
}

func assertAccessResponseCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	got, _ := payload["code"].(string)
	if got != want {
		t.Fatalf("response code=%q want=%q payload=%s", got, want, rec.Body.String())
	}
}
