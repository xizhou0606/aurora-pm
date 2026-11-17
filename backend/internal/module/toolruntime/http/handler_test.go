package toolruntimehttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type fakeExecutor struct {
	catalog        []toolruntime.Definition
	catalogErr     error
	description    toolruntime.Definition
	describeErr    error
	result         toolruntime.InvocationResult
	invokeErr      error
	auth           toolruntime.ExecutionAuthorization
	invocation     toolruntime.Invocation
	catalogRuns    int
	describeRuns   int
	invokeRuns     int
	changeSet      toolruntime.ChangeSet
	changeSetErr   error
	changeSetID    string
	changeSetAuth  toolruntime.ExecutionAuthorization
	showRuns       int
	confirmRuns    int
	preflightItems []toolruntime.ToolPreflightItem
	preflightErr   error
}

func (executor *fakeExecutor) Preflight(_ context.Context, _ []string, authorization toolruntime.ExecutionAuthorization) ([]toolruntime.ToolPreflightItem, error) {
	executor.auth = authorization
	return executor.preflightItems, executor.preflightErr
}

func (executor *fakeExecutor) Catalog(_ context.Context, authorization toolruntime.ExecutionAuthorization) ([]toolruntime.Definition, error) {
	executor.catalogRuns++
	executor.auth = authorization
	return executor.catalog, executor.catalogErr
}

func (executor *fakeExecutor) Describe(_ context.Context, _ string, authorization toolruntime.ExecutionAuthorization) (toolruntime.Definition, error) {
	executor.describeRuns++
	executor.auth = authorization
	return executor.description, executor.describeErr
}

func (executor *fakeExecutor) Invoke(_ context.Context, invocation toolruntime.Invocation) (toolruntime.InvocationResult, error) {
	executor.invokeRuns++
	executor.invocation = invocation
	return executor.result, executor.invokeErr
}

func (executor *fakeExecutor) ShowChangeSet(_ context.Context, id string, authorization toolruntime.ExecutionAuthorization) (toolruntime.ChangeSet, error) {
	executor.showRuns++
	executor.changeSetID, executor.changeSetAuth = id, authorization
	return executor.changeSet, executor.changeSetErr
}

func (executor *fakeExecutor) ConfirmChangeSet(_ context.Context, id string, authorization toolruntime.ExecutionAuthorization) (toolruntime.ChangeSet, error) {
	executor.confirmRuns++
	executor.changeSetID, executor.changeSetAuth = id, authorization
	return executor.changeSet, executor.changeSetErr
}

func httpAuthorization() toolruntime.ExecutionAuthorization {
	return toolruntime.ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "pm-cli",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
}

