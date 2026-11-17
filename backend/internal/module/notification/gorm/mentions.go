package notificationgorm

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"gorm.io/gorm"
)

type MentionRepository struct {
	db     *gorm.DB
	enrich func(context.Context, uint, []notification.Mention) error
}

// ConfigureInboxPresentation enriches only already recipient-validated messages.
func (r *MentionRepository) ConfigureInboxPresentation(enrich func(context.Context, uint, []notification.Mention) error) {
	r.enrich = enrich
}

func NewMentionRepository(db *gorm.DB) *MentionRepository { return &MentionRepository{db: db} }

func (r *MentionRepository) ListMentions(ctx context.Context, workspaceID, userID, before uint) (notification.MentionPage, error) {
	result := notification.MentionPage{Items: make([]notification.Mention, 0)}
	if workspaceID == 0 || userID == 0 {
		return result, gorm.ErrRecordNotFound
	}
	query := r.db.WithContext(ctx).Model(&auditgorm.OutboxEvent{}).
		Where("workspace_id = ? AND event_type IN ?", workspaceID, []string{notification.EventType(notification.SceneDocumentCommentMentioned), notification.EventType(notification.SceneDocumentAccessRequested), notification.EventType(notification.SceneDocumentAccessReviewed)})
	// Recipient filtering happens before pagination, never in the browser.
	if r.db.Dialector.Name() == "postgres" {
		query = query.Where("payload::jsonb -> 'recipient_user_ids' @> ?::jsonb", "["+strconv.FormatUint(uint64(userID), 10)+"]")
	} else {
		query = query.Where("EXISTS (SELECT 1 FROM json_each(payload, '$.recipient_user_ids') WHERE value = ?)", userID)
	}
	if before > 0 {
		query = query.Where("id < ?", before)
	}
	var rows []auditgorm.OutboxEvent
	if err := query.Select("id", "payload", "occurred_at").Order("id DESC").Limit(31).Find(&rows).Error; err != nil {
		return result, err
	}
	if len(rows) > 30 {
		rows = rows[:30]
		result.NextCursor = rows[29].ID
	}
	for _, row := range rows {
		var intent notification.Intent
		if json.Unmarshal([]byte(row.Payload), &intent) != nil || intent.WorkspaceID != workspaceID || !notification.ValidScope(intent) {
			continue
		}
		if intent.Scene == notification.SceneDocumentAccessRequested || intent.Scene == notification.SceneDocumentAccessReviewed {
			if (intent.DocumentID > 0 || intent.ResourceType == "bitable" && intent.ResourceID != "") && intent.RequestID > 0 {
				result.Items = append(result.Items, notification.Mention{ID: row.ID, ResourceType: intent.ResourceType, ResourceID: intent.ResourceID, DocumentID: intent.DocumentID, RequestID: intent.RequestID, Scene: intent.Scene, ActorUserID: intent.ActorUserID, OccurredAt: row.OccurredAt})
			}
			continue
		}
		if intent.Scene != notification.SceneDocumentCommentMentioned {
			continue
		}
		thread := intent.ThreadID
		if thread == 0 {
			// Compatibility with previously queued URL-only notifications.
			link, err := url.Parse(intent.Vars["url"])
			if err != nil || !strings.HasPrefix(link.Fragment, "comment-") {
				continue
			}
			parsed, err := strconv.ParseUint(strings.TrimPrefix(link.Fragment, "comment-"), 10, 64)
			if err != nil || parsed == 0 {
				continue
			}
			thread = uint(parsed)
		}
		result.Items = append(result.Items, notification.Mention{ID: row.ID, Scene: intent.Scene, DocumentID: intent.DocumentID, ThreadID: thread, ActorUserID: intent.ActorUserID, OccurredAt: row.OccurredAt})
	}
	return result, nil
}
