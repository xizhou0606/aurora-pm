package mcp

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

type testExecutor struct {
	definitions []toolruntime.Definition
	result      toolruntime.InvocationResult
	err         error
	invocation  toolruntime.Invocation
	auth        toolruntime.ExecutionAuthorization
	changeSet   toolruntime.ChangeSet
	changeSetID string
	showRuns    int
	confirmRuns int
}

func (executor *testExecutor) Catalog(_ context.Context, authorization toolruntime.ExecutionAuthorization) ([]toolruntime.Definition, error) {
	executor.auth = authorization
	return executor.definitions, executor.err
}

func (executor *testExecutor) Describe(_ context.Context, name string, authorization toolruntime.ExecutionAuthorization) (toolruntime.Definition, error) {
	executor.auth = authorization
	for _, definition := range executor.definitions {
		if definition.Name == name {
			return definition, executor.err
		}
	}
	return toolruntime.Definition{}, toolruntime.ErrToolUnknown
}

func (executor *testExecutor) Invoke(_ context.Context, invocation toolruntime.Invocation) (toolruntime.InvocationResult, error) {
	executor.invocation = invocation
	return executor.result, executor.err
}

func (executor *testExecutor) ShowChangeSet(_ context.Context, id string, authorization toolruntime.ExecutionAuthorization) (toolruntime.ChangeSet, error) {
	executor.showRuns++
	executor.changeSetID, executor.auth = id, authorization
	return executor.changeSet, executor.err
}

func (executor *testExecutor) ConfirmChangeSet(_ context.Context, id string, authorization toolruntime.ExecutionAuthorization) (toolruntime.ChangeSet, error) {
	executor.confirmRuns++
	executor.changeSetID, executor.auth = id, authorization
	return executor.changeSet, executor.err
}

type testResolver struct {
	authorization toolruntime.ExecutionAuthorization
	err           error
}

func (resolver testResolver) ResolveAuthorization(*gin.Context) (toolruntime.ExecutionAuthorization, error) {
	return resolver.authorization, resolver.err
}

func mcpAuthorization() toolruntime.ExecutionAuthorization {
	return toolruntime.ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "codex",
		Channel: toolruntime.DeliveryMCP, Audience: toolruntime.AudienceMCP,
	}
}

func mcpDefinition(name string, lifecycle toolruntime.Lifecycle) toolruntime.Definition {
	return toolruntime.Definition{
		Name: name, Version: "1.2.3", Summary: "Test tool", Lifecycle: lifecycle,
		InputSchema:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer"}}}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"items":{"type":"array"}}}`),
		Authorization: toolruntime.AuthorizationContract{
			IdentityRequired: true, WorkspaceRequired: true, GrantRequired: true, GrantScope: "project:read",
		},
		Risk: toolruntime.RiskRead, ChangeSet: toolruntime.ChangeSetForbidden,
		UAT: toolruntime.UATWithinGrantWindow, AuditEvent: "tool.project.test",
	}
}

func mcpEngine(executor toolruntime.Executor, resolver AuthorizationResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(executor, resolver).RegisterRoutes(engine.Group("/api/v1"))
	return engine
}

func mcpAuthorizationEngine(executor toolruntime.Executor, resolver AuthorizationResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(executor, resolver, "https://pm.example.test/workspace/mcp-cli/authorize?channel=mcp").RegisterRoutes(engine.Group("/api/v1"))
	return engine
}

func mcpRequest(engine *gin.Engine, method, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/api/v1/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	engine.ServeHTTP(response, request)
	return response
}

