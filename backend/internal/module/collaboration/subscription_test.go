package collaboration_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"gorm.io/gorm"
)

type accessFake struct {
	mu         sync.Mutex
	calls      int
	allowCalls int
}

func (fake *accessFake) EvaluateCollaborationAccess(
	_ context.Context,
	_ collaboration.Principal,
	_ collaboration.Room,
) (collaboration.AccessDecision, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.calls++
	if fake.allowCalls > 0 && fake.calls > fake.allowCalls {
		return collaboration.AccessDecision{CanView: false}, nil
	}
	return collaboration.AccessDecision{CanView: true, PolicyRevision: 1, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func subscriptionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(collaborationgorm.MigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	return db
}

func appendEventError(db *gorm.DB, room collaboration.Room, operationID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		appender, err := collaborationgorm.NewAppender(tx)
		if err != nil {
			return err
		}
		_, err = appender.Append(context.Background(), collaboration.AppendInput{
			Room: room, OperationID: operationID, ActorUserID: 7, Type: "record.updated",
			Payload: json.RawMessage(`{"id":"r1"}`), PolicyRevision: 1,
		})
		return err
	})
}

func appendEvent(t *testing.T, db *gorm.DB, room collaboration.Room, operationID string) {
	t.Helper()
	if err := appendEventError(db, room, operationID); err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptionClosesReplaySubscribeRaceWithoutDuplicates(t *testing.T) {
	db := subscriptionDB(t)
	reader, _ := collaborationgorm.NewRepository(db)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-stream"}
	appendEvent(t, db, room, "op-1")
	access := &accessFake{}
	service, err := collaboration.NewSubscriptionService(reader, reader, access, collaboration.StreamOptions{
		PollInterval: 5 * time.Millisecond, ReauthorizeInterval: time.Second, ReplayLimit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sequences []uint64
	inserted := false
	done := make(chan error, 1)
	go func() {
		done <- service.Stream(ctx, collaboration.Principal{WorkspaceID: 11, UserID: 7}, room, 0, func(item collaboration.StreamItem) error {
			if item.Event != nil {
				sequences = append(sequences, item.Event.Sequence)
			}
			if item.Kind == "presence" && !inserted {
				inserted = true
				if err := appendEventError(db, room, "op-2"); err != nil {
					return err
				}
			}
			if len(sequences) == 2 {
				cancel()
			}
			return nil
		})
	}()
	select {
	case <-time.After(time.Second):
		t.Fatal("subscription did not observe raced append")
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stream error = %v", err)
		}
	}
	if len(sequences) != 2 || sequences[0] != 1 || sequences[1] != 2 {
		t.Fatalf("sequences = %#v", sequences)
	}
}

func TestSubscriptionPeriodicallyReauthorizesAndClosesOnRevocation(t *testing.T) {
	db := subscriptionDB(t)
	repository, _ := collaborationgorm.NewRepository(db)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-revoked"}
	appendEvent(t, db, room, "op-1")
	access := &accessFake{allowCalls: 1}
	service, _ := collaboration.NewSubscriptionService(repository, repository, access, collaboration.StreamOptions{
		PollInterval: 5 * time.Millisecond, ReauthorizeInterval: 10 * time.Millisecond, ReplayLimit: 10,
	})
	err := service.Stream(context.Background(), collaboration.Principal{WorkspaceID: 11, UserID: 7}, room, 0,
		func(collaboration.StreamItem) error { return nil })
	if !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("stream error = %v", err)
	}
}
