package collaborationgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

func TestCellLeaseConflictGuardMaterializesAndLocksTargets(t *testing.T) {
	db := testDB(t)
	guard, err := NewCellLeaseConflictGuard(db)
	if err != nil {
		t.Fatal(err)
	}
	key := cellKey()
	if err = guard.RequireNoActiveCellLeases(context.Background(), []collaboration.CellKey{key, key}); err != nil {
		t.Fatalf("guard empty target: %v", err)
	}
	var count int64
	if err = db.Model(&CellLeaseRecord{}).Where(
		"workspace_id = ? AND resource_id = ? AND table_id = ? AND record_id = ? AND field_id = ?",
		key.Room.WorkspaceID, key.Room.ResourceID, key.TableID, key.RecordID, key.FieldID,
	).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("placeholder count=%d err=%v", count, err)
	}
}

func TestCellLeaseConflictGuardRejectsActiveButAllowsExpiredLease(t *testing.T) {
	db := testDB(t)
	key := cellKey()
	now := time.Now().UTC()
	leaseID := "6b5fcf49-a150-4867-904e-d72a0948824b"
	connectionID, editorID, actor, revision := "connection-1", "editor-1", uint(7), uint64(4)
	record := CellLeaseRecord{
		WorkspaceID: key.Room.WorkspaceID, ResourceID: key.Room.ResourceID, TableID: key.TableID,
		RecordID: key.RecordID, FieldID: key.FieldID, LeaseID: &leaseID, FencingToken: 1,
		OwnerConnectionID: &connectionID, OwnerEditorInstanceID: &editorID, ActorUserID: &actor,
		PolicyRevision: &revision, ExpiresAt: ptrTime(now.Add(time.Minute)), UpdatedAt: now,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	guard, _ := NewCellLeaseConflictGuard(db)
	if err := guard.RequireNoActiveCellLeases(context.Background(), []collaboration.CellKey{key}); !errors.Is(err, collaboration.ErrLeaseHeld) {
		t.Fatalf("active lease error=%v", err)
	}
	if err := db.Model(&CellLeaseRecord{}).Where(
		"workspace_id = ? AND resource_id = ? AND table_id = ? AND record_id = ? AND field_id = ?",
		key.Room.WorkspaceID, key.Room.ResourceID, key.TableID, key.RecordID, key.FieldID,
	).Update("expires_at", now.Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := guard.RequireNoActiveCellLeases(context.Background(), []collaboration.CellKey{key}); err != nil {
		t.Fatalf("expired lease error=%v", err)
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