func TestStreamableHTTPInitializeListAndCall(t *testing.T) {
	active := mcpDefinition("project.list", toolruntime.LifecycleActive)
	draft := mcpDefinition("project.create", toolruntime.LifecycleDraft)
	executor := &testExecutor{
		definitions: []toolruntime.Definition{draft, active},
		result: toolruntime.InvocationResult{
			Name: active.Name, Version: active.Version,
			Output: json.RawMessage(`{"items":[]}`), AuditEvent: active.AuditEvent,
		},
	}
	engine := mcpEngine(executor, testResolver{authorization: mcpAuthorization()})

	initialize := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	if initialize.Code != http.StatusOK || !strings.Contains(initialize.Body.String(), `"tools":{"listChanged":false}`) {
		t.Fatalf("initialize status=%d body=%s", initialize.Code, initialize.Body.String())
	}

	list := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"name":"project.list"`) ||
		strings.Contains(list.Body.String(), `"name":"project.create"`) || !strings.Contains(list.Body.String(), `"inputSchema"`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	call := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"project.list","arguments":{"id":9}}}`)
	if call.Code != http.StatusOK || !strings.Contains(call.Body.String(), `"structuredContent":{"items":[]}`) {
		t.Fatalf("call status=%d body=%s", call.Code, call.Body.String())
	}
	if executor.invocation.Name != active.Name || string(executor.invocation.Input) != `{"id":9}` ||
		executor.invocation.Authorization != mcpAuthorization() {
		t.Fatalf("invocation=%+v", executor.invocation)
	}
}

func TestStreamableHTTPMapsControlledHandlerInputErrorsToInvalidParams(t *testing.T) {
	executor := &testExecutor{err: toolruntime.ErrToolInputInvalid}
	engine := mcpEngine(executor, testResolver{authorization: mcpAuthorization()})
	response := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"task.workflow.status.list","arguments":{"kind":"task","project_id":12}}}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":-32602`) ||
		!strings.Contains(response.Body.String(), `"message":"invalid tool request"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestStreamableHTTPChangeSetMethodsAndMutationMetadataKeepBindingPrivate(t *testing.T) {
	changeSet := toolruntime.ChangeSet{
		ID: "changeset_12345678", Status: toolruntime.ChangeSetPending, ToolName: "project.update", ToolVersion: "1.0.0",
		Risk: toolruntime.RiskMutation, AuditEvent: "tool.project.update", Authorization: mcpAuthorization(),
		InputDigest: strings.Repeat("a", 64), InputSize: 18,
	}
	executor := &testExecutor{
		result:    toolruntime.InvocationResult{Name: changeSet.ToolName, Version: changeSet.ToolVersion, AuditEvent: changeSet.AuditEvent, ChangeSet: &changeSet},
		changeSet: changeSet,
	}
	engine := mcpEngine(executor, testResolver{authorization: mcpAuthorization()})

	call := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"project.update","arguments":{"title":"Updated"},"_meta":{"pm/idempotencyKey":"request-key-0001"}}}`)
	if call.Code != http.StatusOK || executor.invocation.IdempotencyKey != "request-key-0001" || !strings.Contains(call.Body.String(), `"pm/changeSetId":"changeset_12345678"`) {
		t.Fatalf("status=%d invocation=%+v body=%s", call.Code, executor.invocation, call.Body.String())
	}
	assertNoMCPChangeSetBindingFacts(t, call.Body.String())

	for _, method := range []string{"pm/changesets/show", "pm/changesets/confirm"} {
		response := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"`+method+`","params":{"id":"changeset_12345678"}}`)
		if response.Code != http.StatusOK || executor.changeSetID != "changeset_12345678" || executor.auth != mcpAuthorization() {
			t.Fatalf("method=%s status=%d id=%q auth=%+v body=%s", method, response.Code, executor.changeSetID, executor.auth, response.Body.String())
		}
		assertNoMCPChangeSetBindingFacts(t, response.Body.String())
	}
	if executor.showRuns != 1 || executor.confirmRuns != 1 {
		t.Fatalf("show=%d confirm=%d", executor.showRuns, executor.confirmRuns)
	}
}

func TestStreamableHTTPProjectsChangeSetControlsAsStandardTools(t *testing.T) {
	changeSet := toolruntime.ChangeSet{
		ID: "changeset_12345678", Status: toolruntime.ChangeSetPending, ToolName: "task.create", ToolVersion: "1.0.0",
		Risk: toolruntime.RiskMutation, AuditEvent: "tool.task.create", Authorization: mcpAuthorization(),
		InputDigest: strings.Repeat("a", 64), InputSize: 18,
	}
	executor := &testExecutor{changeSet: changeSet}
	engine := mcpEngine(executor, testResolver{authorization: mcpAuthorization()})

	listed := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	var payload struct {
		Result struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
	}
	if listed.Code != http.StatusOK || json.Unmarshal(listed.Body.Bytes(), &payload) != nil {
		t.Fatalf("status=%d body=%s", listed.Code, listed.Body.String())
	}
	byName := make(map[string]Tool, len(payload.Result.Tools))
	for _, tool := range payload.Result.Tools {
		byName[tool.Name] = tool
	}
	show, showOK := byName[toolruntime.ChangeSetShowControlName]
	confirm, confirmOK := byName[toolruntime.ChangeSetConfirmControlName]
	if !showOK || !show.Annotations.ReadOnlyHint || show.Annotations.DestructiveHint ||
		!confirmOK || confirm.Annotations.ReadOnlyHint || !confirm.Annotations.DestructiveHint ||
		!strings.Contains(string(confirm.InputSchema), `"required":["id"]`) {
		t.Fatalf("show=%+v confirm=%+v", show, confirm)
	}

	for _, name := range []string{toolruntime.ChangeSetShowControlName, toolruntime.ChangeSetConfirmControlName} {
		called := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"`+name+`","arguments":{"id":"changeset_12345678"}}}`)
		if called.Code != http.StatusOK || !strings.Contains(called.Body.String(), `"pm/name":"`+name+`"`) ||
			!strings.Contains(called.Body.String(), `"structuredContent":{"id":"changeset_12345678"`) {
			t.Fatalf("name=%s status=%d body=%s", name, called.Code, called.Body.String())
		}
		assertNoMCPChangeSetBindingFacts(t, called.Body.String())
	}
	if executor.showRuns != 1 || executor.confirmRuns != 1 || executor.changeSetID != changeSet.ID || executor.auth != mcpAuthorization() {
		t.Fatalf("show=%d confirm=%d id=%q auth=%+v", executor.showRuns, executor.confirmRuns, executor.changeSetID, executor.auth)
	}

	invalid := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"changeset.confirm","arguments":{"id":"changeset_12345678","unexpected":true}}}`)
	if invalid.Code != http.StatusOK || !strings.Contains(invalid.Body.String(), `"code":-32602`) || executor.confirmRuns != 1 {
		t.Fatalf("status=%d confirm=%d body=%s", invalid.Code, executor.confirmRuns, invalid.Body.String())
	}

	shortID := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"changeset.confirm","arguments":{"id":"short"}}}`)
	if shortID.Code != http.StatusOK || !strings.Contains(shortID.Body.String(), `"code":-32602`) || executor.confirmRuns != 1 {
		t.Fatalf("status=%d confirm=%d body=%s", shortID.Code, executor.confirmRuns, shortID.Body.String())
	}
}

