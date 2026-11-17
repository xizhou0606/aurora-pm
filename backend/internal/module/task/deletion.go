package task

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrTaskDeletionInvalid  = errors.New("task deletion is invalid")
	ErrTaskDeletionNotFound = errors.New("task deletion target not found")
	ErrTaskDeletionConflict = errors.New("task deletion conflicts with current state")
)

type TaskDeletionCommand struct {
	WorkspaceID uint
	PrincipalID uint
	ApplyKey    string
	ChangeSetID string
	InputDigest string
	RootIDs     []uint
	Cascade     bool
}

type TaskDeletionWork struct {
	ObjectKeys []string `json:"-"`
}

type TaskDeletionResult struct {
	Deleted      bool              `json:"deleted"`
	DeletedIDs   []uint            `json:"deleted_ids"`
	DeletedCount int               `json:"deleted_count"`
	Pending      *TaskDeletionWork `json:"-"`
}

type TaskDeletionStore interface {
	PrepareTaskDeletion(context.Context, TaskDeletionCommand) (TaskDeletionResult, error)
	MarkTaskDeletionObjectRemoved(context.Context, TaskDeletionCommand, string) error
	CompleteTaskDeletion(context.Context, TaskDeletionCommand) (TaskDeletionResult, error)
}

// TaskDeletionExecutor is the reusable owning-domain delete contract consumed
// by both task.delete and atomic task.batch_delete.
type TaskDeletionExecutor interface {
	DeleteTasks(context.Context, TaskDeletionCommand) (TaskDeletionResult, error)
}

type TaskDeletionService struct {
	store   TaskDeletionStore
	remover AttachmentObjectRemover
}

func NewTaskDeletionService(store TaskDeletionStore, remover AttachmentObjectRemover) (*TaskDeletionService, error) {
	if store == nil || remover == nil {
		return nil, ErrTaskDeletionInvalid
	}
	return &TaskDeletionService{store: store, remover: remover}, nil
}

func (service *TaskDeletionService) DeleteTasks(ctx context.Context, command TaskDeletionCommand) (TaskDeletionResult, error) {
	if service == nil || service.store == nil || service.remover == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		strings.TrimSpace(command.ApplyKey) == "" || strings.TrimSpace(command.ChangeSetID) == "" || len(command.InputDigest) != 64 || len(command.RootIDs) == 0 {
		return TaskDeletionResult{}, ErrTaskDeletionInvalid
	}
	seen := make(map[uint]struct{}, len(command.RootIDs))
	for _, id := range command.RootIDs {
		if id == 0 {
			return TaskDeletionResult{}, ErrTaskDeletionInvalid
		}
		if _, duplicate := seen[id]; duplicate {
			return TaskDeletionResult{}, ErrTaskDeletionInvalid
		}
		seen[id] = struct{}{}
	}
	result, err := service.store.PrepareTaskDeletion(ctx, command)
	if err != nil || result.Pending == nil {
		return result, err
	}
	for _, key := range result.Pending.ObjectKeys {
		if err := service.remover.RemoveObject(ctx, key); err != nil {
			return TaskDeletionResult{}, err
		}
		if err := service.store.MarkTaskDeletionObjectRemoved(ctx, command, key); err != nil {
			return TaskDeletionResult{}, err
		}
	}
	return service.store.CompleteTaskDeletion(ctx, command)
}

func IsTaskDeletionBusinessError(err error) bool {
	return errors.Is(err, ErrTaskDeletionInvalid) || errors.Is(err, ErrTaskDeletionNotFound) || errors.Is(err, ErrTaskDeletionConflict)
}

var _ TaskDeletionExecutor = (*TaskDeletionService)(nil)
