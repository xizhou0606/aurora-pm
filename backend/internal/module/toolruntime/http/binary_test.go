package toolruntimehttp

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type binaryExecutor struct {
	fakeExecutor
	data []byte
}

func (e *binaryExecutor) ConfirmChangeSetWithFile(_ context.Context, id string, auth toolruntime.ExecutionAuthorization, file toolruntime.BinaryFile) (toolruntime.ChangeSet, error) {
	e.changeSetID, e.changeSetAuth = id, auth
	e.data, _ = io.ReadAll(file.Reader)
	return toolruntime.ChangeSet{ID: id, Status: toolruntime.ChangeSetSucceeded}, nil
}
func TestMultipartConfirmationUsesAuthenticatedIdentity(t *testing.T) {
	e := &binaryExecutor{}
	engine := toolRuntimeEngine(e, allowedResolver())
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "proof.png")
	_, _ = part.Write([]byte("proof"))
	_ = writer.Close()
	request := httptest.NewRequest("POST", "/api/v1/tool-runtime/changesets/cs-1/confirm", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != 200 || string(e.data) != "proof" || e.changeSetAuth != httpAuthorization() {
		t.Fatalf("response=%s auth=%+v", response.Body, e.changeSetAuth)
	}
}
