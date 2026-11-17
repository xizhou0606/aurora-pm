package taskgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CollaborationNotificationFactory func(*gorm.DB) (task.CollaborationNotificationWriter, error)

type CollaborationMutationRepository struct {
	db                  *gorm.DB
	notificationFactory CollaborationNotificationFactory
}

func NewCollaborationMutationRepository(
	db *gorm.DB,
	notificationFactory CollaborationNotificationFactory,
) (*CollaborationMutationRepository, error) {
	if db == nil || notificationFactory == nil {
		return nil, task.ErrCollaborationMutationInvalid
	}
	return &CollaborationMutationRepository{db: db, notificationFactory: notificationFactory}, nil
}

func (repository *CollaborationMutationRepository) ApplyCollaborationMutation(
	ctx context.Context, command task.CollaborationMutation,
) (task.CollaborationMutationResult, error) {
	if repository == nil || repository.db == nil || repository.notificationFactory == nil {
		return task.CollaborationMutationResult{}, task.ErrCollaborationMutationInvalid
	}
	if replay, found, err := repository.replay(ctx, command); err != nil || found {
		return replay, err
	}
	var result task.CollaborationMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := TaskToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID,
			Operation: string(command.Operation), InputDigest: command.InputDigest, Status: taskMutationPending,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		taskRow, err := lockCollaborationTask(tx, command)
		if err != nil {
			return err
		}
		notifications, err := repository.notificationFactory(tx)
		if err != nil || notifications == nil {
			if err != nil {
				return err
			}
			return task.ErrCollaborationMutationInvalid
		}
		switch command.Operation {
		case task.CommentCreate:
			result, err = applyCommentCreate(tx, command)
			if err == nil && result.Comment != nil {
				assigneeID, reporterID := uint(0), uint(0)
				if taskRow.AssigneeID != nil {
					assigneeID = *taskRow.AssigneeID
				}
				if taskRow.ReporterUserID != nil {
					reporterID = *taskRow.ReporterUserID
				}
				err = notifications.RecordTaskComment(ctx, task.CommentNotification{
					CommentID:   result.Comment.ID,
					WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
					TeamID: taskRow.TeamID, ProjectSetID: taskRow.ProjectSetID, PlanUnitID: taskRow.ProjectID,
					TaskID: taskRow.ID, AssigneeUserID: assigneeID, ReporterUserID: reporterID,
					TaskKey: taskRow.TaskKey, Title: taskRow.Title, Body: command.Body,
				})
			}
		case task.CommentDelete:
			result, err = applyCommentDelete(tx, command)
		case task.WatchUpdate:
			result, err = applyWatchUpdate(tx, command)
		case task.AttachmentDelete:
			result, err = prepareAttachmentDelete(tx, command)
		default:
			err = task.ErrCollaborationMutationInvalid
		}
		if err != nil {
			return err
		}
		if command.Operation == task.AttachmentDelete {
			return nil
		}
		return finalizeTaskMutation(tx, ledger.ID, result)
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replay(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return task.CollaborationMutationResult{}, normalizeCollaborationMutationError(err)
}

func (repository *CollaborationMutationRepository) CompleteAttachmentDeletion(
	ctx context.Context, command task.CollaborationMutation,
) (task.CollaborationMutationResult, error) {
	var result task.CollaborationMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockCollaborationTask(tx, command); err != nil {
			return err
		}
		ledger, err := lockTaskMutationLedger(tx, command)
		if err != nil {
			return err
		}
		if ledger.Status == taskMutationSucceeded {
			return json.Unmarshal([]byte(ledger.ResultJSON), &result)
		}
		if ledger.Status != taskMutationPending || command.Operation != task.AttachmentDelete {
			return task.ErrCollaborationMutationConflict
		}
		var saga AttachmentDeletionSaga
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND work_task_id = ? AND attachment_id = ? AND apply_key = ?",
			command.WorkspaceID, command.TaskID, command.AttachmentID, command.ApplyKey,
		).First(&saga).Error; err != nil {
			return normalizeCollaborationMutationError(err)
		}
		if saga.Status != taskMutationPending || saga.ObjectKey == "" {
			return task.ErrCollaborationMutationConflict
		}
		deleted := tx.Where(
			"id = ? AND workspace_id = ? AND work_task_id = ? AND mutation_state = ?",
			command.AttachmentID, command.WorkspaceID, command.TaskID, attachmentStateDeleting,
		).Delete(&WorkTaskAttachment{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return task.ErrCollaborationMutationConflict
		}
		if err := tx.Create(&WorkTaskActivity{
			WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, UserID: command.PrincipalID,
			Action: "attachment_deleted", Summary: "删除任务附件",
		}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		updated := tx.Model(&AttachmentDeletionSaga{}).Where("id = ? AND status = ?", saga.ID, taskMutationPending).
			Updates(map[string]any{"status": taskMutationSucceeded, "completed_at": &now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return task.ErrCollaborationMutationConflict
		}
		result = task.CollaborationMutationResult{DeletedID: command.AttachmentID, Deleted: true}
		return finalizeTaskMutation(tx, ledger.ID, result)
	})
	return result, normalizeCollaborationMutationError(err)
}

func (repository *CollaborationMutationRepository) replay(
	ctx context.Context, command task.CollaborationMutation,
) (task.CollaborationMutationResult, bool, error) {
	var ledger TaskToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.CollaborationMutationResult{}, false, nil
	}
	if err != nil {
		return task.CollaborationMutationResult{}, false, err
	}
	if !taskMutationLedgerMatches(ledger, command) {
		return task.CollaborationMutationResult{}, true, task.ErrCollaborationMutationConflict
	}
	if ledger.Status == taskMutationSucceeded {
		var result task.CollaborationMutationResult
		if ledger.ResultJSON == "" || json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
			return task.CollaborationMutationResult{}, true, task.ErrCollaborationMutationConflict
		}
		return result, true, nil
	}
	if ledger.Status != taskMutationPending || command.Operation != task.AttachmentDelete {
		return task.CollaborationMutationResult{}, true, task.ErrCollaborationMutationConflict
	}
	var saga AttachmentDeletionSaga
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND work_task_id = ? AND attachment_id = ? AND apply_key = ? AND status = ?",
		command.WorkspaceID, command.TaskID, command.AttachmentID, command.ApplyKey, taskMutationPending,
	).First(&saga).Error; err != nil {
		return task.CollaborationMutationResult{}, true, normalizeCollaborationMutationError(err)
	}
	return task.CollaborationMutationResult{DeletedID: command.AttachmentID, PendingObjectKey: saga.ObjectKey}, true, nil
}

