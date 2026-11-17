package notification

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
)

func TestEnqueueCreatesOneTransactionalEventPerRecipient(t *testing.T) {
	db := notificationTestDB(t)
	intent := Intent{
		WorkspaceID: 1, Scene: SceneWorkTaskAssigned, TeamID: 10, ProjectID: 12,
		ActorUserID: 7, RecipientUserIDs: []uint{8, 8, 0, 9},
		Vars:              map[string]string{"title": "assigned task"},
		FallbackPlainText: "任务已分配：assigned task",
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return Enqueue(tx, "work_task_notification", 42, intent)
	}); err != nil {
		t.Fatal(err)
	}
	var events []auditgorm.OutboxEvent
	if err := db.Order("id ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events=%d want=2", len(events))
	}
	for index, event := range events {
		decoded, err := decodeIntent(event)
		if err != nil {
			t.Fatal(err)
		}
		wantRecipient := uint(8 + index)
		if event.AggregateType != "work_task_notification" || event.AggregateID != 42 ||
			event.EventType != EventType(SceneWorkTaskAssigned) ||
			len(decoded.RecipientUserIDs) != 1 || decoded.RecipientUserIDs[0] != wantRecipient {
			t.Fatalf("event=%+v intent=%+v", event, decoded)
		}
	}

	rollbackErr := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := Enqueue(tx, "work_task_notification", 43, intent); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("rollback err=%v", err)
	}
	var rolledBack int64
	db.Model(&auditgorm.OutboxEvent{}).Where("aggregate_id = ?", 43).Count(&rolledBack)
	var receipts int64
	db.Model(&records.InboxReceipt{}).Count(&receipts)
	if receipts != 2 {
		t.Fatalf("transaction leaked or lost receipts: %d", receipts)
	}
	if rolledBack != 0 {
		t.Fatalf("rolled back events=%d", rolledBack)
	}
}

func notificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func decodeIntent(event auditgorm.OutboxEvent) (Intent, error) {
	var intent Intent
	if err := json.Unmarshal([]byte(event.Payload), &intent); err != nil {
		return intent, err
	}
	return intent, nil
}

func ExampleEventType() {
	fmt.Println(EventType(SceneQualityBugReturned))
	// Output: pm.notification.quality_bug.returned
}

func TestEnqueueRejectsMissingPMOwnedText(t *testing.T) {
	db := notificationTestDB(t)
	err := Enqueue(db, "work_task_notification", 42, Intent{
		WorkspaceID: 1, Scene: SceneWorkTaskAssigned, TeamID: 10, ProjectID: 12,
		RecipientUserIDs: []uint{8}, FallbackPlainText: "  ",
	})
	if err == nil {
		t.Fatal("missing PM-owned text was accepted")
	}
}

func TestEnqueueAllowsIdentityNotificationWithoutTeamOrProject(t *testing.T) {
	db := notificationTestDB(t)
	vars := map[string]string{"user": "Alice", "operator": "Admin", "url": "https://pm.test/reset-password#token=once"}
	err := Enqueue(db, "identity_password_reset", 9, Intent{
		WorkspaceID: 1, Scene: SceneIdentityPasswordResetRequested,
		ActorUserID: 7, RecipientUserIDs: []uint{8}, Vars: vars,
		FallbackPlainText: PlainText(SceneIdentityPasswordResetRequested, vars),
	})
	if err != nil {
		t.Fatal(err)
	}
	var event auditgorm.OutboxEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.EventType != EventType(SceneIdentityPasswordResetRequested) {
		t.Fatalf("event=%+v", event)
	}
}

func TestSelfCreationAndStatusChangesDoNotCreateInboxReceipts(t *testing.T) {
	for _, scene := range []string{SceneQualityBugCreated, SceneQualityBugReviewRequested, SceneQualityBugReturned, SceneWorkTaskStatusChanged} {
		t.Run(scene, func(t *testing.T) {
			db := notificationTestDB(t)
			intent := Intent{WorkspaceID: 1, TeamID: 10, ProjectID: 12, Scene: scene, ActorUserID: 7, RecipientUserIDs: []uint{7, 8}, FallbackPlainText: "changed"}
			if err := Enqueue(db, "notification", 42, intent); err != nil {
				t.Fatal(err)
			}
			var rows []records.InboxReceipt
			if err := db.Find(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].RecipientUserID != 8 {
				t.Fatalf("receipts=%+v", rows)
			}
			var events int64
			db.Model(&auditgorm.OutboxEvent{}).Count(&events)
			if events != 2 {
				t.Fatalf("external delivery changed: %d", events)
			}
		})
	}
}
