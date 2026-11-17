package taskgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

func (repository *ReadRepository) ListRelated(ctx context.Context, workspaceID, taskID uint) ([]task.RelatedItem, error) {
	current, err := repository.taskByID(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	base := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND project_set_id = ? AND requirement_id = ? AND mutation_state = ?",
		workspaceID, current.ProjectSetID, current.RequirementID, workTaskStateActive,
	)
	result := make([]task.RelatedItem, 0)
	appendRows := func(relation string, rows []WorkTask) {
		for _, row := range rows {
			result = append(result, task.RelatedItem{Relation: relation, Task: taskListItem(row)})
		}
	}
	if current.ParentID != nil {
		var parent WorkTask
		if err := base.Session(&gorm.Session{}).Where("id = ?", *current.ParentID).First(&parent).Error; err == nil {
			appendRows("parent", []WorkTask{parent})
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	var children []WorkTask
	if err := base.Session(&gorm.Session{}).Where("parent_id = ?", current.ID).Order("id DESC").Limit(200).Find(&children).Error; err != nil {
		return nil, err
	}
	appendRows("child", children)
	if current.ParentID != nil {
		var siblings []WorkTask
		if err := base.Session(&gorm.Session{}).Where("parent_id = ? AND id <> ?", *current.ParentID, current.ID).
			Order("id DESC").Limit(200).Find(&siblings).Error; err != nil {
			return nil, err
		}
		appendRows("sibling", siblings)
	}
	return result, nil
}

func (repository *ReadRepository) ListActivities(ctx context.Context, workspaceID, taskID uint, limit, offset int) ([]task.ActivitySnapshot, int64, error) {
	if _, err := repository.taskByID(ctx, workspaceID, taskID); err != nil {
		return nil, 0, err
	}
	query := repository.db.WithContext(ctx).Where("workspace_id = ? AND work_task_id = ?", workspaceID, taskID)
	var total int64
	if err := query.Model(&WorkTaskActivity{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []WorkTaskActivity
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]task.ActivitySnapshot, 0, len(rows))
	for _, row := range rows {
		field, oldValue, newValue, summary := row.FieldKey, row.OldValue, row.NewValue, row.Summary
		if field == "" && strings.HasPrefix(strings.TrimSpace(row.Detail), "{") {
			var legacy map[string]string
			if json.Unmarshal([]byte(row.Detail), &legacy) == nil {
				field, oldValue, newValue, summary = legacy["field_key"], legacy["old_value"], legacy["new_value"], legacy["summary"]
			}
		}
		result = append(result, task.ActivitySnapshot{
			ID: row.ID, WorkspaceID: row.WorkspaceID, WorkTaskID: row.WorkTaskID, UserID: row.UserID,
			Action: row.Action, Detail: row.Detail, FieldKey: field, OldValue: oldValue,
			NewValue: newValue, Summary: summary, CreatedAt: row.CreatedAt,
		})
	}
	return result, total, nil
}

func (repository *ReadRepository) ListComments(ctx context.Context, workspaceID, taskID uint) ([]task.CommentSnapshot, error) {
	if _, err := repository.taskByID(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	var rows []WorkTaskComment
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND work_task_id = ?", workspaceID, taskID).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]task.CommentSnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, task.CommentSnapshot{ID: row.ID, WorkspaceID: row.WorkspaceID, WorkTaskID: row.WorkTaskID, UserID: row.UserID, Body: row.Body, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

func (repository *ReadRepository) ListAttachments(ctx context.Context, workspaceID, taskID uint) ([]task.AttachmentSnapshot, error) {
	if _, err := repository.taskByID(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	var rows []WorkTaskAttachment
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND work_task_id = ? AND mutation_state = ?", workspaceID, taskID, attachmentStateActive).
		Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]task.AttachmentSnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, task.AttachmentSnapshot{
			ID: row.ID, WorkspaceID: row.WorkspaceID, WorkTaskID: row.WorkTaskID,
			UploaderUserID: row.UploaderUserID, Filename: row.Filename, ContentType: row.ContentType,
			Size: row.Size, StorageBackend: row.StorageBackend, CreatedAt: row.CreatedAt,
		})
	}
	return result, nil
}

func (repository *ReadRepository) ListDueConflicts(ctx context.Context, workspaceID, taskID uint, proposed time.Time) ([]task.DueConflict, error) {
	if _, err := repository.taskByID(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	seen := map[uint]struct{}{taskID: {}}
	frontier := []uint{taskID}
	result := make([]task.DueConflict, 0)
	for len(frontier) > 0 {
		var rows []WorkTask
		if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND parent_id IN ? AND mutation_state = ?", workspaceID, frontier, workTaskStateActive).
			Order("id ASC").Find(&rows).Error; err != nil {
			return nil, err
		}
		frontier = frontier[:0]
		for _, row := range rows {
			if _, exists := seen[row.ID]; exists {
				continue
			}
			seen[row.ID] = struct{}{}
			frontier = append(frontier, row.ID)
			if row.DueAt != nil && row.DueAt.After(proposed) {
				result = append(result, task.DueConflict{ID: row.ID, TaskKey: row.TaskKey, Title: row.Title, DueAt: row.DueAt.UTC()})
			}
			if len(seen) > 10001 {
				return nil, task.ErrReadQueryInvalid
			}
		}
	}
	return result, nil
}

func (repository *ReadRepository) taskByID(ctx context.Context, workspaceID, taskID uint) (WorkTask, error) {
	var row WorkTask
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND mutation_state = ?", workspaceID, taskID, workTaskStateActive).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WorkTask{}, task.ErrTaskNotFound
	}
	return row, err
}
