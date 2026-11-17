package toolruntime

import (
	"context"
	"encoding/json"
	"io"
)

const MaxToolFileBytes int64 = 200 * 1024 * 1024

// BinaryFile is transient confirmation data. It never enters ChangeSet input,
// result, audit or an invocation JSON document.
type BinaryFile struct {
	Reader io.ReadSeeker
	Size   int64
}
type BinaryMutationHandler interface {
	IdempotentMutationHandler
	ApplyFile(context.Context, MutationCall, BinaryFile) (json.RawMessage, error)
}
type BinaryChangeSetExecutor interface {
	ConfirmChangeSetWithFile(context.Context, string, ExecutionAuthorization, BinaryFile) (ChangeSet, error)
}

func (a *Application) ConfirmChangeSetWithFile(ctx context.Context, id string, auth ExecutionAuthorization, file BinaryFile) (ChangeSet, error) {
	if file.Reader == nil || file.Size <= 0 || file.Size > MaxToolFileBytes {
		return ChangeSet{}, LookupBusinessError("TOOL_FILE_INVALID")
	}
	return a.confirmChangeSet(ctx, id, auth, &file)
}
