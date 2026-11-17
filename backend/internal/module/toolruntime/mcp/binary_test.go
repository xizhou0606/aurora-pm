package mcp

import (
	"encoding/json"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"io"
	"testing"
)

func TestBinaryMCPConfirmationSeparatesFileFromControlInput(t *testing.T) {
	raw := json.RawMessage(`{"id":"changeset-1","file_base64":"aW1hZ2U="}`)
	clean, file, err := binaryControlInput(toolruntime.ChangeSetConfirmControlName, raw)
	if err != nil || file == nil {
		t.Fatal(err)
	}
	if _, err := decodeChangeSetControlInput(clean); err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(file.Reader)
	if string(data) != "image" || file.Size != 5 {
		t.Fatal("invalid file")
	}
	if _, _, err := binaryControlInput(toolruntime.ChangeSetShowControlName, raw); err == nil {
		t.Fatal("show accepted upload")
	}
}
