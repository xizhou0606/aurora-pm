package toolruntimehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestManagementHandlerAuthorizationFlowIsScopedAndSingleUse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	principal := toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "研发"}
	engine := managementTestEngine(t, &principal)

	catalog := managementServe(engine, http.MethodGet, "/api/v1/platform/tool-runtime/catalog", "")
	if catalog.Code != http.StatusOK || !strings.Contains(catalog.Body.String(), `"name":"workspace.member.list"`) ||
		!strings.Contains(catalog.Body.String(), `"lifecycle":"draft"`) || !strings.Contains(catalog.Body.String(), "project.list") {
		t.Fatalf("catalog status=%d body=%s", catalog.Code, catalog.Body.String())
	}
	created := managementServe(engine, http.MethodPost, "/api/v1/platform/external-tooling/authorization-requests", `{
		"channel":"cli"
	}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	requestID := managementRequestID(t, created)

	principal.WorkspaceID = 12
	unauthorized := managementServe(engine, http.MethodGet, "/api/v1/platform/external-tooling/authorization-requests/"+requestID, "")
	if unauthorized.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}
	principal.WorkspaceID = 11
	request := managementServe(engine, http.MethodGet, "/api/v1/platform/external-tooling/authorization-requests/"+requestID, "")
	if request.Code != http.StatusOK || !strings.Contains(request.Body.String(), `"grant_scope":"project:read"`) || !strings.Contains(request.Body.String(), `"name":"PM CLI"`) {
		t.Fatalf("request status=%d body=%s", request.Code, request.Body.String())
	}

	authorized := managementServe(engine, http.MethodPost, "/api/v1/platform/external-tooling/authorization-requests/"+requestID+"/decision", `{"decision":"authorize"}`)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("authorize status=%d body=%s", authorized.Code, authorized.Body.String())
	}
	effective := managementServe(engine, http.MethodGet, "/api/v1/platform/external-tooling/effective-access", "")
	if effective.Code != http.StatusOK || !strings.Contains(effective.Body.String(), `"status":"active"`) ||
		!strings.Contains(effective.Body.String(), `"name":"project.list"`) || strings.Contains(effective.Body.String(), "project.search") {
		t.Fatalf("effective access status=%d body=%s", effective.Code, effective.Body.String())
	}
	replay := managementServe(engine, http.MethodPost, "/api/v1/platform/external-tooling/authorization-requests/"+requestID+"/decision", `{"decision":"authorize"}`)
	if replay.Code != http.StatusConflict || !strings.Contains(replay.Body.String(), "AUTHORIZATION_REQUEST_DECIDED") {
		t.Fatalf("replay status=%d body=%s", replay.Code, replay.Body.String())
	}
	grants := managementServe(engine, http.MethodGet, "/api/v1/platform/external-tooling/grants", "")
	if grants.Code != http.StatusOK || !strings.Contains(grants.Body.String(), `"tool_scopes":["project.list"]`) ||
		strings.Contains(grants.Body.String(), "project.search") || !strings.Contains(grants.Body.String(), `"status":"active"`) {
		t.Fatalf("grants status=%d body=%s", grants.Code, grants.Body.String())
	}
}

func TestManagementHandlerRejectsInvalidScopeBodyAndSession(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	principal := toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "研发"}
	engine := managementTestEngine(t, &principal)
	path := "/api/v1/platform/external-tooling/configuration"
	for name, test := range map[string]struct {
		body   string
		status int
		code   string
	}{
		"draft":         {fmt.Sprintf(`{"mode":"window","tool_scopes":["workspace.member.list"],"effective_from":%q,"expires_at":%q}`, now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)), http.StatusUnprocessableEntity, "AUTHORIZATION_SCOPE_INVALID"},
		"duplicate":     {fmt.Sprintf(`{"mode":"window","tool_scopes":["project.list","project.list"],"effective_from":%q,"expires_at":%q}`, now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)), http.StatusBadRequest, "EXTERNAL_TOOLING_CONFIGURATION_INVALID"},
		"unknown field": {`{"unexpected":true}`, http.StatusBadRequest, "EXTERNAL_TOOLING_CONFIGURATION_INVALID"},
		"ttl too large": {`{"mode":"permanent","tool_scopes":["project.list"],"cli_grant_ttl_hours":8761}`, http.StatusBadRequest, "EXTERNAL_TOOLING_CONFIGURATION_INVALID"},
	} {
		t.Run(name, func(t *testing.T) {
			response := managementServe(engine, http.MethodPut, path, test.body)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}

	principal = toolruntime.ManagementPrincipal{}
	unauthorizedEngine := managementTestEngine(t, &principal)
	response := managementServe(unauthorizedEngine, http.MethodGet, "/api/v1/platform/tool-runtime/catalog", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestManagementHandlerReadsAndSavesRealWorkspaceConfiguration(t *testing.T) {
	principal := toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "研发"}
	engine := managementTestEngine(t, &principal)
	current := managementServe(engine, http.MethodGet, "/api/v1/platform/external-tooling/configuration", "")
	if current.Code != http.StatusOK || !strings.Contains(current.Body.String(), `"tool_scopes":["project.list"]`) ||
		!strings.Contains(current.Body.String(), `"version":1`) {
		t.Fatalf("current status=%d body=%s", current.Code, current.Body.String())
	}

	now := time.Now().UTC().Truncate(time.Second)
	updated := managementServe(engine, http.MethodPut, "/api/v1/platform/external-tooling/configuration", fmt.Sprintf(`{
		"mode":"window","tool_scopes":["project.search"],"effective_from":%q,"expires_at":%q,"cli_grant_ttl_hours":30
	}`, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339)))
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"tool_scopes":["project.search"]`) ||
		!strings.Contains(updated.Body.String(), `"grant_scopes":["project:read"]`) ||
		!strings.Contains(updated.Body.String(), `"updated_by":7`) || !strings.Contains(updated.Body.String(), `"version":2`) ||
		!strings.Contains(updated.Body.String(), `"cli_grant_ttl_hours":30`) {
		t.Fatalf("updated status=%d body=%s", updated.Code, updated.Body.String())
	}

	principal.WorkspaceID = 12
	missing := managementServe(engine, http.MethodGet, "/api/v1/platform/external-tooling/configuration", "")
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "EXTERNAL_TOOLING_CONFIGURATION_NOT_FOUND") {
		t.Fatalf("missing status=%d body=%s", missing.Code, missing.Body.String())
	}
}

