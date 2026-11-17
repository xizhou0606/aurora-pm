package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestBusinessFailureProjectionPreservesSafeRecovery(t *testing.T) {
	failure := LookupBusinessError("DOCUMENT_EDITING_IN_PROGRESS")
	if failure == nil || failure.Status != 409 || failure.Type == "authorization" {
		t.Fatal(failure)
	}
	wrapped := fmt.Errorf("private transport detail: %w", failure)
	if public := PublicBusinessError(wrapped); public == nil || strings.Contains(public.Message, "private") {
		t.Fatal(public)
	}
	if PublicBusinessError(errors.New("database secret")) != nil {
		t.Fatal("unknown error exposed")
	}
	encoded, err := json.Marshal(ChangeSet{Status: ChangeSetFailed, ErrorCode: failure.Code})
	if err != nil || !strings.Contains(string(encoded), `"error":{"code":"DOCUMENT_EDITING_IN_PROGRESS"`) {
		t.Fatalf("%s %v", encoded, err)
	}
	// Persisted public code remains authoritative even if a caller constructs
	// a descriptor containing unsafe dynamic text.
	altered := *failure
	altered.Message = "private secret"
	if PublicBusinessError(&altered).Message == altered.Message {
		t.Fatal("unsafe copy escaped")
	}
}

type failingBusinessMutation struct{ calls int }

func (*failingBusinessMutation) Invoke(context.Context, ToolCall) (json.RawMessage, error) {
	return nil, ErrChangeSetRequired
}
func (h *failingBusinessMutation) Apply(context.Context, MutationCall) (json.RawMessage, error) {
	h.calls++
	return nil, LookupBusinessError("DOCUMENT_EDITING_IN_PROGRESS")
}
func TestChangeSetPersistsBusinessFailureWithoutRepeatingMutation(t *testing.T) {
	definition := validDefinition()
	definition.Name = "project.archive"
	definition.AuditEvent = "tool.project.archive"
	definition.Risk = RiskMutation
	definition.ChangeSet = ChangeSetRequired
	handler := &failingBusinessMutation{}
	service, err := NewChangeSetService(&changeSetMemoryRepository{}, DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApplication(mustCatalog(t, definition), &executorAuthorizer{}, map[string]ToolHandler{definition.Name: handler}, WithChangeSetService(service))
	if err != nil {
		t.Fatal(err)
	}
	pending, err := app.Invoke(context.Background(), Invocation{Name: definition.Name, Input: json.RawMessage(`{"workspace_id":11}`), IdempotencyKey: "business-failure-001", Authorization: executionAuthorization()})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := app.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, executionAuthorization())
	if err != nil || failed.Status != ChangeSetFailed || failed.ErrorCode != "DOCUMENT_EDITING_IN_PROGRESS" {
		t.Fatalf("%+v %v", failed, err)
	}
	shown, err := app.ShowChangeSet(context.Background(), failed.ID, executionAuthorization())
	if err != nil || shown.ErrorCode != failed.ErrorCode {
		t.Fatal("stored failure lost")
	}
	_, _ = app.ConfirmChangeSet(context.Background(), failed.ID, executionAuthorization())
	if handler.calls != 1 {
		t.Fatal("failed request automatically retried")
	}
}

func TestReadInvocationPreservesRegisteredBusinessError(t *testing.T) {
	definition := validDefinition()
	for _, code := range []string{"DOCUMENT_NOT_FOUND", "DOCUMENT_ACCESS_CONFLICT", "DOCUMENT_SERVICE_UNAVAILABLE"} {
		handler := ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) {
			return nil, LookupBusinessError(code)
		})
		app, err := NewApplication(mustCatalog(t, definition), &executorAuthorizer{}, map[string]ToolHandler{definition.Name: handler})
		if err != nil {
			t.Fatal(err)
		}
		_, err = app.Invoke(context.Background(), Invocation{Name: definition.Name, Input: json.RawMessage(`{"workspace_id":11}`), Authorization: executionAuthorization()})
		if failure := PublicBusinessError(err); failure == nil || failure.Code != code {
			t.Fatalf("lost %s: %v", code, err)
		}
	}
}
