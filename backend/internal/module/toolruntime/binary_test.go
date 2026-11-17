package toolruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

type binaryTestHandler struct{ calls int }

func (*binaryTestHandler) Invoke(context.Context, ToolCall) (json.RawMessage, error) {
	return nil, ErrChangeSetRequired
}
func (*binaryTestHandler) Apply(context.Context, MutationCall) (json.RawMessage, error) {
	panic("file bypassed")
}
func (h *binaryTestHandler) ApplyFile(context.Context, MutationCall, BinaryFile) (json.RawMessage, error) {
	h.calls++
	return json.RawMessage(`{"items":[]}`), nil
}
func TestBinaryConfirmationRequiresFileAndRetainsChangeSetIdempotency(t *testing.T) {
	d := validDefinition()
	d.Risk = RiskMutation
	d.ChangeSet = ChangeSetRequired
	h := &binaryTestHandler{}
	sets, _ := NewChangeSetService(&changeSetMemoryRepository{}, DefaultChangeSetPolicy())
	app, err := NewApplication(mustCatalog(t, d), &executorAuthorizer{}, map[string]ToolHandler{d.Name: h}, WithChangeSetService(sets))
	if err != nil {
		t.Fatal(err)
	}
	auth := executionAuthorization()
	ctx := context.Background()
	result, err := app.Invoke(ctx, Invocation{Name: d.Name, Input: json.RawMessage(`{"workspace_id":11}`), IdempotencyKey: "binary-upload-001", Authorization: auth})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.ConfirmChangeSet(ctx, result.ChangeSet.ID, auth)
	if e := PublicBusinessError(err); e == nil || e.Code != "TOOL_FILE_REQUIRED" {
		t.Fatal(err)
	}
	bad := auth
	bad.WorkspaceID++
	if _, err = app.ConfirmChangeSetWithFile(ctx, result.ChangeSet.ID, bad, BinaryFile{Reader: bytes.NewReader([]byte("x")), Size: 1}); err == nil {
		t.Fatal("cross workspace accepted")
	}
	confirmed, err := app.ConfirmChangeSetWithFile(ctx, result.ChangeSet.ID, auth, BinaryFile{Reader: bytes.NewReader([]byte("x")), Size: 1})
	if err != nil || confirmed.Status != ChangeSetSucceeded {
		t.Fatalf("%+v %v", confirmed, err)
	}
	if _, err = app.ConfirmChangeSet(ctx, result.ChangeSet.ID, auth); err != nil || h.calls != 1 {
		t.Fatal("replay performed upload again")
	}
}
