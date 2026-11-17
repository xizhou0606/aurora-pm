package quality

import (
	"context"
	"errors"
)

var (
	ErrTaskReferenceCleanupInvalid  = errors.New("quality task reference cleanup is invalid")
	ErrTaskReferenceCleanupConflict = errors.New("quality task reference cleanup conflicts with current state")
)

// TaskReferenceCleanupFacts are immutable identifiers frozen by the task
// owner before its testing records are removed.
type TaskReferenceCleanupFacts struct {
	WorkspaceID      uint
	TaskIDs          []uint
	TestExecutionIDs []uint
}

type TaskReferenceCleanupStore interface {
	ClearTaskReferences(context.Context, TaskReferenceCleanupFacts) error
}

type TaskReferenceCleanupService struct{ store TaskReferenceCleanupStore }

func NewTaskReferenceCleanupService(store TaskReferenceCleanupStore) (*TaskReferenceCleanupService, error) {
	if store == nil {
		return nil, ErrTaskReferenceCleanupInvalid
	}
	return &TaskReferenceCleanupService{store: store}, nil
}

func (service *TaskReferenceCleanupService) ClearTaskReferences(ctx context.Context, facts TaskReferenceCleanupFacts) error {
	if service == nil || service.store == nil || facts.WorkspaceID == 0 || len(facts.TaskIDs) == 0 ||
		!positiveUniqueQualityIDs(facts.TaskIDs) || !positiveUniqueQualityIDs(facts.TestExecutionIDs) {
		return ErrTaskReferenceCleanupInvalid
	}
	return service.store.ClearTaskReferences(ctx, facts)
}

func positiveUniqueQualityIDs(ids []uint) bool {
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
