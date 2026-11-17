package outbox

import (
	"context"
	"encoding/json"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"testing"
)

func TestDecodeDocumentMentionWithoutInventingProjectScope(t *testing.T) {
	intent := notification.DocumentCommentMention(7, 5, 11, 21, []uint{8}, "https://pm.example.test")
	raw, _ := json.Marshal(intent)
	event := auditgorm.OutboxEvent{WorkspaceID: 7, EventType: notification.EventType(intent.Scene), Payload: string(raw)}
	if _, err := decodeIntent(event); err != nil {
		t.Fatal(err)
	}
	event.EventType = notification.EventType(notification.SceneWorkTaskCommented)
	if _, err := decodeIntent(event); err == nil {
		t.Fatal("scene mismatch accepted")
	}
}

func TestDisabledExternalMentionDoesNotResolveOrSend(t *testing.T) {
	intent := notification.DocumentCommentMention(7, 5, 11, 21, []uint{8}, "https://example.test")
	intent.SkipExternal = true
	raw, _ := json.Marshal(intent)
	dispatcher := newIMDispatcher(nil, nil, nil)
	outcome, err := dispatcher.Dispatch(context.Background(), auditgorm.OutboxEvent{WorkspaceID: 7, EventType: notification.EventType(intent.Scene), Payload: string(raw)})
	if err != nil || !outcome.Skipped {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}