func TestManagementHandlerConfigurationModeContractUsesNullTimesOutsideWindow(t *testing.T) {
	principal := toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "研发"}
	engine := managementTestEngine(t, &principal)
	path := "/api/v1/platform/external-tooling/configuration"

	permanent := managementServe(engine, http.MethodPut, path, `{
		"mode":"permanent","tool_scopes":["project.list"],"effective_from":null,"expires_at":null
	}`)
	if permanent.Code != http.StatusOK || !strings.Contains(permanent.Body.String(), `"mode":"permanent"`) ||
		!strings.Contains(permanent.Body.String(), `"effective_from":null`) || !strings.Contains(permanent.Body.String(), `"expires_at":null`) {
		t.Fatalf("permanent status=%d body=%s", permanent.Code, permanent.Body.String())
	}
	created := managementServe(engine, http.MethodPost, "/api/v1/platform/external-tooling/authorization-requests", `{"channel":"mcp"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("permanent request create status=%d body=%s", created.Code, created.Body.String())
	}
	request := managementServe(
		engine,
		http.MethodGet,
		"/api/v1/platform/external-tooling/authorization-requests/"+managementRequestID(t, created),
		"",
	)
	if request.Code != http.StatusOK || !strings.Contains(request.Body.String(), `"mode":"permanent"`) ||
		!strings.Contains(request.Body.String(), `"effective_from":null`) || !strings.Contains(request.Body.String(), `"expires_at":null`) {
		t.Fatalf("permanent request status=%d body=%s", request.Code, request.Body.String())
	}

	disabled := managementServe(engine, http.MethodPut, path, `{
		"mode":"disabled","tool_scopes":["project.list"],"effective_from":null,"expires_at":null
	}`)
	if disabled.Code != http.StatusOK || !strings.Contains(disabled.Body.String(), `"mode":"disabled"`) ||
		!strings.Contains(disabled.Body.String(), `"tool_scopes":["project.list"]`) ||
		!strings.Contains(disabled.Body.String(), `"effective_from":null`) || !strings.Contains(disabled.Body.String(), `"expires_at":null`) {
		t.Fatalf("disabled status=%d body=%s", disabled.Code, disabled.Body.String())
	}

	for name, body := range map[string]string{
		"permanent with window": `{"mode":"permanent","tool_scopes":["project.list"],"effective_from":"2026-08-27T04:00:00Z","expires_at":"2026-08-28T04:00:00Z"}`,
		"window without times":  `{"mode":"window","tool_scopes":["project.list"],"effective_from":null,"expires_at":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := managementServe(engine, http.MethodPut, path, body)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "EXTERNAL_TOOLING_CONFIGURATION_INVALID") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func managementTestEngine(t *testing.T, principal *toolruntime.ManagementPrincipal) *gin.Engine {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&toolruntimegorm.ExternalAuthorizationGrant{}, &toolruntimegorm.ExternalAuthorizationRequest{}, &toolruntimegorm.ExternalToolingWorkspaceConfiguration{}); err != nil {
		t.Fatal(err)
	}
	active := managementDefinition("project.list", "project:read", toolruntime.LifecycleActive)
	sharedScope := managementDefinition("project.search", "project:read", toolruntime.LifecycleActive)
	draft := managementDefinition("workspace.member.list", "workspace:read", toolruntime.LifecycleDraft)
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{active, sharedScope, draft})
	if err != nil {
		t.Fatal(err)
	}
	requests, grants := toolruntimegorm.NewAuthorizationRequestRepository(db), toolruntimegorm.NewGrantRepository(db)
	configurations := toolruntimegorm.NewWorkspaceConfigurationRepository(db)
	seedNow := time.Now().UTC().Truncate(time.Second)
	if _, err := configurations.Save(context.Background(), toolruntime.WorkspaceConfiguration{
		WorkspaceID: 11, Mode: toolruntime.WorkspaceConfigurationWindow, AllowedTools: []string{"project.list"}, AllowedScopes: []string{"project:read"},
		EffectiveFrom: seedNow.Add(-time.Hour), ExpiresAt: seedNow.Add(24 * time.Hour), UpdatedBy: 7,
		CreatedAt: seedNow.Add(-time.Hour), UpdatedAt: seedNow.Add(-time.Hour), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	service, err := toolruntime.NewAuthorizationService(catalog, requests, grants, configurations)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewManagementHandler(service, ManagementPrincipalResolverFunc(func(*gin.Context) (toolruntime.ManagementPrincipal, error) {
		if principal == nil {
			return toolruntime.ManagementPrincipal{}, errors.New("missing session")
		}
		return *principal, nil
	}))
	engine := gin.New()
	handler.RegisterRoutes(engine.Group("/api/v1/platform"))
	return engine
}

func managementDefinition(name, scope string, lifecycle toolruntime.Lifecycle) toolruntime.Definition {
	return toolruntime.Definition{
		Name: name, Version: "1.0.0", Summary: "List workspace data", Lifecycle: lifecycle,
		InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`),
		Authorization: toolruntime.AuthorizationContract{
			IdentityRequired: true, WorkspaceRequired: true, GrantRequired: true, GrantScope: scope,
		},
		Risk: toolruntime.RiskRead, ChangeSet: toolruntime.ChangeSetForbidden,
		UAT: toolruntime.UATWithinGrantWindow, AuditEvent: "tool." + name,
	}
}

func managementServe(engine http.Handler, method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(response, request)
	return response
}

func managementRequestID(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Data struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Data.RequestID == "" {
		t.Fatalf("request id body=%s err=%v", response.Body.String(), err)
	}
	return body.Data.RequestID
}
