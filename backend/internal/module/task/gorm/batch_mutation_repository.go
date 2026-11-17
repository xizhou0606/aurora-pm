package taskgorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type BatchMutationDependencies struct {
	Scopes        task.BatchScopeResolver
	Users         task.BatchUserVerifier
	Effects       task.BatchEffectWriter
	Notifications task.BatchNotificationWriter
}

type BatchMutationDependencyFactory func(*gorm.DB) (BatchMutationDependencies, error)

type BatchMutationRepository struct {
	db           *gorm.DB
	dependencies BatchMutationDependencyFactory
}

func NewBatchMutationRepository(
	db *gorm.DB,
	dependencies BatchMutationDependencyFactory,
) (*BatchMutationRepository, error) {
	if db == nil || dependencies == nil {
		return nil, task.ErrBatchMutationInvalid
	}
	return &BatchMutationRepository{db: db, dependencies: dependencies}, nil
}

func (repository *BatchMutationRepository) ApplyTaskBatchMutation(
	ctx context.Context,
	command task.BatchMutation,
) (task.BatchMutationResult, error) {
	if replay, found, err := repository.replay(ctx, command); err != nil || found {
		return replay, err
	}
	var result task.BatchMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := TaskToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID,
			Operation: string(command.Operation), InputDigest: command.InputDigest, Status: taskMutationPending,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		dependencies, err := repository.dependencies(tx)
		if err != nil || dependencies.Scopes == nil || dependencies.Users == nil || dependencies.Effects == nil || dependencies.Notifications == nil {
			return task.ErrBatchMutationUnavailable
		}
		switch command.Operation {
		case task.BatchCreate, task.ChildrenBatchCreate:
			result, err = applyTaskBatchCreate(ctx, tx, dependencies, command)
		case task.BatchUpdate:
			result, err = applyTaskBatchUpdate(ctx, tx, dependencies, command)
		default:
			err = task.ErrBatchMutationInvalid
		}
		if err != nil {
			return err
		}
		return finalizeTaskBatchMutation(tx, ledger.ID, result)
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replay(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return task.BatchMutationResult{}, normalizeTaskBatchMutationError(err)
}

func (repository *BatchMutationRepository) replay(
	ctx context.Context,
	command task.BatchMutation,
) (task.BatchMutationResult, bool, error) {
	var ledger TaskToolMutationApply
	err := repository.db.WithContext(ctx).Where(
		"apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID,
	).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.BatchMutationResult{}, false, nil
	}
	if err != nil {
		return task.BatchMutationResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.ApplyKey != command.ApplyKey || ledger.ChangeSetID != command.ChangeSetID ||
		ledger.Operation != string(command.Operation) || ledger.InputDigest != command.InputDigest || ledger.Status != taskMutationSucceeded || ledger.ResultJSON == "" {
		return task.BatchMutationResult{}, true, task.ErrBatchMutationConflict
	}
	var result task.BatchMutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return task.BatchMutationResult{}, true, task.ErrBatchMutationConflict
	}
	return result, true, nil
}

func finalizeTaskBatchMutation(tx *gorm.DB, ledgerID uint, result task.BatchMutationResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	updated := tx.Model(&TaskToolMutationApply{}).Where(
		"id = ? AND status = ? AND result_json = ''", ledgerID, taskMutationPending,
	).Updates(map[string]any{"status": taskMutationSucceeded, "result_json": string(encoded)})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return task.ErrBatchMutationConflict
	}
	return nil
}

func normalizeTaskBatchMutationError(err error) error {
	if err == nil || task.IsBatchMutationBusinessError(err) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrBatchMutationNotFound
	}
	return fmt.Errorf("task batch mutation store: %w", err)
}

var _ task.BatchMutationStore = (*BatchMutationRepository)(nil)
