package api

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

var toolCommentUsernameMention = regexp.MustCompile(`@([\p{L}\p{N}_.-]+)`)
var toolCommentAttachmentMarkup = regexp.MustCompile(`!\[[^\]]*\]\(task-attach:\d+\)`)

type taskToolCollaborationNotifications struct {
	tx     *gorm.DB
	config config.Notification
}

func (writer taskToolCollaborationNotifications) RecordTaskComment(
	ctx context.Context,
	comment task.CommentNotification,
) error {
	if writer.tx == nil || comment.WorkspaceID == 0 || comment.PrincipalID == 0 || comment.TaskID == 0 ||
		comment.TeamID == 0 || comment.PlanUnitID == 0 {
		return task.ErrCollaborationMutationConflict
	}
	users, err := toolNotificationUsers(ctx, writer.tx, comment.WorkspaceID, comment.PrincipalID)
	if err != nil {
		return err
	}
	mentions := toolCommentMentionUserIDs(comment.Body, users)
	validated := make([]uint, 0, len(mentions))
	for _, id := range mentions {
		if _, ok := users[id]; ok {
			validated = append(validated, id)
		}
	}
	actor, err := identitygorm.NewToolDirectoryRepository(writer.tx).FindActiveUser(ctx, comment.PrincipalID)
	if err != nil {
		return err
	}
	actorName := strings.TrimSpace(actor.DisplayName)
	if actorName == "" {
		actorName = actor.Username
	}
	if err := notification.EnqueueCommentMention(writer.tx.WithContext(ctx), comment.WorkspaceID, comment.PrincipalID, comment.TeamID, comment.PlanUnitID, comment.TaskID, comment.CommentID, "work_task", validated, writer.config.PublicBaseURL, writer.config.Enabled, actorName, comment.Title); err != nil {
		return err
	}
	var watcherIDs, participantIDs []uint
	if err := writer.tx.WithContext(ctx).Model(&taskgorm.WorkTaskWatcher{}).
		Where("workspace_id = ? AND work_task_id = ?", comment.WorkspaceID, comment.TaskID).
		Pluck("user_id", &watcherIDs).Error; err != nil {
		return err
	}
	if err := writer.tx.WithContext(ctx).Model(&taskgorm.WorkTaskComment{}).
		Where("workspace_id = ? AND work_task_id = ?", comment.WorkspaceID, comment.TaskID).
		Distinct("user_id").Pluck("user_id", &participantIDs).Error; err != nil {
		return err
	}
	direct := []uint{comment.AssigneeUserID, comment.ReporterUserID}
	recipients := toolNotificationRecipients(comment.PrincipalID, false, direct, watcherIDs, participantIDs)
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
		"operator": toolNotificationUserLabel(users, comment.PrincipalID),
		"title":    strings.TrimSpace(comment.Title),
		"taskKey":  strings.TrimSpace(comment.TaskKey),
		"comment":  toolCommentExcerpt(comment.Body),
		"url":      notification.WorkItemURL(writer.config.PublicBaseURL, comment.TaskID) + "#comment-" + strconv.FormatUint(uint64(comment.CommentID), 10),
	}
	if vars["title"] == "" {
		vars["title"] = "未命名任务"
	}
	if vars["taskKey"] == "" {
		vars["taskKey"] = "TASK-" + strconv.FormatUint(uint64(comment.TaskID), 10)
	}
	return notification.Enqueue(writer.tx.WithContext(ctx), "work_task_notification", comment.TaskID, notification.Intent{
		ResourceType: "work_task", ResourceID: strconv.FormatUint(uint64(comment.TaskID), 10), CommentID: comment.CommentID,
		WorkspaceID: comment.WorkspaceID, Scene: notification.SceneWorkTaskCommented,
		TeamID: comment.TeamID, ProjectID: comment.PlanUnitID, ActorUserID: comment.PrincipalID,
		RecipientUserIDs: recipients, Vars: vars, SkipExternal: !writer.config.Enabled,
		FallbackPlainText: notification.PlainText(notification.SceneWorkTaskCommented, vars),
	})
}

func toolCommentExcerpt(body string) string {
	plain := quality.RichTextPlainText(body)
	plain = toolCommentAttachmentMarkup.ReplaceAllString(plain, "[图片]")
	lines := strings.Split(plain, "\n")
	visible := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "> #cmt:") {
			continue
		}
		visible = append(visible, strings.TrimPrefix(line, "> "))
	}
	value := []rune(strings.Join(visible, " "))
	if len(value) > 300 {
		value = value[:300]
	}
	return string(value)
}

func toolCommentMentionUserIDs(body string, users map[uint]identity.ToolUser) []uint {
	result := toolCommentStructuredMentionIDs(body)
	for _, match := range toolCommentUsernameMention.FindAllStringSubmatch(body, -1) {
		if len(match) < 2 {
			continue
		}
		username := strings.TrimSpace(match[1])
		for userID, user := range users {
			if username != "" && strings.EqualFold(username, strings.TrimSpace(user.Username)) {
				result = append(result, userID)
			}
		}
	}
	return result
}

func toolCommentStructuredMentionIDs(body string) []uint {
	body = strings.TrimPrefix(strings.TrimSpace(body), "\ufeff")
	if !strings.HasPrefix(body, "{") && !strings.HasPrefix(body, "[") {
		return nil
	}
	var root any
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil {
		return nil
	}
	seen := map[uint]struct{}{}
	result := make([]uint, 0)
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if kind, _ := typed["type"].(string); kind == "mention" {
				if attrs, ok := typed["attrs"].(map[string]any); ok {
					for _, key := range []string{"id", "userId", "user_id"} {
						id := toolCommentMentionID(attrs[key])
						if id > 0 {
							if _, exists := seen[id]; !exists {
								seen[id] = struct{}{}
								result = append(result, id)
							}
							break
						}
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(root)
	return result
}

func toolCommentMentionID(value any) uint {
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case json.Number:
		raw = typed.String()
	case float64:
		if typed <= 0 || typed != float64(uint(typed)) {
			return 0
		}
		return uint(typed)
	default:
		return 0
	}
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return 0
	}
	return uint(id)
}

var _ task.CollaborationNotificationWriter = taskToolCollaborationNotifications{}
