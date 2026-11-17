package pmtools

import (
	"errors"
	"fmt"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func TestTaskBatchPublicErrorsPreserveCausesAndSafeStatus(t *testing.T) {
	for _, test := range []struct {
		cause  error
		code   string
		status int
	}{
		{task.ErrBatchMutationInvalid, "TASK_BATCH_INPUT_INVALID", 400},
		{task.ErrBatchMutationNotFound, "TASK_BATCH_NOT_FOUND", 404},
		{task.ErrBatchMutationConflict, "TASK_BATCH_CONFLICT", 409},
		{task.ErrBatchMutationAmbiguous, "TASK_REFERENCE_AMBIGUOUS", 409},
	} {
		t.Run(test.code, func(t *testing.T) {
			err := taskBatchPublicError(fmt.Errorf("private diagnostic: %w", test.cause))
			failure := toolruntime.PublicBusinessError(err)
			if !errors.Is(err, test.cause) || failure == nil || failure.Code != test.code || failure.Status != test.status {
				t.Fatalf("err=%v failure=%+v", err, failure)
			}
		})
	}
	internal := errors.New("database unavailable")
	if err := taskBatchPublicError(internal); err != internal || toolruntime.PublicBusinessError(err) != nil {
		t.Fatalf("internal error was classified as a business conflict: %v", err)
	}
}
