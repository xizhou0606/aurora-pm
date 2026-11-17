package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"
)

type executorAuthorizer struct {
	grantError error
	tools      map[string]bool
	scopes     map[string]bool
}

func (authorizer *executorAuthorizer) AuthorizeGrant(context.Context, ExecutionAuthorization) (AuthorizedGrant, error) {
	if authorizer.grantError != nil {
		return AuthorizedGrant{}, authorizer.grantError
	}
	scopes := authorizer.scopes
	if scopes == nil {
		scopes = map[string]bool{"project:read": true}
	}
	tools := authorizer.tools
	if tools == nil {
		tools = map[string]bool{"project.list": true, "project.archive": true, "project.update": true}
	}
	allowedTools := make([]string, 0, len(tools))
	for tool, allowed := range tools {
		if allowed {
			allowedTools = append(allowedTools, tool)
		}
	}
	allowedScopes := make([]string, 0, len(scopes))
	for scope, allowed := range scopes {
		if allowed {
			allowedScopes = append(allowedScopes, scope)
		}
	}
	return NewAuthorizedGrant(allowedTools, allowedScopes)
}

func TestApplicationRequiresExactToolAndScopeEntitlement(t *testing.T) {
	allowed := validDefinition()
	hidden := validDefinition()
	hidden.Name, hidden.AuditEvent = "project.search", "tool.project.search"
	application := newExecutionApplication(t, []Definition{allowed, hidden}, &executorAuthorizer{
		tools: map[string]bool{allowed.Name: true}, scopes: map[string]bool{"project:read": true},
	})

	definitions, err := application.Catalog(context.Background(), executionAuthorization())
	if err != nil || len(definitions) != 1 || definitions[0].Name != allowed.Name {
		t.Fatalf("catalog=%+v err=%v", definitions, err)
	}
	if _, err := application.Describe(context.Background(), hidden.Name, executionAuthorization()); !errors.Is(err, ErrToolUnknown) {
		t.Fatalf("same-scope describe err=%v", err)
	}
	if _, err := application.Invoke(context.Background(), Invocation{
		Name: hidden.Name, Input: json.RawMessage(`{"workspace_id":11}`), Authorization: executionAuthorization(),
	}); !errors.Is(err, ErrToolScopeDenied) {
		t.Fatalf("same-scope invoke err=%v", err)
	}
}

func TestApplicationPreflightClassifiesExplicitToolsWithoutEnumeration(t *testing.T) {
	allowed := validDefinition()
	hidden := validDefinition()
	hidden.Name, hidden.AuditEvent = "project.search", "tool.project.search"
	draft := validDefinition()
	draft.Name, draft.AuditEvent, draft.Lifecycle = "project.create", "tool.project.create", LifecycleDraft
	application := newExecutionApplication(t, []Definition{allowed, hidden, draft}, &executorAuthorizer{
		tools: map[string]bool{allowed.Name: true}, scopes: map[string]bool{"project:read": true},
	})

	items, err := application.Preflight(context.Background(), []string{allowed.Name, hidden.Name, draft.Name, "project.missing", allowed.Name}, executionAuthorization())
	if err != nil {
		t.Fatal(err)
	}
	want := []ToolPreflightStatus{ToolPreflightAvailable, ToolPreflightNotGranted, ToolPreflightDraft, ToolPreflightUnknown}
	if len(items) != len(want) {
		t.Fatalf("items=%+v", items)
	}
	for index := range want {
		if items[index].Status != want[index] {
			t.Fatalf("item[%d]=%+v want=%s", index, items[index], want[index])
		}
	}
	if items[1].DenialReason != ToolPreflightDeniedByGrant {
		t.Fatalf("hidden item denial reason=%q", items[1].DenialReason)
	}
	if items[0].DenialReason != "" || items[2].DenialReason != "" || items[3].DenialReason != "" {
		t.Fatalf("non-denied items must not report a denial reason: %+v", items)
	}
}