func testDefinition(name string, lifecycle toolruntime.Lifecycle) toolruntime.Definition {
	return toolruntime.Definition{
		Name: name, Version: "1.0.0", Summary: "Test tool", Lifecycle: lifecycle,
		InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`),
		Authorization: toolruntime.AuthorizationContract{
			IdentityRequired: true, WorkspaceRequired: true, GrantRequired: true, GrantScope: "project:read",
		},
		Risk: toolruntime.RiskRead, ChangeSet: toolruntime.ChangeSetForbidden,
		UAT: toolruntime.UATWithinGrantWindow, AuditEvent: "tool.project.test",
	}
}

func toolRuntimeEngine(executor toolruntime.Executor, resolver AuthorizationResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(executor, resolver).RegisterRoutes(engine.Group("/api/v1/tool-runtime"))
	return engine
}

func allowedResolver() AuthorizationResolver {
	return AuthorizationResolverFunc(func(*gin.Context) (toolruntime.ExecutionAuthorization, error) {
		return httpAuthorization(), nil
	})
}

func serve(engine *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(response, request)
	return response
}

func responseErrorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Error.Code
}

func TestCatalogRequiresAuthorizationAndFiltersDraftDefensively(t *testing.T) {
	active := testDefinition("project.list", toolruntime.LifecycleActive)
	draft := testDefinition("project.create", toolruntime.LifecycleDraft)
	executor := &fakeExecutor{catalog: []toolruntime.Definition{draft, active}}
	engine := toolRuntimeEngine(executor, allowedResolver())

	response := serve(engine, http.MethodGet, "/api/v1/tool-runtime/catalog", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Tools []toolruntime.Definition `json:"tools"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Tools) != 1 || payload.Data.Tools[0].Name != active.Name {
		t.Fatalf("tools=%+v", payload.Data.Tools)
	}
	if executor.auth != httpAuthorization() {
		t.Fatalf("authorization=%+v", executor.auth)
	}

	denied := &fakeExecutor{}
	deniedEngine := toolRuntimeEngine(denied, AuthorizationResolverFunc(func(*gin.Context) (toolruntime.ExecutionAuthorization, error) {
		return toolruntime.ExecutionAuthorization{}, errors.New("missing grant")
	}))
	response = serve(deniedEngine, http.MethodGet, "/api/v1/tool-runtime/catalog", "")
	if response.Code != http.StatusForbidden || responseErrorCode(t, response) != ErrorToolAuthorizationDenied {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if denied.catalogRuns != 0 {
		t.Fatal("executor must not run when authorization facts cannot be resolved")
	}
}

func TestCatalogReportsInactiveWorkspaceMembership(t *testing.T) {
	executor := &fakeExecutor{}
	resolver := AuthorizationResolverFunc(func(*gin.Context) (toolruntime.ExecutionAuthorization, error) {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ExternalWorkspaceMembershipError{WorkspaceID: 11}
	})
	response := serve(toolRuntimeEngine(executor, resolver), http.MethodGet, "/api/v1/tool-runtime/catalog", "")
	if response.Code != http.StatusForbidden || responseErrorCode(t, response) != ErrorToolWorkspaceMembership ||
		!strings.Contains(response.Body.String(), `"workspace_id":11`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if executor.catalogRuns != 0 {
		t.Fatal("executor must not run when workspace membership is inactive")
	}
}

func TestPreflightReturnsOnlyRequestedStatusesWithCanonicalActionCode(t *testing.T) {
	executor := &fakeExecutor{preflightItems: []toolruntime.ToolPreflightItem{
		{Tool: "task.list", Status: toolruntime.ToolPreflightAvailable},
		{Tool: "task.create", Status: toolruntime.ToolPreflightNotGranted, DenialReason: toolruntime.ToolPreflightDeniedByGrant},
		{Tool: "task.update", Status: toolruntime.ToolPreflightConditional},
	}}
	response := serve(toolRuntimeEngine(executor, allowedResolver()), http.MethodPost, "/api/v1/tool-runtime/preflight", `{"tools":["task.list","task.create","task.update"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Items []struct {
				Tool         string `json:"tool"`
				Status       string `json:"status"`
				ActionCode   string `json:"action_code"`
				Conditional  bool   `json:"conditional"`
				ObjectPolicy string `json:"object_policy"`
				DenialReason string `json:"denial_reason"`
			} `json:"items"`
			ManagementPath       string `json:"management_path"`
			ActionManagementPath string `json:"action_management_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Items) != 3 || payload.Data.Items[0].ActionCode == "" || payload.Data.Items[1].ActionCode == "" ||
		payload.Data.Items[1].DenialReason != string(toolruntime.ToolPreflightDeniedByGrant) ||
		payload.Data.Items[2].Status != string(toolruntime.ToolPreflightAvailable) || !payload.Data.Items[2].Conditional ||
		payload.Data.Items[2].ObjectPolicy != "task.creator" || payload.Data.ManagementPath != "/workspace/mcp-cli" ||
		payload.Data.ActionManagementPath != "/workspace/pm/access-management" {
		t.Fatalf("payload=%s", response.Body.String())
	}
}

func TestCatalogAndDescribeHideScopeExcludedOrDraftTools(t *testing.T) {
	for name, executor := range map[string]*fakeExecutor{
		"scope excluded":             {describeErr: toolruntime.ErrToolUnknown},
		"draft returned defensively": {description: testDefinition("project.create", toolruntime.LifecycleDraft)},
	} {
		t.Run(name, func(t *testing.T) {
			response := serve(toolRuntimeEngine(executor, allowedResolver()), http.MethodGet, "/api/v1/tool-runtime/tools/project.create", "")
			if response.Code != http.StatusNotFound || responseErrorCode(t, response) != ErrorToolNotFound {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestToolRuntimeHTTPRejectsIncompleteOrWrongAudienceFacts(t *testing.T) {
	mcp := httpAuthorization()
	mcp.Channel = toolruntime.DeliveryMCP
	mcp.Audience = toolruntime.AudienceMCP
	for name, authorization := range map[string]toolruntime.ExecutionAuthorization{
		"zero facts":               {},
		"MCP grant on CLI adapter": mcp,
	} {
		t.Run(name, func(t *testing.T) {
			executor := &fakeExecutor{}
			resolver := AuthorizationResolverFunc(func(*gin.Context) (toolruntime.ExecutionAuthorization, error) {
				return authorization, nil
			})
			response := serve(toolRuntimeEngine(executor, resolver), http.MethodGet, "/api/v1/tool-runtime/catalog", "")
			if response.Code != http.StatusForbidden || responseErrorCode(t, response) != ErrorToolAuthorizationDenied {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if executor.catalogRuns != 0 {
				t.Fatal("executor must not run with invalid adapter authorization facts")
			}
		})
	}
}

func TestInvokePassesOnlyResolvedAuthorizationAndRawInput(t *testing.T) {
	executor := &fakeExecutor{result: toolruntime.InvocationResult{
		Name: "project.list", Version: "1.0.0", Output: json.RawMessage(`{"items":[]}`), AuditEvent: "tool.project.list",
	}}
	response := serve(toolRuntimeEngine(executor, allowedResolver()), http.MethodPost, "/api/v1/tool-runtime/tools/project.list/invoke", `{"workspace_id":11}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if executor.invocation.Name != "project.list" || string(executor.invocation.Input) != `{"workspace_id":11}` {
		t.Fatalf("invocation=%+v", executor.invocation)
	}
	if executor.invocation.Authorization != httpAuthorization() {
		t.Fatalf("authorization=%+v", executor.invocation.Authorization)
	}
	if !strings.Contains(response.Body.String(), `"audit_event":"tool.project.list"`) {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestMutationInvokeAndChangeSetEndpointsKeepBindingFactsPrivate(t *testing.T) {
	changeSet := toolruntime.ChangeSet{
		ID: "changeset_12345678", Status: toolruntime.ChangeSetPending, ToolName: "project.update", ToolVersion: "1.0.0",
		Risk: toolruntime.RiskMutation, AuditEvent: "tool.project.update", Authorization: httpAuthorization(),
		InputDigest: strings.Repeat("a", 64), InputSize: 18,
	}
	executor := &fakeExecutor{
		result:    toolruntime.InvocationResult{Name: changeSet.ToolName, Version: changeSet.ToolVersion, AuditEvent: changeSet.AuditEvent, ChangeSet: &changeSet},
		changeSet: changeSet,
	}
	engine := toolRuntimeEngine(executor, allowedResolver())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/tool-runtime/tools/project.update/invoke", strings.NewReader(`{"title":"Updated"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-key-0001")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || executor.invocation.IdempotencyKey != "request-key-0001" {
		t.Fatalf("status=%d invocation=%+v body=%s", response.Code, executor.invocation, response.Body.String())
	}
	assertNoChangeSetBindingFacts(t, response.Body.String())

	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/tool-runtime/changesets/changeset_12345678"},
		{method: http.MethodPost, path: "/api/v1/tool-runtime/changesets/changeset_12345678/confirm"},
	} {
		response = serve(engine, endpoint.method, endpoint.path, "")
		if response.Code != http.StatusOK || executor.changeSetID != "changeset_12345678" || executor.changeSetAuth != httpAuthorization() {
			t.Fatalf("endpoint=%s status=%d id=%q auth=%+v body=%s", endpoint.path, response.Code, executor.changeSetID, executor.changeSetAuth, response.Body.String())
		}
		assertNoChangeSetBindingFacts(t, response.Body.String())
	}
}

func TestChangeSetEndpointsDoNotLeakRepositoryErrors(t *testing.T) {
	executor := &fakeExecutor{changeSetErr: errors.New("private changeset database password must-not-leak")}
	engine := toolRuntimeEngine(executor, allowedResolver())
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/tool-runtime/changesets/changeset_12345678"},
		{method: http.MethodPost, path: "/api/v1/tool-runtime/changesets/changeset_12345678/confirm"},
	} {
		response := serve(engine, endpoint.method, endpoint.path, "")
		if response.Code != http.StatusInternalServerError || responseErrorCode(t, response) != ErrorToolExecutionFailed || strings.Contains(response.Body.String(), "must-not-leak") {
			t.Fatalf("endpoint=%s status=%d body=%s", endpoint.path, response.Code, response.Body.String())
		}
	}
}

func assertNoChangeSetBindingFacts(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"authorization", "principal_id", "workspace_id", "grant_id", "client_id", "channel", "audience"} {
		if strings.Contains(body, `"`+forbidden+`"`) {
			t.Fatalf("binding fact %q leaked in %s", forbidden, body)
		}
	}
}

func TestInvokeMapsStableFailClosedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "unknown", err: toolruntime.ErrToolUnknown, status: http.StatusNotFound, code: ErrorToolNotFound},
		{name: "draft", err: toolruntime.ErrToolDraft, status: http.StatusConflict, code: ErrorToolDraftUnavailable},
		{name: "UAT blocked", err: toolruntime.ErrToolUATBlocked, status: http.StatusForbidden, code: ErrorToolUATBlocked},
		{name: "authorization", err: toolruntime.ErrAuthorizationDenied, status: http.StatusForbidden, code: ErrorToolAuthorizationDenied},
		{name: "scope", err: toolruntime.ErrToolScopeDenied, status: http.StatusForbidden, code: ErrorToolAuthorizationDenied},
		{name: "change set", err: toolruntime.ErrChangeSetRequired, status: http.StatusConflict, code: ErrorToolChangeSetRequired},
		{name: "change set in progress", err: toolruntime.ErrChangeSetInProgress, status: http.StatusConflict, code: ErrorChangeSetInProgress},
		{name: "input", err: toolruntime.ErrToolInputInvalid, status: http.StatusUnprocessableEntity, code: ErrorToolInputInvalid},
		{name: "unavailable", err: toolruntime.ErrToolUnavailable, status: http.StatusServiceUnavailable, code: ErrorToolUnavailable},
		{name: "execution", err: errors.New("private failure"), status: http.StatusInternalServerError, code: ErrorToolExecutionFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &fakeExecutor{invokeErr: test.err}
			response := serve(toolRuntimeEngine(executor, allowedResolver()), http.MethodPost, "/api/v1/tool-runtime/tools/project.list/invoke", `{}`)
			if response.Code != test.status || responseErrorCode(t, response) != test.code {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "private failure") {
				t.Fatalf("internal error leaked: %s", response.Body.String())
			}
		})
	}
}

func TestInvokeRejectsInvalidOrOversizedJSONBeforeExecutor(t *testing.T) {
	for name, body := range map[string]string{
		"empty":           "",
		"malformed":       `{"project_id":`,
		"multiple values": `{} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			executor := &fakeExecutor{}
			response := serve(toolRuntimeEngine(executor, allowedResolver()), http.MethodPost, "/api/v1/tool-runtime/tools/project.list/invoke", body)
			if response.Code != http.StatusUnprocessableEntity || responseErrorCode(t, response) != ErrorToolInputInvalid {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if executor.invokeRuns != 0 {
				t.Fatal("executor must not receive invalid JSON")
			}
		})
	}

	executor := &fakeExecutor{}
	large := `{"value":"` + strings.Repeat("x", maxInvocationBodyBytes) + `"}`
	response := serve(toolRuntimeEngine(executor, allowedResolver()), http.MethodPost, "/api/v1/tool-runtime/tools/project.list/invoke", large)
	if response.Code != http.StatusRequestEntityTooLarge || responseErrorCode(t, response) != ErrorToolInputTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if executor.invokeRuns != 0 {
		t.Fatal("executor must not receive oversized JSON")
	}
}

func TestInvokeFailsClosedBeforeParsingInputWhenResolverFails(t *testing.T) {
	executor := &fakeExecutor{}
	resolver := AuthorizationResolverFunc(func(*gin.Context) (toolruntime.ExecutionAuthorization, error) {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ErrToolUATBlocked
	})
	response := serve(toolRuntimeEngine(executor, resolver), http.MethodPost, "/api/v1/tool-runtime/tools/project.list/invoke", `{"broken":`)
	if response.Code != http.StatusForbidden || responseErrorCode(t, response) != ErrorToolUATBlocked {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if executor.invokeRuns != 0 {
		t.Fatal("executor must not run outside the grant window")
	}
}
