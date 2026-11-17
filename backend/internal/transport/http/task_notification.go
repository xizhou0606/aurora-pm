package httpapi

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	taskdomain "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func (s *Server) recordTaskAssignmentNotification(
	tx *gorm.DB,
	c *gin.Context,
	task taskgorm.WorkTask,
	ctx workspaceContext,
) error {
	if task.AssigneeID == nil || *task.AssigneeID == 0 {
		return nil
	}
	actorID := currentFactUserID(c)
	// Assignment is addressed to the assignee even when they assigned the task
	// to themselves. A self-assignment is still a real task hand-off/reminder.
	recipients := []uint{*task.AssigneeID}
	vars := map[string]string{
		"operator": notificationPeopleLabel(tx, ctx, actorID),
		"title":    strings.TrimSpace(task.Title),
		"taskKey":  strings.TrimSpace(task.TaskKey),
		"kind":     taskdomain.WorkTaskKindLabel(task.Kind),
		"priority": taskdomain.WorkTaskPriorityLabel(task.Priority),
		"status":   taskdomain.WorkTaskStatusLabel(task.Status), "statusKey": task.Status,
		"assignee": notificationPeopleLabel(tx, ctx, *task.AssigneeID),
		"url":      s.taskNotificationURL(task.ID),
	}
	if vars["title"] == "" {
		vars["title"] = "未命名任务"
	}
	if vars["taskKey"] == "" {
		vars["taskKey"] = fmt.Sprintf("TASK-%d", task.ID)
	}
	return notification.Enqueue(tx, "work_task_notification", task.ID, notification.Intent{
		ResourceType: "work_task", ResourceID: fmt.Sprint(task.ID),
		WorkspaceID: task.WorkspaceID, Scene: notification.SceneWorkTaskAssigned,
		TeamID: task.TeamID, ProjectID: taskNotificationProjectID(task), ActorUserID: actorID,
		RecipientUserIDs: recipients, Vars: vars, SkipExternal: !s.cfg.Notification.Enabled,
		FallbackPlainText: taskAssignmentNotificationPlainText(vars),
	})
}

func (s *Server) recordTaskCommentNotification(
	tx *gorm.DB,
	c *gin.Context,
	task taskgorm.WorkTask,
	comment taskgorm.WorkTaskComment,
	ctx workspaceContext,
) error {
	validated, err := commentMentionWorkspaceRecipients(tx, task.WorkspaceID, comment.Body)
	if err != nil {
		return err
	}
	if err := notification.EnqueueCommentMention(tx, task.WorkspaceID, comment.UserID, task.TeamID, task.ProjectID, task.ID, comment.ID, "work_task", validated, s.cfg.Notification.PublicBaseURL, s.cfg.Notification.Enabled, notificationPeopleLabel(tx, ctx, comment.UserID), task.Title); err != nil {
		return err
	}
	var watcherIDs, participantIDs []uint
	if err := tx.Model(&taskgorm.WorkTaskWatcher{}).
		Where("workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID).
		Pluck("user_id", &watcherIDs).Error; err != nil {
		return err
	}
	if err := tx.Model(&taskgorm.WorkTaskComment{}).
		Where("workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID).
		Distinct("user_id").Pluck("user_id", &participantIDs).Error; err != nil {
		return err
	}
	direct := make([]uint, 0, 2)
	if task.AssigneeID != nil {
		direct = append(direct, *task.AssigneeID)
	}
	if task.ReporterUserID != nil {
		direct = append(direct, *task.ReporterUserID)
	}
	recipients := commentNotificationRecipients(
		comment.UserID, direct, watcherIDs, participantIDs, nil,
	)
	mentionSet := map[uint]bool{}
	for _, id := range validated {
		mentionSet[id] = true
	}
	remaining := recipients[:0]
	for _, id := range recipients {
		if !mentionSet[id] {
			remaining = append(remaining, id)
		}
	}
	recipients = remaining
	if len(recipients) == 0 {
		return nil
	}
	vars := map[string]string{
		"operator": notificationPeopleLabel(tx, ctx, comment.UserID),
		"title":    strings.TrimSpace(task.Title), "taskKey": strings.TrimSpace(task.TaskKey),
		"comment": commentNotificationExcerpt(comment.Body), "url": fmt.Sprintf("%s#comment-%d", s.taskNotificationURL(task.ID), comment.ID),
	}
	if vars["title"] == "" {
		vars["title"] = "未命名任务"
	}
	if vars["taskKey"] == "" {
		vars["taskKey"] = fmt.Sprintf("TASK-%d", task.ID)
	}
	return notification.Enqueue(tx, "work_task_notification", task.ID, notification.Intent{
		ResourceType: "work_task", ResourceID: fmt.Sprint(task.ID),
		WorkspaceID: task.WorkspaceID, Scene: notification.SceneWorkTaskCommented, CommentID: comment.ID,
		TeamID: task.TeamID, ProjectID: task.ProjectID, ActorUserID: comment.UserID,
		RecipientUserIDs: recipients, Vars: vars, SkipExternal: !s.cfg.Notification.Enabled,
		FallbackPlainText: taskCommentNotificationPlainText(vars),
	})
}

func taskNotificationProjectID(task taskgorm.WorkTask) uint {
	if task.ProjectID > 0 {
		return task.ProjectID
	}
	return task.ProjectSetID
}

func (s *Server) taskNotificationURL(taskID uint) string {
	return notification.WorkItemURL(s.cfg.Notification.PublicBaseURL, taskID)
}

func taskAssignmentNotificationPlainText(vars map[string]string) string {
	return notification.PlainText(notification.SceneWorkTaskAssigned, vars)
}

func taskCommentNotificationPlainText(vars map[string]string) string {
	return notification.PlainText(notification.SceneWorkTaskCommented, vars)
}

func notificationPeopleLabel(tx *gorm.DB, ctx workspaceContext, userID uint) string {
	if userID == 0 {
		return "-"
	}
	if label := strings.TrimSpace(userDisplayName(ctx, &userID)); ctx.Users[userID].ID != 0 && label != "" {
		return label
	}
	return workflowNotificationUserLabel(tx, userID)
}

func notificationUsersTx(tx *gorm.DB, userIDs ...uint) workspaceContext {
	ctx := workspaceContext{Users: map[uint]identitygorm.User{}}
	ids := make([]uint, 0, len(userIDs))
	seen := map[uint]struct{}{}
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		ids = append(ids, userID)
	}
	if tx == nil || len(ids) == 0 {
		return ctx
	}
	// Notification references use PM internal user IDs.
	var users []identitygorm.User
	_ = tx.Select("id", "username", "display_name").
		Where("id IN ?", ids).
		Find(&users).Error
	for _, user := range users {
		ctx.Users[user.ID] = user
	}

	return ctx
}

func taskAssigneeChanged(before, after *uint) bool {
	if before == nil || *before == 0 {
		return after != nil && *after > 0
	}
	if after == nil || *after == 0 {
		return true
	}
	return *before != *after
}
