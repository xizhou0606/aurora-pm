package taskgorm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const taskDeleteOperation = "task.delete"

func (repository *CoreMutationRepository) PrepareTaskDeletion(ctx context.Context, command task.TaskDeletionCommand) (task.TaskDeletionResult, error) {
	if replay, found, err := repository.replayTaskDeletion(ctx, command); err != nil || found {
		return replay, err
	}
	var result task.TaskDeletionResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := TaskToolMutationApply{WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID, Operation: taskDeleteOperation, InputDigest: command.InputDigest, Status: taskMutationPending}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		facts, err := repository.factsFactory(tx)
		if err != nil || facts == nil {
			if err == nil {
				err = task.ErrTaskDeletionInvalid
			}
			return err
		}
		rows, err := lockDeletionRoots(tx, command.WorkspaceID, command.RootIDs)
		if err != nil {
			return err
		}
		descendants, err := collectCoreDescendants(tx, command.WorkspaceID, command.RootIDs)
		if err != nil {
			return err
		}
		selected := make(map[uint]struct{}, len(command.RootIDs))
		for _, id := range command.RootIDs {
			selected[id] = struct{}{}
		}
		if !command.Cascade {
			for _, row := range descendants {
				if _, included := selected[row.ID]; !included {
					return task.ErrTaskDeletionConflict
				}
			}
		}
		byID := make(map[uint]WorkTask, len(rows)+len(descendants))
		for _, row := range rows {
			byID[row.ID] = row
		}
		for _, row := range descendants {
			byID[row.ID] = row
		}
		ids := make([]uint, 0, len(byID))
		for id := range byID {
			ids = append(ids, id)
		}
		ids = sortedCoreIDs(ids)
		for _, id := range ids {
			row := byID[id]
			scope, resolveErr := facts.ResolveCoreMutationScope(ctx, command.WorkspaceID, row.RequirementID)
			if resolveErr != nil {
				return resolveErr
			}
			if !coreTaskMatchesScope(row, scope) {
				return task.ErrTaskDeletionConflict
			}
		}
		encodedIDs, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		saga := TaskDeletionSaga{WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, TaskIDsJSON: string(encodedIDs), Status: taskMutationPending}
		if err := tx.Create(&saga).Error; err != nil {
			return err
		}
		var attachments []WorkTaskAttachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND work_task_id IN ?", command.WorkspaceID, ids).Order("id").Find(&attachments).Error; err != nil {
			return err
		}
		objects, keys, seenKeys := make([]TaskDeletionObject, 0, len(attachments)), []string{}, map[string]struct{}{}
		for _, attachment := range attachments {
			if attachment.MutationState != attachmentStateActive || attachment.StorageBackend != "s3" || attachment.ObjectKey == "" {
				return task.ErrTaskDeletionConflict
			}
			objects = append(objects, TaskDeletionObject{SagaID: saga.ID, AttachmentID: attachment.ID, ObjectKey: attachment.ObjectKey, Status: deletionObjectPending})
			if _, exists := seenKeys[attachment.ObjectKey]; !exists {
				seenKeys[attachment.ObjectKey] = struct{}{}
				keys = append(keys, attachment.ObjectKey)
			}
		}
		if len(objects) > 0 {
			if err := tx.Create(&objects).Error; err != nil {
				return err
			}
		}
		updated := tx.Model(&WorkTask{}).Where("workspace_id = ? AND id IN ? AND mutation_state = ?", command.WorkspaceID, ids, workTaskStateActive).Update("mutation_state", workTaskStateDeleting)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != int64(len(ids)) {
			return task.ErrTaskDeletionConflict
		}
		result = task.TaskDeletionResult{Pending: &task.TaskDeletionWork{ObjectKeys: keys}}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replayTaskDeletion(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return task.TaskDeletionResult{}, normalizeDeletionError(err)
}

func (repository *CoreMutationRepository) MarkTaskDeletionObjectRemoved(ctx context.Context, command task.TaskDeletionCommand, objectKey string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger, err := lockDeletionLedger(tx, command)
		if err != nil {
			return err
		}
		if ledger.Status == taskMutationSucceeded {
			return nil
		}
		saga, err := lockDeletionSaga(tx, command.ApplyKey)
		if err != nil {
			return err
		}
		updated := tx.Model(&TaskDeletionObject{}).Where("saga_id = ? AND object_key = ? AND status = ?", saga.ID, objectKey, deletionObjectPending).Update("status", deletionObjectRemoved)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			var count int64
			if err := tx.Model(&TaskDeletionObject{}).Where("saga_id = ? AND object_key = ? AND status = ?", saga.ID, objectKey, deletionObjectRemoved).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return task.ErrTaskDeletionConflict
			}
		}
		return nil
	})
}

