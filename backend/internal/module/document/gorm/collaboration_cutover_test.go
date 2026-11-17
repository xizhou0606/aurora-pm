package documentgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func TestCollaborationCutoverUsesCallerTransactionAndSourceCAS(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&Document{}); err != nil {
		t.Fatal(err)
	}
	content, hash, plainText, err := document.NormalizeContent(document.DefaultContent())
	if err != nil {
		t.Fatal(err)
	}
	row := Document{
		WorkspaceID: 7, NodeID: 2, ContentJSON: string(content), SchemaVersion: document.SchemaVersionV1,
		Revision: 4, ContentHash: hash, PlainText: plainText, OwnerUserID: 5, CreatedBy: 5, UpdatedBy: 5,
	}
	if err = db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		locked, lockErr := LockDocumentForCollaboration(context.Background(), tx, 7, row.ID)
		if lockErr != nil || locked.Revision != 4 || locked.ContentHash != hash {
			t.Fatalf("locked=%+v err=%v", locked, lockErr)
		}
		if markErr := MarkDocumentCollaborationCutover(
			context.Background(), tx, 7, row.ID, 4, hash, time.Now().UTC(),
		); markErr != nil {
			t.Fatal(markErr)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("rollback error=%v", err)
	}
	var afterRollback Document
	if err = db.First(&afterRollback, row.ID).Error; err != nil || afterRollback.CollaborationCutoverAt != nil {
		t.Fatalf("after rollback=%+v err=%v", afterRollback, err)
	}
	cutoverAt := time.Now().UTC()
	if err = db.Transaction(func(tx *gorm.DB) error {
		return MarkDocumentCollaborationCutover(context.Background(), tx, 7, row.ID, 4, hash, cutoverAt)
	}); err != nil {
		t.Fatal(err)
	}
	var committed Document
	if err = db.First(&committed, row.ID).Error; err != nil || committed.CollaborationCutoverAt == nil {
		t.Fatalf("committed=%+v err=%v", committed, err)
	}
	legacy := documentFromRecord(committed)
	legacy.PlainText = "must not become a second body"
	if _, err = NewRepository(db).UpdateDocument(
		context.Background(), legacy, committed.Revision, 5,
	); !errors.Is(err, document.ErrLegacyContentWriteDisabled) {
		t.Fatalf("legacy write after cutover error=%v", err)
	}
	var afterRejectedWrite Document
	if err = db.First(&afterRejectedWrite, row.ID).Error; err != nil || afterRejectedWrite.PlainText != committed.PlainText {
		t.Fatalf("legacy content changed after cutover: row=%+v err=%v", afterRejectedWrite, err)
	}
	if err = MarkDocumentCollaborationCutover(
		context.Background(), db, 7, row.ID, 4, hash, cutoverAt,
	); !errors.Is(err, document.ErrRevisionConflict) {
		t.Fatalf("repeated cutover error=%v", err)
	}
}
