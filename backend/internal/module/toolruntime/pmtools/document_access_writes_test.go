package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func TestAccessWritesRequireChangeSetAndRejectForgedIdentity(t *testing.T) {
	s := &DocumentAccessTools{Users: documentToolUsers{}}
	for _, name := range DocumentAccessWriteHandlerNames() {
		h := documentAccessWriteHandler{service: s, name: name}
		if _, err := h.Invoke(context.Background(), toolruntime.ToolCall{}); !errors.Is(err, toolruntime.ErrChangeSetRequired) {
			t.Fatalf("%s bypassed confirmation", name)
		}
		call := toolruntime.MutationCall{ChangeSetID: "cs", ApplyKey: "cs", Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 7, WorkspaceID: 1}, Input: json.RawMessage(`{"resource_id":"r","user_id":1}`)}
		if _, err := h.Apply(context.Background(), call); !errors.Is(err, toolruntime.ErrToolInputInvalid) {
			t.Fatalf("%s accepted forged actor: %v", name, err)
		}
		call.Input = json.RawMessage(`{"resource_id":"r"}`)
		call.Authorization.PrincipalID = 0
		if _, err := h.Apply(context.Background(), call); !errors.Is(err, toolruntime.ErrAuthorizationDenied) {
			t.Fatalf("%s accepted missing identity: %v", name, err)
		}
	}
}
