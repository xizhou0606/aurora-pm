package notification

import (
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"testing"
)

func TestDocumentMentionWorkspaceScopeAndRecipients(t *testing.T) {
	intent := DocumentCommentMention(7, 5, 11, 21, []uint{5, 8, 8, 0, 9}, "https://pm.example.test")
	if !ValidScope(intent) || len(intent.RecipientUserIDs) != 2 || intent.RecipientUserIDs[0] != 8 {
		t.Fatalf("intent=%+v", intent)
	}
	if intent.Vars["url"] != "https://pm.example.test/workspace/documents/11#comment-21" {
		t.Fatal(intent.Vars)
	}
	db := notificationTestDB(t)
	if err := Enqueue(db, "document_comment", 31, intent); err != nil {
		t.Fatal(err)
	}
	var events []auditgorm.OutboxEvent
	if err := db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != EventType(SceneDocumentCommentMentioned) {
		t.Fatalf("events=%+v", events)
	}
	intent.Scene = SceneWorkTaskCommented
	if ValidScope(intent) {
		t.Fatal("workspace-only scope leaked to project scene")
	}
	intent.Scene = SceneDocumentCommentMentioned
	intent.DocumentID = 0
	if ValidScope(intent) {
		t.Fatal("missing document accepted")
	}
}