func (repository *CoreMutationRepository) CompleteTaskDeletion(ctx context.Context, command task.TaskDeletionCommand) (task.TaskDeletionResult, error) {
	var result task.TaskDeletionResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger, err := lockDeletionLedger(tx, command)
		if err != nil {
			return err
		}
		if ledger.Status == taskMutationSucceeded {
			return json.Unmarshal([]byte(ledger.ResultJSON), &result)
		}
		saga, err := lockDeletionSaga(tx, command.ApplyKey)
		if err != nil {
			return err
		}
		var pending int64
		if err := tx.Model(&TaskDeletionObject{}).Where("saga_id = ? AND status <> ?", saga.ID, deletionObjectRemoved).Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return task.ErrTaskDeletionConflict
		}
		var ids []uint
		if json.Unmarshal([]byte(saga.TaskIDsJSON), &ids) != nil || len(ids) == 0 {
			return task.ErrTaskDeletionConflict
		}
		var rows []WorkTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id IN ? AND mutation_state = ?", command.WorkspaceID, ids, workTaskStateDeleting).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return task.ErrTaskDeletionConflict
		}
		executionIDs, err := frozenTaskTestExecutionIDs(tx, command.WorkspaceID, ids)
		if err != nil {
			return err
		}
		cleaner, err := repository.cleanerFactory(tx)
		if err != nil || cleaner == nil {
			if err == nil {
				err = task.ErrTaskDeletionInvalid
			}
			return err
		}
		if err := cleaner.ClearTaskReferences(ctx, task.TaskDependentReferenceFacts{
			WorkspaceID: command.WorkspaceID, TaskIDs: ids, TestExecutionIDs: executionIDs,
		}); err != nil {
			return err
		}
		if err := deleteOwnedTaskDependents(tx, command.WorkspaceID, ids); err != nil {
			return err
		}
		effects, err := repository.effectFactory(tx)
		if err != nil || effects == nil {
			if err == nil {
				err = task.ErrTaskDeletionInvalid
			}
			return err
		}
		sort.Slice(rows, func(i, j int) bool { return deletionDepth(rows, rows[i].ID) > deletionDepth(rows, rows[j].ID) })
		for _, row := range rows {
			if err := effects.RecordCoreTaskMutation(ctx, task.CoreMutationEffect{
				WorkspaceID: row.WorkspaceID, PrincipalID: command.PrincipalID, TeamID: row.TeamID,
				ProjectSetID: row.ProjectSetID, PlanUnitID: row.ProjectID, TaskID: row.ID, TaskKey: row.TaskKey,
				Action: "deleted", Target: row.Title, Payload: snapshotPayload(row),
			}); err != nil {
				return err
			}
			deleted := tx.Where("workspace_id = ? AND id = ? AND mutation_state = ?", command.WorkspaceID, row.ID, workTaskStateDeleting).Delete(&WorkTask{})
			if deleted.Error != nil {
				return deleted.Error
			}
			if deleted.RowsAffected != 1 {
				return task.ErrTaskDeletionConflict
			}
		}
		result = task.TaskDeletionResult{Deleted: true, DeletedIDs: ids, DeletedCount: len(ids)}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if updated := tx.Model(&TaskDeletionSaga{}).Where("id = ? AND status = ?", saga.ID, taskMutationPending).Updates(map[string]any{"status": taskMutationSucceeded, "completed_at": &now}); updated.Error != nil {
			return updated.Error
		} else if updated.RowsAffected != 1 {
			return task.ErrTaskDeletionConflict
		}
		if updated := tx.Model(&TaskToolMutationApply{}).Where("id = ? AND status = ? AND result_json = ''", ledger.ID, taskMutationPending).Updates(map[string]any{"status": taskMutationSucceeded, "result_json": string(encoded)}); updated.Error != nil {
			return updated.Error
		} else if updated.RowsAffected != 1 {
			return task.ErrTaskDeletionConflict
		}
		return nil
	})
	return result, normalizeDeletionError(err)
}