func lockTaskMutationLedger(tx *gorm.DB, command task.CollaborationMutation) (TaskToolMutationApply, error) {
	var ledger TaskToolMutationApply
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID).First(&ledger).Error
	if err != nil {
		return TaskToolMutationApply{}, normalizeCollaborationMutationError(err)
	}
	if !taskMutationLedgerMatches(ledger, command) {
		return TaskToolMutationApply{}, task.ErrCollaborationMutationConflict
	}
	return ledger, nil
}

func taskMutationLedgerMatches(ledger TaskToolMutationApply, command task.CollaborationMutation) bool {
	return ledger.WorkspaceID == command.WorkspaceID && ledger.ApplyKey == command.ApplyKey &&
		ledger.ChangeSetID == command.ChangeSetID && ledger.Operation == string(command.Operation) && ledger.InputDigest == command.InputDigest
}

func finalizeTaskMutation(tx *gorm.DB, ledgerID uint, result task.CollaborationMutationResult) error {
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
		return task.ErrCollaborationMutationConflict
	}
	return nil
}

func lockCollaborationTask(tx *gorm.DB, command task.CollaborationMutation) (WorkTask, error) {
	var row WorkTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND mutation_state = ?", command.TaskID, command.WorkspaceID, workTaskStateActive).First(&row).Error
	if err != nil {
		return WorkTask{}, normalizeCollaborationMutationError(err)
	}
	if row.TeamID != command.TaskScope.TeamID || row.ProjectSetID != command.TaskScope.ProjectSetID ||
		row.ProjectID != command.TaskScope.PlanUnitID || row.RequirementID != command.TaskScope.RequirementID {
		return WorkTask{}, task.ErrCollaborationMutationConflict
	}
	return row, nil
}

func applyCommentCreate(tx *gorm.DB, command task.CollaborationMutation) (task.CollaborationMutationResult, error) {
	row := WorkTaskComment{WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, UserID: command.PrincipalID, Body: command.Body}
	if err := tx.Create(&row).Error; err != nil {
		return task.CollaborationMutationResult{}, err
	}
	if err := tx.Create(&WorkTaskActivity{
		WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, UserID: command.PrincipalID,
		Action: "comment", Detail: truncateCollaborationText(command.Body, 500),
	}).Error; err != nil {
		return task.CollaborationMutationResult{}, err
	}
	comment := task.CommentSnapshot{ID: row.ID, WorkspaceID: row.WorkspaceID, WorkTaskID: row.WorkTaskID, UserID: row.UserID, Body: row.Body, CreatedAt: row.CreatedAt}
	return task.CollaborationMutationResult{Comment: &comment}, nil
}