func TestApplicationPreservesControlledHandlerInputErrors(t *testing.T) {
	definition := validDefinition()
	application, err := NewApplication(
		mustCatalog(t, definition),
		&executorAuthorizer{},
		map[string]ToolHandler{definition.Name: ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) {
			return nil, fmt.Errorf("invalid workflow scope: %w", ErrToolInputInvalid)
		})},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"workspace_id":11}`), Authorization: executionAuthorization(),
	})
	if !errors.Is(err, ErrToolInputInvalid) || errors.Is(err, ErrToolExecution) {
		t.Fatalf("err=%v", err)
	}
}

func TestApplicationImmediatelyAppliesWorkspaceConfigurationToExistingGrant(t *testing.T) {
	now := time.Date(2026, 8, 27, 5, 0, 0, 0, time.UTC)
	grant, err := NewGrant(GrantRequest{
		PrincipalID: 7, WorkspaceID: 11, ClientID: "pm-cli", Channel: DeliveryCLI,
		AllowedTools: []string{"project.list", "project.search"}, AllowedScopes: []string{"project:read"},
		EffectiveFrom: now.Add(-time.Hour), ExpiresAt: now.Add(24 * time.Hour),
	}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 13
	repository := &authorizerGrantRepository{
		grant:         grant,
		configuration: testWorkspaceConfiguration(11, []string{"project.list"}, []string{"project:read"}, now.Add(-time.Hour), now.Add(time.Hour)),
	}
	authorizer, err := NewGrantAuthorizer(repository, func() time.Time { return now }, repository)
	if err != nil {
		t.Fatal(err)
	}
	visible := validDefinition()
	hidden := validDefinition()
	hidden.Name, hidden.AuditEvent = "project.search", "tool.project.search"
	application := newExecutionApplication(t, []Definition{visible, hidden}, authorizer)

	definitions, err := application.Catalog(context.Background(), executionAuthorization())
	if err != nil || len(definitions) != 1 || definitions[0].Name != visible.Name {
		t.Fatalf("catalog=%+v err=%v", definitions, err)
	}
	if _, err := application.Describe(context.Background(), hidden.Name, executionAuthorization()); !errors.Is(err, ErrToolUnknown) {
		t.Fatalf("disabled describe err=%v", err)
	}
	if _, err := application.Invoke(context.Background(), Invocation{
		Name: hidden.Name, Input: json.RawMessage(`{"workspace_id":11}`), Authorization: executionAuthorization(),
	}); !errors.Is(err, ErrToolScopeDenied) {
		t.Fatalf("disabled invoke err=%v", err)
	}

	repository.configuration.ExpiresAt = now
	if _, err := application.Catalog(context.Background(), executionAuthorization()); !errors.Is(err, ErrToolUATBlocked) {
		t.Fatalf("expired configuration catalog err=%v", err)
	}
}

func executionAuthorization() ExecutionAuthorization {
	return ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "pm-cli",
		Channel: DeliveryCLI, Audience: AudienceCLI,
	}
}

func executionHandler() ToolHandler {
	return ToolHandlerFunc(func(_ context.Context, call ToolCall) (json.RawMessage, error) {
		return json.RawMessage(`{"items":[],"workspace_id":` + strconv.FormatUint(uint64(call.Authorization.WorkspaceID), 10) + `}`), nil
	})
}

func newExecutionApplication(t *testing.T, definitions []Definition, authorizer InvocationAuthorizer) *Application {
	t.Helper()
	catalog, err := NewCatalog(definitions)
	if err != nil {
		t.Fatal(err)
	}
	handlers := make(map[string]ToolHandler)
	for _, definition := range definitions {
		if definition.Lifecycle != LifecycleDraft {
			handlers[definition.Name] = executionHandler()
		}
	}
	application, err := NewApplication(catalog, authorizer, handlers)
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func TestApplicationCatalogIsGrantScopedAndHidesDrafts(t *testing.T) {
	visible := validDefinition()
	hidden := validDefinition()
	hidden.Name = "quality.list"
	hidden.Authorization.GrantScope = "quality:read"
	hidden.AuditEvent = "tool.quality.list"
	draft := validDefinition()
	draft.Name = "project.create"
	draft.Lifecycle = LifecycleDraft
	draft.Authorization.GrantScope = "project:write"
	draft.AuditEvent = "tool.project.create"
	authorizer := &executorAuthorizer{scopes: map[string]bool{"project:read": true}}
	application := newExecutionApplication(t, []Definition{hidden, draft, visible}, authorizer)

	definitions, err := application.Catalog(context.Background(), executionAuthorization())
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 || definitions[0].Name != visible.Name {
		t.Fatalf("catalog=%+v, want only grant-visible active tool", definitions)
	}
	if _, err := application.Describe(context.Background(), hidden.Name, executionAuthorization()); !errors.Is(err, ErrToolUnknown) {
		t.Fatalf("scope-excluded describe err=%v, want ErrToolUnknown", err)
	}
	if _, err := application.Describe(context.Background(), draft.Name, executionAuthorization()); !errors.Is(err, ErrToolUnknown) {
		t.Fatalf("draft describe err=%v, want ErrToolUnknown", err)
	}
}

func TestApplicationCatalogFailsClosedForGrantAndUATFailures(t *testing.T) {
	for name, grantError := range map[string]error{
		"authorization denied": errors.New("grant mismatch"),
		"UAT inactive":         ErrToolUATBlocked,
	} {
		t.Run(name, func(t *testing.T) {
			application := newExecutionApplication(t, []Definition{validDefinition()}, &executorAuthorizer{grantError: grantError})
			_, err := application.Catalog(context.Background(), executionAuthorization())
			if grantError == ErrToolUATBlocked && !errors.Is(err, ErrToolUATBlocked) {
				t.Fatalf("err=%v, want UAT blocked", err)
			}
			if grantError != ErrToolUATBlocked && !errors.Is(err, ErrAuthorizationDenied) {
				t.Fatalf("err=%v, want authorization denied", err)
			}
		})
	}

	application := newExecutionApplication(t, []Definition{validDefinition()}, &executorAuthorizer{})
	invalid := executionAuthorization()
	invalid.WorkspaceID = 0
	if _, err := application.Catalog(context.Background(), invalid); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("zero workspace err=%v, want authorization denied", err)
	}
}

func TestApplicationInvokeDistinguishesFailClosedOutcomes(t *testing.T) {
	active := validDefinition()
	draft := validDefinition()
	draft.Name = "project.create"
	draft.Lifecycle = LifecycleDraft
	draft.AuditEvent = "tool.project.create"
	blocked := validDefinition()
	blocked.Name = "project.archive"
	blocked.UAT = UATBlocked
	blocked.AuditEvent = "tool.project.archive"
	mutation := validDefinition()
	mutation.Name = "project.update"
	mutation.Risk = RiskMutation
	mutation.ChangeSet = ChangeSetRequired
	mutation.Authorization.GrantScope = "project:write"
	mutation.AuditEvent = "tool.project.update"
	definitions := []Definition{active, draft, blocked, mutation}

	tests := []struct {
		name       string
		toolName   string
		input      json.RawMessage
		auth       ExecutionAuthorization
		authorizer *executorAuthorizer
		want       error
	}{
		{name: "unknown", toolName: "project.missing", input: json.RawMessage(`{"workspace_id":11}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{}, want: ErrToolUnknown},
		{name: "draft", toolName: draft.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{}, want: ErrToolDraft},
		{name: "UAT blocked definition", toolName: blocked.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{}, want: ErrToolUATBlocked},
		{name: "change set foundation unavailable", toolName: mutation.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{tools: map[string]bool{mutation.Name: true}, scopes: map[string]bool{"project:write": true}}, want: ErrToolUnavailable},
		{name: "inactive grant window", toolName: active.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{grantError: ErrToolUATBlocked}, want: ErrToolUATBlocked},
		{name: "authorization denied", toolName: active.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{grantError: errors.New("grant mismatch")}, want: ErrAuthorizationDenied},
		{name: "scope denied", toolName: active.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{scopes: map[string]bool{"quality:read": true}}, want: ErrToolScopeDenied},
		{name: "missing identity", toolName: active.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: ExecutionAuthorization{}, authorizer: &executorAuthorizer{}, want: ErrAuthorizationDenied},
		{name: "unauthorized draft enumeration", toolName: draft.Name, input: json.RawMessage(`{"workspace_id":11}`), auth: ExecutionAuthorization{}, authorizer: &executorAuthorizer{}, want: ErrAuthorizationDenied},
		{name: "malformed input", toolName: active.Name, input: json.RawMessage(`{"workspace_id":`), auth: executionAuthorization(), authorizer: &executorAuthorizer{}, want: ErrToolInputInvalid},
		{name: "schema mismatch", toolName: active.Name, input: json.RawMessage(`{}`), auth: executionAuthorization(), authorizer: &executorAuthorizer{}, want: ErrToolInputInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application := newExecutionApplication(t, definitions, test.authorizer)
			_, err := application.Invoke(context.Background(), Invocation{
				Name: test.toolName, Input: test.input, Authorization: test.auth,
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("err=%v, want %v", err, test.want)
			}
		})
	}
}

