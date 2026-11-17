package pmtools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const QualityMaterialUpload = "quality.material.upload"

type QualityUploadHandler struct {
	Service *quality.MaterialUploadService
}

func (h QualityUploadHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}
func (h QualityUploadHandler) Apply(context.Context, toolruntime.MutationCall) (json.RawMessage, error) {
	return nil, toolruntime.LookupBusinessError("TOOL_FILE_REQUIRED")
}
func (h QualityUploadHandler) ApplyFile(ctx context.Context, call toolruntime.MutationCall, file toolruntime.BinaryFile) (json.RawMessage, error) {
	if call.ApplyKey == "" || call.ChangeSetID == "" || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 {
		return nil, toolruntime.ErrAuthorizationDenied
	}
	var input quality.MaterialUploadInput
	if decodeInput(call.Input, &input) != nil || input.Size != file.Size {
		return nil, toolruntime.LookupBusinessError("TOOL_FILE_INVALID")
	}
	if h.Service == nil {
		return nil, toolruntime.LookupBusinessError("QUALITY_UPLOAD_UNAVAILABLE")
	}
	result, err := h.Service.Upload(ctx, call.Authorization.WorkspaceID, call.Authorization.PrincipalID, call.ApplyKey, input, file.Reader)
	if err != nil {
		switch {
		case errors.Is(err, quality.ErrBugMutationNotFound):
			return nil, toolruntime.LookupBusinessError("QUALITY_MATERIAL_SCOPE_DENIED")
		case errors.Is(err, quality.ErrBugMutationInvalid), errors.Is(err, quality.ErrMaterialFileMismatch):
			return nil, toolruntime.LookupBusinessError("TOOL_FILE_INVALID")
		default:
			return nil, toolruntime.LookupBusinessError("QUALITY_UPLOAD_UNAVAILABLE")
		}
	}
	return json.Marshal(result)
}
