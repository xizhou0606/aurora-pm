package httpapi

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func (s *Server) recordTaskStatusNotification(tx *gorm.DB, row taskgorm.WorkTask, previous string, actorID uint) error {
	if previous == row.Status {
		return nil
	}
	snapshot := task.Snapshot{ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, PlanUnitID: row.ProjectID,
		Title: row.Title, TaskKey: row.TaskKey, Status: row.Status, AssigneeID: row.AssigneeID, ReporterUserID: row.ReporterUserID}
	intent, send := task.StatusNotification(snapshot, previous, actorID, workflowNotificationUserLabel(tx, actorID), s.cfg.Notification.PublicBaseURL, s.cfg.Notification.Enabled)
	if !send {
		return nil
	}
	return notification.Enqueue(tx, "work_task_notification", row.ID, intent)
}