func TestApplicationInvokeValidatesOutputAndReturnsAuditFacts(t *testing.T) {
	definition := validDefinition()
	catalog, err := NewCatalog([]Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	authorizer := &executorAuthorizer{}
	application, err := NewApplication(catalog, authorizer, map[string]ToolHandler{definition.Name: executionHandler()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"workspace_id":11}`), Authorization: executionAuthorization(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != definition.Name || result.Version != definition.Version || result.AuditEvent != definition.AuditEvent {
		t.Fatalf("result=%+v", result)
	}

	application, err = NewApplication(catalog, authorizer, map[string]ToolHandler{
		definition.Name: ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) {
			return json.RawMessage(`{"wrong":true}`), nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"workspace_id":11}`), Authorization: executionAuthorization(),
	}); !errors.Is(err, ErrToolOutputInvalid) {
		t.Fatalf("err=%v, want output validation failure", err)
	}
}

func TestNewApplicationRejectsUnsafeWiring(t *testing.T) {
	catalog, err := NewCatalog([]Definition{validDefinition()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewApplication(nil, &executorAuthorizer{}, nil); !errors.Is(err, ErrExecutorInvalid) {
		t.Fatalf("nil catalog err=%v", err)
	}
	if _, err := NewApplication(catalog, nil, nil); !errors.Is(err, ErrExecutorInvalid) {
		t.Fatalf("nil authorizer err=%v", err)
	}
	if _, err := NewApplication(catalog, &executorAuthorizer{}, nil); !errors.Is(err, ErrToolUnavailable) {
		t.Fatalf("missing handler err=%v", err)
	}
}
