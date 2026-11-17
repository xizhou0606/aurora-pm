package taskgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResolveTaskBatchSelection is a read-only helper used by owning-domain tests
// and adapters that do not need durable deletion replay.
func (repository *BatchMutationRepository) ResolveTaskBatchSelection(
	ctx context.Context,
	workspaceID uint,
	selector task.BatchTaskSelector,
) ([]uint, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return nil, task.ErrBatchMutationInvalid
	}
	return resolveTaskBatchSelection(repository.db.WithContext(ctx), workspaceID, selector, false)
}

func (repository *BatchMutationRepository) ResolveTaskBatchDeletion(
	ctx context.Context,
	command task.BatchMutation,
) ([]uint, error) {
	if repository == nil || repository.db == nil || command.Operation != task.BatchDelete || command.WorkspaceID == 0 ||
		command.ApplyKey == "" || command.ChangeSetID == "" || len(command.InputDigest) != 64 {
		return nil, task.ErrBatchMutationInvalid
	}
	if ids, found, err := repository.replayTaskBatchDeletion(ctx, command); err != nil || found {
		return ids, err
	}
	var ids []uint
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		ids, err = resolveTaskBatchSelection(tx, command.WorkspaceID, command.Selector, true)
		if err != nil {
			return err
		}
		selectorJSON, err := json.Marshal(command.Selector)
		if err != nil {
			return err
		}
		idsJSON, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		return tx.Create(&TaskBatchDeletionApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID,
			InputDigest: command.InputDigest, SelectorJSON: string(selectorJSON), RootIDsJSON: string(idsJSON),
		}).Error
	})
	if err == nil {
		return ids, nil
	}
	if replay, found, replayErr := repository.replayTaskBatchDeletion(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return nil, normalizeTaskBatchMutationError(err)
}

func (repository *BatchMutationRepository) replayTaskBatchDeletion(
	ctx context.Context,
	command task.BatchMutation,
) ([]uint, bool, error) {
	var row TaskBatchDeletionApply
	err := repository.db.WithContext(ctx).Where(
		"apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID,
	).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if row.WorkspaceID != command.WorkspaceID || row.ApplyKey != command.ApplyKey || row.ChangeSetID != command.ChangeSetID ||
		row.InputDigest != command.InputDigest {
		return nil, true, task.ErrBatchMutationConflict
	}
	var ids []uint
	if json.Unmarshal([]byte(row.RootIDsJSON), &ids) != nil || len(ids) == 0 || len(ids) > 500 {
		return nil, true, task.ErrBatchMutationConflict
	}
	return ids, true, nil
}

func resolveTaskBatchSelection(
	db *gorm.DB,
	workspaceID uint,
	selector task.BatchTaskSelector,
	lock bool,
) ([]uint, error) {
	seen, result := map[uint]bool{}, make([]uint, 0, len(selector.IDs)+len(selector.TaskRefs))
	for _, id := range selector.IDs {
		if id == 0 || seen[id] {
			return nil, task.ErrBatchMutationInvalid
		}
		seen[id] = true
		result = append(result, id)
	}
	for _, rawRef := range selector.TaskRefs {
		ref := strings.TrimSpace(rawRef)
		if ref == "" {
			return nil, task.ErrBatchMutationInvalid
		}
		rows, err := exactTaskReferenceRows(db, workspaceID, selector, "LOWER(TRIM(task_key)) = ?", strings.ToLower(ref), lock)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			rows, err = exactTaskReferenceRows(db, workspaceID, selector, "LOWER(TRIM(title)) = ?", strings.ToLower(ref), lock)
			if err != nil {
				return nil, err
			}
		}
		if len(rows) == 0 {
			return nil, task.ErrBatchMutationNotFound
		}
		if len(rows) != 1 {
			return nil, task.ErrBatchMutationAmbiguous
		}
		if !seen[rows[0].ID] {
			seen[rows[0].ID] = true
			result = append(result, rows[0].ID)
		}
	}
	if len(result) == 0 || len(result) > 500 {
		return nil, task.ErrBatchMutationInvalid
	}
	var rows []WorkTask
	query := scopedBatchTaskQuery(db, workspaceID, selector).Where("id IN ?", result)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(result) {
		return nil, task.ErrBatchMutationNotFound
	}
	return result, nil
}

func exactTaskReferenceRows(
	db *gorm.DB,
	workspaceID uint,
	selector task.BatchTaskSelector,
	predicate string,
	value string,
	lock bool,
) ([]WorkTask, error) {
	var rows []WorkTask
	query := scopedBatchTaskQuery(db, workspaceID, selector).Where(predicate, value).Limit(2)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func scopedBatchTaskQuery(db *gorm.DB, workspaceID uint, selector task.BatchTaskSelector) *gorm.DB {
	query := db.Model(&WorkTask{}).Where(
		"workspace_id = ? AND mutation_state = ? AND LOWER(TRIM(kind)) IN ?", workspaceID, workTaskStateActive, PMWorkTaskKinds(),
	)
	if selector.ProjectSetID > 0 {
		query = query.Where("project_set_id = ?", selector.ProjectSetID)
	}
	if selector.PlanUnitID > 0 {
		query = query.Where("project_id = ?", selector.PlanUnitID)
	}
	if selector.RequirementID > 0 {
		query = query.Where("requirement_id = ?", selector.RequirementID)
	}
	return query
}

var _ task.BatchMutationStore = (*BatchMutationRepository)(nil)
