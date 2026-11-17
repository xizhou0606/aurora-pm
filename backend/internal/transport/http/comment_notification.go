package httpapi

import (
	"regexp"
	"strings"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

var commentAttachmentMarkup = regexp.MustCompile(`!\[[^\]]*\]\(task-attach:\d+\)`)

func commentNotificationExcerpt(body string) string {
	plain := quality.RichTextPlainText(body)
	plain = commentAttachmentMarkup.ReplaceAllString(plain, "[图片]")
	lines := strings.Split(plain, "\n")
	visible := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "> #cmt:") {
			continue
		}
		visible = append(visible, strings.TrimPrefix(line, "> "))
	}
	return truncateTaskComment(strings.Join(visible, " "), 300)
}

func commentNotificationRecipients(actorID uint, groups ...[]uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0)
	for _, group := range groups {
		for _, id := range group {
			if id == 0 || id == actorID {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

func commentMentionRecipientIDs(body string, ctx workspaceContext) []uint {
	result := append([]uint(nil), taskCommentMentionIDs(body)...)
	for _, match := range taskCommentUsernameMention.FindAllStringSubmatch(body, -1) {
		if len(match) < 2 {
			continue
		}
		username := strings.TrimSpace(match[1])
		for userID, user := range ctx.Users {
			if username != "" && strings.EqualFold(username, strings.TrimSpace(user.Username)) {
				result = append(result, userID)
			}
		}
	}
	return result
}

// Resolve mentions independently of the small display context for the business
// object: a mentioned workspace member need not be an assignee or participant.
func commentMentionWorkspaceRecipients(tx *gorm.DB, workspaceID uint, body string) ([]uint, error) {
	ids := taskCommentMentionIDs(body)
	usernames := make([]string, 0)
	for _, match := range taskCommentUsernameMention.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 {
			usernames = append(usernames, strings.ToLower(strings.TrimSpace(match[1])))
		}
	}
	if len(ids) == 0 && len(usernames) == 0 {
		return nil, nil
	}
	var users []identitygorm.User
	query := tx.Model(&identitygorm.User{}).
		Joins("JOIN workspace_members wm ON wm.user_id = users.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Where("users.active = ?", true)
	if err := query.Where("("+pmUserFactIDSQL+" IN ? OR LOWER(users.username) IN ?)", ids, usernames).
		Distinct("users.*").Find(&users).Error; err != nil {
		return nil, err
	}
	resolved := workspaceContext{Users: make(map[uint]identitygorm.User, len(users))}
	for _, user := range users {
		resolved.Users[factUserID(user)] = user
	}
	recipients := make([]uint, 0, len(users))
	for _, id := range commentMentionRecipientIDs(body, resolved) {
		if _, valid := resolved.Users[id]; valid {
			recipients = append(recipients, id)
		}
	}
	return mergeSortedUserIDs(recipients), nil
}