func (repository *CoreMutationRepository) replayTaskDeletion(ctx context.Context, command task.TaskDeletionCommand) (task.TaskDeletionResult, bool, error) {
	var ledger TaskToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.TaskDeletionResult{}, false, nil
	}
	if err != nil {
		return task.TaskDeletionResult{}, false, err
	}
	if !deletionLedgerMatches(ledger, command) {
		return task.TaskDeletionResult{}, true, task.ErrTaskDeletionConflict
	}
	if ledger.Status == taskMutationSucceeded {
		var result task.TaskDeletionResult
		if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
			return task.TaskDeletionResult{}, true, task.ErrTaskDeletionConflict
		}
		return result, true, nil
	}
	if ledger.Status != taskMutationPending {
		return task.TaskDeletionResult{}, true, task.ErrTaskDeletionConflict
	}
	var saga TaskDeletionSaga
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND apply_key = ? AND status = ?", command.WorkspaceID, command.ApplyKey, taskMutationPending).First(&saga).Error; err != nil {
		return task.TaskDeletionResult{}, true, normalizeDeletionError(err)
	}
	var objects []TaskDeletionObject
	if err := repository.db.WithContext(ctx).Where("saga_id = ? AND status = ?", saga.ID, deletionObjectPending).Order("id").Find(&objects).Error; err != nil {
		return task.TaskDeletionResult{}, true, err
	}
	keys, seen := []string{}, map[string]struct{}{}
	for _, row := range objects {
		if _, exists := seen[row.ObjectKey]; !exists {
			seen[row.ObjectKey] = struct{}{}
			keys = append(keys, row.ObjectKey)
		}
	}
	return task.TaskDeletionResult{Pending: &task.TaskDeletionWork{ObjectKeys: keys}}, true, nil
}

func deletionLedgerMatches(row TaskToolMutationApply, command task.TaskDeletionCommand) bool {
	return row.WorkspaceID == command.WorkspaceID && row.ApplyKey == command.ApplyKey && row.ChangeSetID == command.ChangeSetID && row.Operation == taskDeleteOperation && row.InputDigest == command.InputDigest
}

func lockDeletionLedger(tx *gorm.DB, command task.TaskDeletionCommand) (TaskToolMutationApply, error) {
	var row TaskToolMutationApply
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID).First(&row).Error
	if err != nil {
		return row, normalizeDeletionError(err)
	}
	if !deletionLedgerMatches(row, command) {
		return row, task.ErrTaskDeletionConflict
	}
	return row, nil
}
func lockDeletionSaga(tx *gorm.DB, applyKey string) (TaskDeletionSaga, error) {
	var row TaskDeletionSaga
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ?", applyKey).First(&row).Error
	return row, normalizeDeletionError(err)
}

func lockDeletionRoots(tx *gorm.DB, workspaceID uint, ids []uint) ([]WorkTask, error) {
	var rows []WorkTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id IN ? AND mutation_state = ?", workspaceID, ids, workTaskStateActive).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, task.ErrTaskDeletionNotFound
	}
	return rows, nil
}

func deleteOwnedTaskDependents(tx *gorm.DB, workspaceID uint, ids []uint) error {
	for _, target := range []any{&TaskTestExecutionRun{}, &TaskTestExecution{}, &TaskTestDefinition{}, &WorkTaskComment{}, &WorkTaskWatcher{}, &WorkTaskActivity{}, &WorkTaskAttachment{}} {
		if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, ids).Delete(target).Error; err != nil {
			return err
		}
	}
	return nil
}

func frozenTaskTestExecutionIDs(tx *gorm.DB, workspaceID uint, taskIDs []uint) ([]uint, error) {
	var ids []uint
	if err := tx.Model(&TaskTestExecution{}).
		Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Order("id").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func deletionDepth(rows []WorkTask, id uint) int {
	parents := map[uint]*uint{}
	for index := range rows {
		parents[rows[index].ID] = rows[index].ParentID
	}
	depth, seen := 0, map[uint]struct{}{}
	for {
		parent, exists := parents[id]
		if !exists || parent == nil {
			return depth
		}
		if _, loop := seen[*parent]; loop {
			return depth
		}
		seen[*parent] = struct{}{}
		depth++
		id = *parent
	}
}

func normalizeDeletionError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrTaskDeletionNotFound
	}
	return err
}

var _ task.TaskDeletionStore = (*CoreMutationRepository)(nil)
