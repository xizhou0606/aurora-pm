package taskgorm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type CoreMutationFactsFactory func(*gorm.DB) (task.CoreMutationFacts, error)
type CoreMutationEffectFactory func(*gorm.DB) (task.CoreMutationEffectSink, error)
type TaskDependentCleanerFactory func(*gorm.DB) (task.TaskDependentCleaner, error)

type CoreMutationRepository struct {
	db             *gorm.DB
	factsFactory   CoreMutationFactsFactory
	effectFactory  CoreMutationEffectFactory
	cleanerFactory TaskDependentCleanerFactory
}

func NewCoreMutationRepository(
	db *gorm.DB,
	factsFactory CoreMutationFactsFactory,
	effectFactory CoreMutationEffectFactory,
	cleanerFactory TaskDependentCleanerFactory,
) (*CoreMutationRepository, error) {
	if db == nil || factsFactory == nil || effectFactory == nil || cleanerFactory == nil {
		return nil, task.ErrCoreMutationInvalid
	}
	return &CoreMutationRepository{db: db, factsFactory: factsFactory, effectFactory: effectFactory, cleanerFactory: cleanerFactory}, nil
}

func (repository *CoreMutationRepository) ApplyCoreMutation(
	ctx context.Context, command task.CoreMutation,
) (task.CoreMutationResult, error) {
	if repository == nil || repository.db == nil {
		return task.CoreMutationResult{}, task.ErrCoreMutationInvalid
	}
	if replay, found, err := repository.replayCoreMutation(ctx, command); err != nil || found {
		return replay, err
	}
	var result task.CoreMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := TaskToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID,
			Operation: string(command.Operation), InputDigest: command.InputDigest, Status: taskMutationPending,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		facts, effects, err := repository.coreTransactionDependencies(tx)
		if err != nil {
			return err
		}
		switch command.Operation {
		case task.CoreCreate:
			result, err = applyCoreTaskCreate(ctx, tx, facts, effects, command)
		case task.CoreUpdate:
			result, err = applyCoreTaskUpdate(ctx, tx, facts, effects, command)
		case task.CoreClone:
			result, err = applyCoreTaskClone(ctx, tx, facts, effects, command)
		default:
			err = task.ErrCoreMutationInvalid
		}
		if err != nil {
			return err
		}
		return finalizeCoreMutation(tx, ledger.ID, result)
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replayCoreMutation(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return task.CoreMutationResult{}, normalizeCoreMutationError(err)
}

func (repository *CoreMutationRepository) coreTransactionDependencies(tx *gorm.DB) (task.CoreMutationFacts, task.CoreMutationEffectSink, error) {
	facts, err := repository.factsFactory(tx)
	if err != nil || facts == nil {
		if err == nil {
			err = task.ErrCoreMutationInvalid
		}
		return nil, nil, err
	}
	effects, err := repository.effectFactory(tx)
	if err != nil || effects == nil {
		if err == nil {
			err = task.ErrCoreMutationInvalid
		}
		return nil, nil, err
	}
	return facts, effects, nil
}

func (repository *CoreMutationRepository) replayCoreMutation(ctx context.Context, command task.CoreMutation) (task.CoreMutationResult, bool, error) {
	var ledger TaskToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.CoreMutationResult{}, false, nil
	}
	if err != nil {
		return task.CoreMutationResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.ApplyKey != command.ApplyKey || ledger.ChangeSetID != command.ChangeSetID ||
		ledger.Operation != string(command.Operation) || ledger.InputDigest != command.InputDigest || ledger.Status != taskMutationSucceeded || ledger.ResultJSON == "" {
		return task.CoreMutationResult{}, true, task.ErrCoreMutationConflict
	}
	var result task.CoreMutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return task.CoreMutationResult{}, true, task.ErrCoreMutationConflict
	}
	return result, true, nil
}

func finalizeCoreMutation(tx *gorm.DB, ledgerID uint, result task.CoreMutationResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	updated := tx.Model(&TaskToolMutationApply{}).Where("id = ? AND status = ? AND result_json = ''", ledgerID, taskMutationPending).
		Updates(map[string]any{"status": taskMutationSucceeded, "result_json": string(encoded)})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return task.ErrCoreMutationConflict
	}
	return nil
}

var _ task.CoreMutationStore = (*CoreMutationRepository)(nil)
