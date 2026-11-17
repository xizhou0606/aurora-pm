package notification

import (
	"context"
	"errors"
	"time"
)

// InboxFilter is recipient-scoped by the caller, never by request-supplied user IDs.
type InboxFilter struct {
	Before     uint
	Category   string
	UnreadOnly bool
}
type InboxPage struct {
	Items            []Mention        `json:"items"`
	NextCursor       uint             `json:"next_cursor,omitempty"`
	HighWatermark    uint             `json:"high_watermark"`
	UnreadByCategory map[string]int64 `json:"unread_by_category"`
	UnreadCount      int64            `json:"unread_count"`
}
type InboxStore interface {
	ListInbox(context.Context, uint, uint, InboxFilter) (InboxPage, error)
	MarkRead(context.Context, uint, uint, uint, time.Time) error
	MarkAllRead(context.Context, uint, uint, uint, string, time.Time) error
}

func InboxCategory(scene string) string {
	switch scene {
	case SceneDocumentCommentMentioned, SceneWorkTaskCommentMentioned, SceneQualityCommentMentioned:
		return "mention"
	case SceneWorkspaceAccessRequested, SceneWorkspaceAccessReviewed, SceneDocumentAccessRequested, SceneDocumentAccessReviewed, SceneProjectAccessRequested, SceneProjectAccessReviewed:
		return "access"
	case SceneQualityBugCreated, SceneQualityBugAssigned, SceneQualityBugReviewRequested, SceneQualityBugReturned, SceneQualityBugCommented, SceneWorkTaskAssigned, SceneWorkTaskCommented, SceneWorkTaskStatusChanged, SceneWorkflowNodeAssigned, SceneWorkflowPreReviewInvited, ScenePlanUnitMemberAssigned:
		return "message"
	default:
		return ""
	}
}

var ErrInboxNotFound = errors.New("notification not found")

// ValidInboxCategory accepts only the public category allowlist.
func ValidInboxCategory(category string) bool {
	return category == "" || category == "mention" || category == "message" || category == "access"
}
