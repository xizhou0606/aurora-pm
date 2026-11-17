package pmtools

import (
	"errors"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

// Preserve the domain cause for in-process callers while exposing only stable,
// registered descriptions through CLI/MCP and persisted ChangeSet failures.
func taskBatchPublicError(err error) error {
	code := ""
	switch {
	case errors.Is(err, task.ErrBatchMutationInvalid):
		code = "TASK_BATCH_INPUT_INVALID"
	case errors.Is(err, task.ErrBatchMutationNotFound):
		code = "TASK_BATCH_NOT_FOUND"
	case errors.Is(err, task.ErrBatchMutationAmbiguous):
		code = "TASK_REFERENCE_AMBIGUOUS"
	case errors.Is(err, task.ErrBatchMutationConflict):
		code = "TASK_BATCH_CONFLICT"
	}
	if code == "" {
		return err
	}
	return fmt.Errorf("%w: %w", toolruntime.LookupBusinessError(code), err)
}
