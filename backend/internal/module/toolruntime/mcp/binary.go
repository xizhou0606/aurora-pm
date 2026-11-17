package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

// Larger files use the authenticated multipart confirmation endpoint. Inline
// MCP files are bounded to 512 KiB by the existing one-MiB protocol envelope.
func binaryControlInput(name string, raw json.RawMessage) (json.RawMessage, *toolruntime.BinaryFile, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, nil, toolruntime.ErrChangeSetInvalid
	}
	encoded, found := fields["file_base64"]
	if !found {
		return raw, nil, nil
	}
	if name != toolruntime.ChangeSetConfirmControlName {
		return nil, nil, toolruntime.ErrChangeSetInvalid
	}
	var value string
	if json.Unmarshal(encoded, &value) != nil || len(value) > 699052 {
		return nil, nil, toolruntime.LookupBusinessError("TOOL_FILE_INVALID")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || len(data) == 0 || len(data) > 512*1024 {
		return nil, nil, toolruntime.LookupBusinessError("TOOL_FILE_INVALID")
	}
	delete(fields, "file_base64")
	input, _ := json.Marshal(fields)
	return input, &toolruntime.BinaryFile{Reader: bytes.NewReader(data), Size: int64(len(data))}, nil
}
