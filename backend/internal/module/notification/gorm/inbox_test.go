package notificationgorm

import (
	"context"
	"errors"
	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestInboxIsolationCategoriesAndReadBoundary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := NewMentionRepository(db)
	add := func(w, u, comment uint) {
		t.Helper()
		if err := db.Transaction(func(tx *gorm.DB) error {
			return notification.EnqueueCommentMention(tx, w, 1, 11, 21, 31, comment, "work_task", []uint{u, u, 1}, "https://example.test", false, "测试用户", "测试任务")
		}); err != nil {
			t.Fatal(err)
		}
	}
	add(7, 8, 1)
	add(7, 8, 1)
	add(7, 9, 2)
	add(8, 8, 3)
	page, err := repo.ListInbox(ctx, 7, 8, notification.InboxFilter{Category: "mention"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.UnreadCount != 1 || page.UnreadByCategory["mention"] != 1 {
		t.Fatalf("page=%+v", page)
	}
	var count int64
	db.Model(&auditgorm.OutboxEvent{}).Count(&count)
	if count != 3 {
		t.Fatalf("duplicate external events: %d", count)
	}
	if err := repo.MarkRead(ctx, 7, 9, page.Items[0].ID, time.Now()); err == nil {
		t.Fatal("cross-recipient write accepted")
	}
	if err := repo.MarkRead(ctx, 8, 8, page.Items[0].ID, time.Now()); err == nil {
		t.Fatal("cross-workspace write accepted")
	}
	add(7, 8, 4)
	if err := repo.MarkAllRead(ctx, 7, 8, page.HighWatermark, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	next, _ := repo.ListInbox(ctx, 7, 8, notification.InboxFilter{UnreadOnly: true})
	if len(next.Items) != 1 || next.Items[0].CommentID != 4 {
		t.Fatalf("new arrival read: %+v", next)
	}
	access, _ := repo.ListInbox(ctx, 7, 8, notification.InboxFilter{Category: "access"})
	if len(access.Items) != 0 {
		t.Fatal("category filtering failed")
	}
	sentinel := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := notification.EnqueueCommentMention(tx, 7, 1, 11, 21, 31, 5, "work_task", []uint{8}, "", false, "测试用户", "测试任务"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	after, _ := repo.ListInbox(ctx, 7, 8, notification.InboxFilter{})
	if len(after.Items) != 2 {
		t.Fatal("receipt survived rollback")
	}
	for i := uint(10); i < 45; i++ {
		add(7, 8, i)
	}
	first, _ := repo.ListInbox(ctx, 7, 8, notification.InboxFilter{})
	second, _ := repo.ListInbox(ctx, 7, 8, notification.InboxFilter{Before: first.NextCursor})
	if len(first.Items) != 30 || len(second.Items) != 7 {
		t.Fatalf("pagination %d/%d", len(first.Items), len(second.Items))
	}
	if first.Items[29].ID <= second.Items[0].ID {
		t.Fatal("cursor overlap")
	}
}

func TestInboxRestoresLegacyDocumentRequestResource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	intent := notification.Intent{WorkspaceID: 7, Scene: notification.SceneDocumentAccessRequested, DocumentID: 11, RequestID: 21, ActorUserID: 1, RecipientUserIDs: []uint{8}, SkipExternal: true, Vars: map[string]string{"resourceId": "dr_11", "url": "https://example.test"}, FallbackPlainText: "申请编辑"}
	if err := notification.Enqueue(db, "document_access_request", 21, intent); err != nil {
		t.Fatal(err)
	}
	page, err := NewMentionRepository(db).ListInbox(context.Background(), 7, 8, notification.InboxFilter{})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].ResourceID != "dr_11" || page.Items[0].ResourceType != "rich_text" {
		t.Fatalf("legacy request not actionable: %+v", page.Items[0])
	}
}

func TestCategoryReadAndReviewReceiptIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := NewMentionRepository(db)
	for _, aggregate := range []string{"membership_access_request", "document_access_request"} {
		scene := notification.SceneProjectAccessRequested
		if aggregate == "document_access_request" {
			scene = notification.SceneDocumentAccessRequested
		}
		intent := notification.Intent{WorkspaceID: 7, Scene: scene, ActorUserID: 1, RecipientUserIDs: []uint{8, 9}, RequestID: 21, FallbackPlainText: "request", TeamID: 11, ProjectID: 21}
		if aggregate == "document_access_request" {
			intent.TeamID, intent.ProjectID = 0, 0
			intent.Vars = map[string]string{"resourceId": "dr_fixture"}
		}
		if err := notification.Enqueue(db, aggregate, 21, intent); err != nil {
			t.Fatal(err)
		}
		if err := repo.MarkRequestReviewed(ctx, 7, 8, 21, aggregate); err != nil {
			t.Fatal(err)
		}
	}
	reviewed, _ := repo.ListInbox(ctx, 7, 8, notification.InboxFilter{})
	other, _ := repo.ListInbox(ctx, 7, 9, notification.InboxFilter{})
	if reviewed.UnreadCount != 0 || other.UnreadCount != 2 {
		t.Fatalf("review read scope: %d/%d", reviewed.UnreadCount, other.UnreadCount)
	}
	if err := notification.EnqueueCommentMention(db, 7, 1, 11, 21, 31, 8, "work_task", []uint{9}, "", false, "actor", "title"); err != nil {
		t.Fatal(err)
	}
	page, _ := repo.ListInbox(ctx, 7, 9, notification.InboxFilter{})
	if err := repo.MarkAllRead(ctx, 7, 9, page.HighWatermark, "access", time.Now()); err != nil {
		t.Fatal(err)
	}
	page, _ = repo.ListInbox(ctx, 7, 9, notification.InboxFilter{})
	if page.UnreadCount != 1 || page.UnreadByCategory["mention"] != 1 || page.UnreadByCategory["access"] != 0 {
		t.Fatalf("category read leaked: %+v", page)
	}
}