func TestStreamableHTTPChangeSetMethodsDoNotLeakRepositoryErrors(t *testing.T) {
	executor := &testExecutor{err: errors.New("private changeset database password must-not-leak")}
	engine := mcpEngine(executor, testResolver{authorization: mcpAuthorization()})
	for _, method := range []string{"pm/changesets/show", "pm/changesets/confirm"} {
		response := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"`+method+`","params":{"id":"changeset_12345678"}}`)
		if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "must-not-leak") || !strings.Contains(response.Body.String(), `"message":"internal error"`) {
			t.Fatalf("method=%s status=%d body=%s", method, response.Code, response.Body.String())
		}
	}
}

func TestStreamableHTTPRetryableChangeSetUsesStableInProgressCode(t *testing.T) {
	executor := &testExecutor{err: toolruntime.ErrChangeSetInProgress}
	engine := mcpEngine(executor, testResolver{authorization: mcpAuthorization()})
	response := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"pm/changesets/confirm","params":{"id":"changeset_12345678"}}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":-32006`) ||
		!strings.Contains(response.Body.String(), `"message":"change set in progress"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func assertNoMCPChangeSetBindingFacts(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"authorization", "principal_id", "workspace_id", "grant_id", "client_id", "channel", "audience"} {
		if strings.Contains(body, `"`+forbidden+`"`) {
			t.Fatalf("binding fact %q leaked in %s", forbidden, body)
		}
	}
}

func TestStreamableHTTPAcceptsInitializedNotificationAndDeclinesSSE(t *testing.T) {
	engine := mcpEngine(&testExecutor{}, testResolver{authorization: mcpAuthorization()})
	notification := mcpRequest(engine, http.MethodPost, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if notification.Code != http.StatusAccepted || notification.Body.Len() != 0 {
		t.Fatalf("notification status=%d body=%s", notification.Code, notification.Body.String())
	}
	get := mcpRequest(engine, http.MethodGet, "")
	if get.Code != http.StatusMethodNotAllowed || get.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET status=%d allow=%q", get.Code, get.Header().Get("Allow"))
	}
}

func TestStreamableHTTPFailsClosedBeforeExecutor(t *testing.T) {
	executor := &testExecutor{}
	cli := mcpAuthorization()
	cli.Channel, cli.Audience = toolruntime.DeliveryCLI, toolruntime.AudienceCLI
	for name, resolver := range map[string]AuthorizationResolver{
		"missing":       testResolver{err: errors.New("private token error")},
		"wrong channel": testResolver{authorization: cli},
	} {
		t.Run(name, func(t *testing.T) {
			response := mcpRequest(mcpEngine(executor, resolver), http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
			if response.Code != http.StatusUnauthorized || response.Body.Len() != 0 || response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestStreamableHTTPReportsInactiveWorkspaceMembershipWithoutReauthorizationLoop(t *testing.T) {
	response := mcpRequest(
		mcpAuthorizationEngine(&testExecutor{}, testResolver{
			err: toolruntime.ExternalWorkspaceMembershipError{WorkspaceID: 11},
		}),
		http.MethodPost,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	)
	if response.Code != http.StatusForbidden || response.Header().Get("WWW-Authenticate") != "" ||
		!strings.Contains(response.Body.String(), `"code":"MCP_WORKSPACE_MEMBERSHIP_REQUIRED"`) ||
		!strings.Contains(response.Body.String(), `"workspace_id":11`) {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}

func TestStreamableHTTPReturnsManualAuthorizationLinkWithoutLeakingTokenFacts(t *testing.T) {
	response := mcpRequest(
		mcpAuthorizationEngine(&testExecutor{}, testResolver{err: errors.New("private token error")}),
		http.MethodPost,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status=%d authenticate=%q", response.Code, response.Header().Get("WWW-Authenticate"))
	}
	wantURL := "https://pm.example.test/workspace/mcp-cli/authorize?channel=mcp"
	if response.Header().Get("Link") != "<"+wantURL+">; rel=\"authorization\"" ||
		!strings.Contains(response.Body.String(), `"code":"MCP_AUTHORIZATION_REQUIRED"`) ||
		!strings.Contains(response.Body.String(), `"authorization_url":"`+wantURL+`"`) {
		t.Fatalf("headers=%v body=%s", response.Header(), response.Body.String())
	}
	for _, forbidden := range []string{"private token error", "principal_id", "workspace_id", "grant_id", "client_id"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("forbidden fact %q leaked in %s", forbidden, response.Body.String())
		}
	}
}

func TestStreamableHTTPValidatesOriginMediaAndAcceptHeaders(t *testing.T) {
	engine := mcpEngine(&testExecutor{}, testResolver{authorization: mcpAuthorization()})
	base := func() *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		return request
	}

	tests := []struct {
		name   string
		mutate func(*http.Request)
		status int
	}{
		{name: "origin", mutate: func(request *http.Request) { request.Header.Set("Origin", "https://attacker.test") }, status: http.StatusForbidden},
		{name: "content type", mutate: func(request *http.Request) { request.Header.Set("Content-Type", "text/plain") }, status: http.StatusUnsupportedMediaType},
		{name: "accept", mutate: func(request *http.Request) { request.Header.Set("Accept", "application/json") }, status: http.StatusNotAcceptable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base()
			test.mutate(request)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestStreamableHTTPRejectsUnsupportedProtocolVersionHeader(t *testing.T) {
	engine := mcpEngine(&testExecutor{}, testResolver{authorization: mcpAuthorization()})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2099-01-01")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "unsupported protocol version") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestStreamableHTTPDoesNotLeakExecutionErrors(t *testing.T) {
	executor := &testExecutor{err: errors.New("database password must-not-leak")}
	response := mcpRequest(
		mcpEngine(executor, testResolver{authorization: mcpAuthorization()}),
		http.MethodPost,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "must-not-leak") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
