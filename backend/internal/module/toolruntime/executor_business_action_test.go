package toolruntime

import (
	"context"
	"encoding/json"
	"testing"
)

type businessActionAuthorizerStub struct {
	preflight BusinessActionDecision
	exact     []BusinessActionDecision
	calls     int
}

type businessActionMutationHandler struct {
	output json.RawMessage
	calls  int
}

func (handler *businessActionMutationHandler) Invoke(context.Context, ToolCall) (json.RawMessage, error) {
	return nil, ErrChangeSetRequired
}

func (handler *businessActionMutationHandler) Apply(context.Context, MutationCall) (json.RawMessage, error) {
	handler.calls++
	return append(json.RawMessage(nil), handler.output...), nil
}

func (stub *businessActionAuthorizerStub) AuthorizeBusinessAction(
	_ context.Context,
	request BusinessActionRequest,
) (BusinessActionDecision, error) {
	if len(request.Input) == 0 {
		return stub.preflight, nil
	}
	index := stub.calls
	stub.calls++
	if index >= len(stub.exact) {
		return BusinessActionDenied, nil
	}
	return stub.exact[index], nil
}

func (stub *businessActionAuthorizerStub) PreflightBusinessActions(
	_ context.Context,
	toolNames []string,
	_ ExecutionAuthorization,
) (map[string]BusinessActionDecision, error) {
	result := make(map[string]BusinessActionDecision, len(toolNames))
	for _, name := range toolNames {
		result[name] = stub.preflight
	}
	return result, nil
}

func TestApplicationReportsObjectScopedToolConditionalAndAuthorizesExactInput(t *testing.T) {
	definition := validDefinition()
	actions := &businessActionAuthorizerStub{
		preflight: BusinessActionConditional,
		exact:     []BusinessActionDecision{BusinessActionAllowed},
	}
	application := newExecutionApplicationWithOptions(
		t, []Definition{definition}, &executorAuthorizer{}, WithBusinessActionAuthorizer(actions),
	)
	items, err := application.Preflight(context.Background(), []string{definition.Name}, executionAuthorization())
	if err != nil || len(items) != 1 || items[0].Status != ToolPreflightConditional {
		t.Fatalf("preflight=%+v err=%v", items, err)
	}
	if _, err := application.Describe(context.Background(), definition.Name, executionAuthorization()); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"workspace_id":11}`), Authorization: executionAuthorization(),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApplicationPreflightReportsBusinessActionDenialSeparatelyFromGrantDenial(t *testing.T) {
	definition := validDefinition()
	actions := &businessActionAuthorizerStub{preflight: BusinessActionDenied}
	application := newExecutionApplicationWithOptions(
		t, []Definition{definition}, &executorAuthorizer{}, WithBusinessActionAuthorizer(actions),
	)
	items, err := application.Preflight(context.Background(), []string{definition.Name}, executionAuthorization())
	if err != nil || len(items) != 1 || items[0].Status != ToolPreflightNotGranted ||
		items[0].DenialReason != ToolPreflightDeniedByAction {
		t.Fatalf("preflight=%+v err=%v", items, err)
	}
}

func TestApplicationRechecksBusinessActionBeforeConfirmedMutation(t *testing.T) {
	definition := validDefinition()
	definition.Name, definition.AuditEvent = "project.archive", "tool.project.archive"
	definition.Risk, definition.ChangeSet = RiskMutation, ChangeSetRequired
	handler := &businessActionMutationHandler{output: json.RawMessage(`{"workspace_id":11}`)}
	changeSets, err := NewChangeSetService(&changeSetMemoryRepository{}, DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	actions := &businessActionAuthorizerStub{
		preflight: BusinessActionAllowed,
		exact:     []BusinessActionDecision{BusinessActionAllowed, BusinessActionDenied},
	}
	application, err := NewApplication(
		mustCatalog(t, definition),
		&executorAuthorizer{tools: map[string]bool{definition.Name: true}, scopes: map[string]bool{"project:read": true}},
		map[string]ToolHandler{definition.Name: handler},
		WithChangeSetService(changeSets), WithBusinessActionAuthorizer(actions),
	)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"workspace_id":11}`),
		IdempotencyKey: "action-recheck-0001", Authorization: executionAuthorization(),
	})
	if err != nil || pending.ChangeSet == nil {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	confirmed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, executionAuthorization())
	if err != nil || confirmed.Status != ChangeSetRejected || confirmed.ErrorCode != "TOOL_AUTHORIZATION_DENIED" {
		t.Fatalf("confirmed=%+v err=%v", confirmed, err)
	}
	if handler.calls != 0 {
		t.Fatalf("mutation applied after Action revocation: %d", handler.calls)
	}
}

func newExecutionApplicationWithOptions(
	t *testing.T,
	definitions []Definition,
	authorizer InvocationAuthorizer,
	options ...ApplicationOption,
) *Application {
	t.Helper()
	handlers := make(map[string]ToolHandler, len(definitions))
	for _, definition := range definitions {
		handlers[definition.Name] = ToolHandlerFunc(func(_ context.Context, _ ToolCall) (json.RawMessage, error) {
			return json.RawMessage(`{"items":[]}`), nil
		})
	}
	application, err := NewApplication(mustCatalog(t, definitions...), authorizer, handlers, options...)
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func mustCatalog(t *testing.T, definitions ...Definition) *Catalog {
	t.Helper()
	catalog, err := NewCatalog(definitions)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
