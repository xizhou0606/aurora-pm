package collaborationgorm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(MigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	return db
}

func appendInTransaction(t *testing.T, db *gorm.DB, input collaboration.AppendInput) (collaboration.Event, error) {
	t.Helper()
	var event collaboration.Event
	err := db.Transaction(func(tx *gorm.DB) error {
		appender, err := NewAppender(tx)
		if err != nil {
			return err
		}
		event, err = appender.Append(context.Background(), input)
		return err
	})
	return event, err
}

func eventInput(room collaboration.Room, operationID string) collaboration.AppendInput {
	return collaboration.AppendInput{
		Room: room, OperationID: operationID, ActorUserID: 7, Type: "record.updated",
		Payload: json.RawMessage(`{"record_id":"r1"}`), PolicyRevision: 3,
	}
}

func TestAppendIsTransactionalMonotonicAndIdempotentAcrossRepositories(t *testing.T) {
	db := testDB(t)
	first, _ := NewRepository(db)
	second, _ := NewRepository(db)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-1"}

	event1, err := appendInTransaction(t, db, eventInput(room, "op-1"))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := appendInTransaction(t, db, eventInput(room, "op-1"))
	if err != nil || replayed.Sequence != event1.Sequence {
		t.Fatalf("idempotent replay = %#v, %v", replayed, err)
	}
	event2, err := appendInTransaction(t, db, eventInput(room, "op-2"))
	if err != nil || event2.Sequence != 2 {
		t.Fatalf("second event = %#v, %v", event2, err)
	}
	if event1.CommittedAt.IsZero() || event2.CommittedAt.Before(event1.CommittedAt) {
		t.Fatalf("commit timestamps are not sequence ordered: %s then %s", event1.CommittedAt, event2.CommittedAt)
	}
	page, err := second.Replay(context.Background(), room, 0, 10)
	if err != nil || len(page.Events) != 2 || page.Events[0].Sequence != 1 || page.Events[1].Sequence != 2 {
		t.Fatalf("cross repository replay = %#v, %v", page, err)
	}
	if _, err := first.Replay(context.Background(), room, 2, 10); err != nil {
		t.Fatal(err)
	}

	conflict := eventInput(room, "op-1")
	conflict.Payload = json.RawMessage(`{"record_id":"different"}`)
	if _, err := appendInTransaction(t, db, conflict); !errors.Is(err, collaboration.ErrConflict) {
		t.Fatalf("operation binding error = %v", err)
	}
}

func TestReplayBeforeFirstCommitKeepsInitialSubscriberConnected(t *testing.T) {
	repository, _ := NewRepository(testDB(t))
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-empty"}
	page, err := repository.Replay(context.Background(), room, 0, 10)
	if err != nil || page.ResetRequired || page.HeadSequence != 0 || page.RetainedFromSequence != 1 || len(page.Events) != 0 {
		t.Fatalf("empty replay = %#v, %v", page, err)
	}
	page, err = repository.Replay(context.Background(), room, 3, 10)
	if err != nil || !page.ResetRequired || page.ResetReason != "stream_not_initialized" {
		t.Fatalf("stale cursor replay = %#v, %v", page, err)
	}
}

func TestAppendRollsBackWithOwningMutation(t *testing.T) {
	db := testDB(t)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-rollback"}
	err := db.Transaction(func(tx *gorm.DB) error {
		appender, _ := NewAppender(tx)
		if _, err := appender.Append(context.Background(), eventInput(room, "op-rollback")); err != nil {
			return err
		}
		return errors.New("owning mutation failed")
	})
	if err == nil {
		t.Fatal("expected owning transaction failure")
	}
	var count int64
	if err := db.Model(&CommittedEventRecord{}).Where("resource_id = ?", room.ResourceID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rolled back events = %d, %v", count, err)
	}
}

