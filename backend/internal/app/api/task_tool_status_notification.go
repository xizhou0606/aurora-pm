package api

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func (writer taskToolAssignmentWriter) recordStatusChange(ctx context.Context, workspaceID, principalID, taskID uint, previous string) error {
	if previous == "" {
		return nil
	}
	if writer.tx == nil || writer.tasks == nil {
		return task.ErrBatchMutationUnavailable
	}
	snapshot, err := writer.tasks.GetTask(ctx, workspaceID, task.Lookup{ID: taskID})
	if err != nil {
		return err
	}
	if snapshot.WorkspaceID != workspaceID || snapshot.ID != taskID {
		return task.ErrBatchMutationConflict
	}
	if snapshot.Status == previous {
		return nil
	}
	users, err := toolNotificationUsers(ctx, writer.tx, workspaceID, principalID)
	if err != nil {
		return err
	}
	intent, send := task.StatusNotification(snapshot, previous, principalID, toolNotificationUserLabel(users, principalID), writer.config.PublicBaseURL, writer.config.Enabled)
	if !send {
		return nil
	}
	return notification.Enqueue(writer.tx.WithContext(ctx), "work_task_notification", taskID, intent)
}