func applyCommentDelete(tx *gorm.DB, command task.CollaborationMutation) (task.CollaborationMutationResult, error) {
	var row WorkTaskComment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", command.CommentID, command.WorkspaceID, command.TaskID,
	).First(&row).Error; err != nil {
		return task.CollaborationMutationResult{}, normalizeCollaborationMutationError(err)
	}
	if row.UserID != command.PrincipalID {
		return task.CollaborationMutationResult{}, task.ErrCollaborationMutationForbidden
	}
	deleted := tx.Where("id = ? AND workspace_id = ? AND work_task_id = ? AND user_id = ?",
		row.ID, command.WorkspaceID, command.TaskID, command.PrincipalID).Delete(&WorkTaskComment{})
	if deleted.Error != nil {
		return task.CollaborationMutationResult{}, deleted.Error
	}
	if deleted.RowsAffected != 1 {
		return task.CollaborationMutationResult{}, task.ErrCollaborationMutationConflict
	}
	if err := tx.Create(&WorkTaskActivity{
		WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, UserID: command.PrincipalID,
		Action: "comment_deleted", Detail: truncateCollaborationText(row.Body, 500), Summary: "删除任务评论",
	}).Error; err != nil {
		return task.CollaborationMutationResult{}, err
	}
	return task.CollaborationMutationResult{DeletedID: row.ID, Deleted: true}, nil
}

func applyWatchUpdate(tx *gorm.DB, command task.CollaborationMutation) (task.CollaborationMutationResult, error) {
	if command.Watch {
		row := WorkTaskWatcher{WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, UserID: command.PrincipalID}
		if err := tx.Where("workspace_id = ? AND work_task_id = ? AND user_id = ?", command.WorkspaceID, command.TaskID, command.PrincipalID).
			FirstOrCreate(&row).Error; err != nil {
			return task.CollaborationMutationResult{}, err
		}
	} else if err := tx.Where("workspace_id = ? AND work_task_id = ? AND user_id = ?", command.WorkspaceID, command.TaskID, command.PrincipalID).
		Delete(&WorkTaskWatcher{}).Error; err != nil {
		return task.CollaborationMutationResult{}, err
	}
	action, summary := "unwatch", "已取消关注此工作项"
	if command.Watch {
		action, summary = "watch", "已关注此工作项"
	}
	if err := tx.Create(&WorkTaskActivity{
		WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, UserID: command.PrincipalID,
		Action: action, Summary: summary,
	}).Error; err != nil {
		return task.CollaborationMutationResult{}, err
	}
	watched := command.Watch
	return task.CollaborationMutationResult{WorkTaskID: command.TaskID, Watched: &watched}, nil
}

func prepareAttachmentDelete(tx *gorm.DB, command task.CollaborationMutation) (task.CollaborationMutationResult, error) {
	var row WorkTaskAttachment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ? AND mutation_state = ?",
		command.AttachmentID, command.WorkspaceID, command.TaskID, attachmentStateActive,
	).First(&row).Error; err != nil {
		return task.CollaborationMutationResult{}, normalizeCollaborationMutationError(err)
	}
	if strings.TrimSpace(row.ObjectKey) == "" || row.StorageBackend != "s3" {
		return task.CollaborationMutationResult{}, task.ErrCollaborationMutationConflict
	}
	updated := tx.Model(&WorkTaskAttachment{}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ? AND mutation_state = ?",
		row.ID, command.WorkspaceID, command.TaskID, attachmentStateActive,
	).Update("mutation_state", attachmentStateDeleting)
	if updated.Error != nil {
		return task.CollaborationMutationResult{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return task.CollaborationMutationResult{}, task.ErrCollaborationMutationConflict
	}
	saga := AttachmentDeletionSaga{
		WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, AttachmentID: row.ID,
		ApplyKey: command.ApplyKey, ObjectKey: row.ObjectKey, Status: taskMutationPending,
	}
	if err := tx.Create(&saga).Error; err != nil {
		return task.CollaborationMutationResult{}, err
	}
	return task.CollaborationMutationResult{DeletedID: row.ID, PendingObjectKey: row.ObjectKey}, nil
}

func truncateCollaborationText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func normalizeCollaborationMutationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrCollaborationMutationNotFound
	}
	return err
}

var _ task.CollaborationMutationStore = (*CollaborationMutationRepository)(nil)