func TestReplayDetectsRetentionAndUnexpectedGaps(t *testing.T) {
	db := testDB(t)
	repository, _ := NewRepository(db)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-gap"}
	for _, operationID := range []string{"op-1", "op-2", "op-3"} {
		if _, err := appendInTransaction(t, db, eventInput(room, operationID)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Where("workspace_id = ? AND resource_id = ? AND sequence = 2", room.WorkspaceID, room.ResourceID).
		Delete(&CommittedEventRecord{}).Error; err != nil {
		t.Fatal(err)
	}
	page, err := repository.Replay(context.Background(), room, 0, 10)
	if err != nil || !page.ResetRequired || page.ResetReason != "event_gap" {
		t.Fatalf("gap replay = %#v, %v", page, err)
	}

	room.ResourceID = "resource-retention"
	for _, operationID := range []string{"op-a", "op-b", "op-c"} {
		if _, err := appendInTransaction(t, db, eventInput(room, operationID)); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.PruneBefore(context.Background(), room, 3); err != nil {
		t.Fatal(err)
	}
	page, err = repository.Replay(context.Background(), room, 0, 10)
	if err != nil || !page.ResetRequired || page.ResetReason != "cursor_outside_retention" {
		t.Fatalf("retention replay = %#v, %v", page, err)
	}
	page, err = repository.Replay(context.Background(), room, 99, 10)
	if err != nil || !page.ResetRequired || page.ResetReason != "cursor_ahead_of_head" {
		t.Fatalf("future cursor replay = %#v, %v", page, err)
	}
}

func TestPresenceIsCrossRepositoryTTLStateWithServerActor(t *testing.T) {
	db := testDB(t)
	first, _ := NewRepository(db)
	second, _ := NewRepository(db)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-presence"}
	if _, err := appendInTransaction(t, db, eventInput(room, "op-seed")); err != nil {
		t.Fatal(err)
	}
	principal := collaboration.Principal{WorkspaceID: 11, UserID: 9, DisplayName: "Authoritative Alice", ConnectionID: "connection-1"}
	if _, err := first.Heartbeat(context.Background(), principal, room, []byte(`{"selection":{"row":"r1"}}`)); err != nil {
		t.Fatal(err)
	}
	impostor := principal
	impostor.UserID = 10
	impostor.DisplayName = "Mallory"
	if _, err := second.Heartbeat(context.Background(), impostor, room, []byte(`{"selection":null}`)); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("presence connection takeover error=%v", err)
	}
	if err := second.Leave(context.Background(), impostor, room); err != nil {
		t.Fatal(err)
	}
	active, err := second.ListActive(context.Background(), room)
	if err != nil || len(active) != 1 || active[0].ActorUserID != 9 || active[0].ActorName != "Authoritative Alice" {
		t.Fatalf("cross repository presence = %#v, %v", active, err)
	}
	if _, err := first.Heartbeat(context.Background(), principal, room, []byte(`{"user":{"id":"forged"}}`)); !errors.Is(err, collaboration.ErrInvalidInput) {
		t.Fatalf("forged actor error = %v", err)
	}
	if err := second.Leave(context.Background(), principal, room); err != nil {
		t.Fatal(err)
	}
	active, _ = first.ListActive(context.Background(), room)
	if len(active) != 0 {
		t.Fatalf("presence not removed: %#v", active)
	}
	principal.ConnectionID = "connection-expired"
	if _, err := first.Heartbeat(context.Background(), principal, room, []byte(`{"selection":null}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&PresenceRecord{}).Where("connection_id = ?", principal.ConnectionID).
		Update("expires_at", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)).Error; err != nil {
		t.Fatal(err)
	}
	active, err = second.ListActive(context.Background(), room)
	if err != nil || len(active) != 0 {
		t.Fatalf("expired presence visible: %#v, %v", active, err)
	}
	deleted, err := first.DeleteExpired(context.Background())
	if err != nil || deleted != 1 {
		t.Fatalf("expired presence cleanup = %d, %v", deleted, err)
	}
}
