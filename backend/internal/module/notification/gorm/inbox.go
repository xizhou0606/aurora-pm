package notificationgorm

import (
	"context"
	"encoding/json"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"gorm.io/gorm"
	"time"
)

func (r *MentionRepository) inbox(ctx context.Context, workspaceID, userID uint) *gorm.DB {
	return r.db.WithContext(ctx).Model(&records.InboxReceipt{}).Where("workspace_id = ? AND recipient_user_id = ?", workspaceID, userID)
}
func (r *MentionRepository) ListInbox(ctx context.Context, workspaceID, userID uint, filter notification.InboxFilter) (notification.InboxPage, error) {
	page := notification.InboxPage{Items: []notification.Mention{}, UnreadByCategory: map[string]int64{"mention": 0, "message": 0, "access": 0}}
	if workspaceID == 0 || userID == 0 {
		return page, gorm.ErrRecordNotFound
	}
	if err := r.inbox(ctx, workspaceID, userID).Where("read_at IS NULL").Count(&page.UnreadCount).Error; err != nil {
		return page, err
	}
	var counts []struct {
		Category string
		Count    int64
	}
	if err := r.inbox(ctx, workspaceID, userID).Select("category, COUNT(*) AS count").Where("read_at IS NULL").Group("category").Scan(&counts).Error; err != nil {
		return page, err
	}
	for _, item := range counts {
		page.UnreadByCategory[item.Category] = item.Count
	}
	if err := r.inbox(ctx, workspaceID, userID).Select("COALESCE(MAX(id),0)").Scan(&page.HighWatermark).Error; err != nil {
		return page, err
	}
	query := r.inbox(ctx, workspaceID, userID)
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Before > 0 {
		query = query.Where("id < ?", filter.Before)
	}
	if filter.UnreadOnly {
		query = query.Where("read_at IS NULL")
	}
	var rows []records.InboxReceipt
	if err := query.Order("id DESC").Limit(31).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > 30 {
		rows = rows[:30]
		page.NextCursor = rows[29].ID
	}
	eventIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		eventIDs = append(eventIDs, row.EventID)
	}
	var events []auditgorm.OutboxEvent
	if len(eventIDs) > 0 {
		if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id IN ?", workspaceID, eventIDs).Find(&events).Error; err != nil {
			return page, err
		}
	}
	byID := map[uint]auditgorm.OutboxEvent{}
	for _, event := range events {
		byID[event.ID] = event
	}
	for _, row := range rows {
		event, ok := byID[row.EventID]
		if !ok {
			continue
		}
		var intent notification.Intent
		if json.Unmarshal([]byte(event.Payload), &intent) != nil || intent.WorkspaceID != workspaceID {
			continue
		}
		addressed := false
		for _, id := range intent.RecipientUserIDs {
			if id == userID {
				addressed = true
			}
		}
		if !addressed || notification.InboxCategory(intent.Scene) == "" || notification.InboxCategory(intent.Scene) != row.Category {
			continue
		}
		// Historical document requests kept the canonical resource ID in vars.
		// Restore the typed route without asking the browser to resolve each item.
		if intent.Scene == notification.SceneDocumentAccessRequested || intent.Scene == notification.SceneDocumentAccessReviewed {
			if intent.ResourceID == "" {
				intent.ResourceID = intent.Vars["resourceId"]
			}
			if intent.ResourceType == "" && intent.DocumentID > 0 {
				intent.ResourceType = "rich_text"
			}
		}
		page.Items = append(page.Items, notification.InboxProjection(intent, row.ID, userID, event.OccurredAt, row.ReadAt))
	}
	if r.enrich != nil {
		if err := r.enrich(ctx, workspaceID, page.Items); err != nil {
			return page, err
		}
	}
	return page, nil
}
func (r *MentionRepository) MarkRead(ctx context.Context, w, u, id uint, now time.Time) error {
	if w == 0 || u == 0 || id == 0 {
		return notification.ErrInboxNotFound
	}
	var count int64
	if err := r.inbox(ctx, w, u).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return notification.ErrInboxNotFound
	}
	return r.inbox(ctx, w, u).Where("id = ? AND read_at IS NULL", id).Update("read_at", now).Error
}
func (r *MentionRepository) MarkAllRead(ctx context.Context, w, u, through uint, category string, now time.Time) error {
	if w == 0 || u == 0 || through == 0 {
		return notification.ErrInboxNotFound
	}
	if !notification.ValidInboxCategory(category) {
		return notification.ErrInboxNotFound
	}
	query := r.inbox(ctx, w, u).Where("id <= ? AND read_at IS NULL", through)
	if category != "" {
		query = query.Where("category = ?", category)
	}
	return query.Update("read_at", now).Error
}

// MarkRequestReviewed marks only the acting reviewer's original request receipt.
// The caller owns the approval transaction; other reviewers and result notices stay unread.
func (r *MentionRepository) MarkRequestReviewed(ctx context.Context, workspaceID, reviewerID, requestID uint, aggregate string) error {
	if workspaceID == 0 || reviewerID == 0 || requestID == 0 {
		return notification.ErrInboxNotFound
	}
	events := r.db.WithContext(ctx).Model(&auditgorm.OutboxEvent{}).Select("id").Where("workspace_id = ? AND aggregate_type = ? AND aggregate_id = ? AND event_type IN ?", workspaceID, aggregate, requestID, []string{notification.EventType(notification.SceneWorkspaceAccessRequested), notification.EventType(notification.SceneProjectAccessRequested), notification.EventType(notification.SceneDocumentAccessRequested)})
	return r.inbox(ctx, workspaceID, reviewerID).Where("event_id IN (?) AND read_at IS NULL", events).Update("read_at", time.Now().UTC()).Error
}
