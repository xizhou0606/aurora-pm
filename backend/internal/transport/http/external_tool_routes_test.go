package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestExternalToolRoutesMountOutsidePMSessionAndRBACGroups(t *testing.T) {
	app := newTestApp(t, nil)
	app.engine = NewWithDependencies(config.Config{Environment: "test"}, app.db, Dependencies{
		MountExternalToolRoutes: func(group *gin.RouterGroup) {
			group.GET("/tool-runtime/catalog", func(c *gin.Context) { c.Status(http.StatusTeapot) })
			group.POST("/mcp", func(c *gin.Context) { c.Status(http.StatusAccepted) })
			group.POST("/tool-auth/device", func(c *gin.Context) { c.Status(http.StatusCreated) })
			group.POST("/tool-auth/token", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		},
	})

	for _, test := range []struct {
		method, path string
		status       int
	}{
		{method: http.MethodGet, path: "/api/v1/tool-runtime/catalog", status: http.StatusTeapot},
		{method: http.MethodPost, path: "/api/v1/mcp", status: http.StatusAccepted},
		{method: http.MethodPost, path: "/api/v1/tool-auth/device", status: http.StatusCreated},
		{method: http.MethodPost, path: "/api/v1/tool-auth/token", status: http.StatusNoContent},
	} {
		response := app.request(t, test.method, test.path, "", 0, nil)
		if response.Code != test.status {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestExternalToolRoutesAreExcludedFromPMRBACInventory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/tool-runtime/catalog", func(*gin.Context) {})
	engine.GET("/api/v1/tool-runtime/tools/:name", func(*gin.Context) {})
	engine.POST("/api/v1/mcp", func(*gin.Context) {})
	engine.POST("/api/v1/tool-auth/token", func(*gin.Context) {})
	engine.POST("/api/v1/tool-auth/device", func(*gin.Context) {})
	engine.POST("/api/v1/tool-auth/revoke", func(*gin.Context) {})
	engine.GET("/api/v1/platform/project-sets", func(*gin.Context) {})

	inventory := NewRouteInventory()
	inventory.Load(engine.Routes())
	candidates := inventory.ServiceInterfaceCandidates(context.Background())
	if len(candidates) != 1 || candidates[0] != (identity.ServiceInterfaceCandidate{
		HTTPMethod: http.MethodGet, RouteTemplate: "/api/v1/platform/project-sets",
	}) {
		t.Fatalf("candidates=%+v", candidates)
	}
}
