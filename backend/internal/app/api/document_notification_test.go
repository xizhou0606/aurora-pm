package api

import (
	"context"
	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	notificationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"gorm.io/gorm"
	"testing"
)

func TestDocumentMentionInboxWithExternalDeliveryDisabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&auditgorm.AuditLog{}, &auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	recorder, err := newDocumentMutationRecorder(db)
	if err != nil {
		t.Fatal(err)
	}
	err = recorder.RecordDocumentMutation(context.Background(), document.MutationEffect{
		WorkspaceID: 7, PrincipalID: 110, Resource: "document_comment", ResourceID: 31, Action: "comment_replied", Payload: map[string]any{},
		CommentMention: &document.CommentMentionNotification{DocumentID: 11, ThreadID: 21, CommentID: 31, RecipientUserIDs: []uint{129}},
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := notificationgorm.NewMentionRepository(db).ListMentions(context.Background(), 7, 129, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].ThreadID != 21 {
		t.Fatalf("inbox: %+v %v", page, err)
	}
}
