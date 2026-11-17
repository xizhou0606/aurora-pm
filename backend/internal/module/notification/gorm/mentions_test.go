package notificationgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"testing"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

func TestMentionsAreRecipientScopedAndPaginated(t *testing.T) {
	db, err := database.Open(config.Config{Environment: "test", DatabaseDriver: "sqlite", DatabaseDSN: "file:mentions?mode=memory&cache=shared", AuthMode: "local", LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 32; index++ {
		intent := notification.DocumentCommentMention(7, 1, 11, 21, []uint{8}, "https://example.test")
		if err = notification.Enqueue(db, "document_comment", 31, intent); err != nil {
			t.Fatal(err)
		}
	}
	for _, scope := range [][2]uint{{7, 9}, {8, 8}} {
		if err = notification.Enqueue(db, "document_comment", 31, notification.DocumentCommentMention(scope[0], 1, 11, 21, []uint{scope[1]}, "https://example.test")); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewMentionRepository(db)
	page, err := repo.ListMentions(context.Background(), 7, 8, 0)
	if err != nil || len(page.Items) != 30 || page.NextCursor == 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].DocumentID != 11 || page.Items[0].ThreadID != 21 {
		t.Fatal(page.Items[0])
	}
	next, err := repo.ListMentions(context.Background(), 7, 8, page.NextCursor)
	if err != nil || len(next.Items) != 2 || next.NextCursor != 0 {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	empty, err := repo.ListMentions(context.Background(), 7, 10, 0)
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}

func TestAccessRequestMentionsUseRecipientAndWorkspaceProjection(t *testing.T) {
	db, err := database.Open(config.Config{Environment: "test", DatabaseDriver: "sqlite", DatabaseDSN: "file:access_mentions?mode=memory&cache=shared", AuthMode: "local", LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	for _, scene := range []string{notification.SceneDocumentAccessRequested, notification.SceneDocumentAccessReviewed} {
		for _, scope := range [][2]uint{{7, 8}, {8, 8}, {7, 9}} {
			vars := map[string]string{"resourceId": "dr_1", "status": "approved", "url": "https://example.test/workspace/documents/11#access-requests"}
			intent := notification.Intent{WorkspaceID: scope[0], Scene: scene, DocumentID: 11, RequestID: 12, ActorUserID: 1, RecipientUserIDs: []uint{scope[1]}, Vars: vars, FallbackPlainText: notification.PlainText(scene, vars)}
			if err = notification.Enqueue(db, "document_access_request", 12, intent); err != nil {
				t.Fatal(err)
			}
		}
	}
	page, err := NewMentionRepository(db).ListMentions(context.Background(), 7, 8, 0)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	for _, item := range page.Items {
		if item.DocumentID != 11 || item.RequestID != 12 || item.ThreadID != 0 || item.Scene == "" {
			t.Fatalf("projection=%+v", item)
		}
	}
	vars := map[string]string{"resourceId": "table_resource", "status": "approved"}
	intent := notification.Intent{WorkspaceID: 7, Scene: notification.SceneDocumentAccessReviewed, ResourceType: "bitable", ResourceID: "table_resource", RequestID: 13, ActorUserID: 1, RecipientUserIDs: []uint{8}, Vars: vars, FallbackPlainText: notification.PlainText(notification.SceneDocumentAccessReviewed, vars)}
	if err = notification.Enqueue(db, "document_access_request", 13, intent); err != nil {
		t.Fatal(err)
	}
	page, err = NewMentionRepository(db).ListMentions(context.Background(), 7, 8, 0)
	if err != nil || len(page.Items) != 3 || page.Items[0].ResourceType != "bitable" || page.Items[0].ResourceID != "table_resource" || page.Items[0].DocumentID != 0 {
		t.Fatalf("table projection=%+v err=%v", page, err)
	}
}
