package collaborationgorm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
)

type leaseAccessFake struct {
	mu       sync.Mutex
	calls    int
	decision collaboration.AccessDecision
}

func (fake *leaseAccessFake) EvaluateCellEditAccess(
	_ context.Context,
	_ collaboration.Principal,
	_ collaboration.CellKey,
) (collaboration.AccessDecision, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.calls++
	return fake.decision, nil
}

func (fake *leaseAccessFake) setDecision(decision collaboration.AccessDecision) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.decision = decision
}

func allowedLeaseAccess(revision uint64) collaboration.AccessDecision {
	return collaboration.AccessDecision{CanView: true, CanEdit: true, PolicyRevision: revision, ExpiresAt: time.Now().Add(time.Minute)}
}

func cellKey() collaboration.CellKey {
	return collaboration.CellKey{
		Room:    collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()},
		TableID: uuid.NewString(), RecordID: uuid.NewString(), FieldID: uuid.NewString(),
	}
}

func leasePrincipal(user uint, connection, name string) collaboration.Principal {
	return collaboration.Principal{WorkspaceID: 11, UserID: user, ConnectionID: connection, DisplayName: name}
}

func leaseService(t *testing.T, repository *Repository, access *leaseAccessFake) *collaboration.CellLeaseService {
	t.Helper()
	service, err := collaboration.NewCellLeaseService(repository, access, collaboration.DefaultCellLeaseTTL)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func deterministicCellKey(fieldSuffix string) collaboration.CellKey {
	return collaboration.CellKey{
		Room:     collaboration.Room{WorkspaceID: 11, ResourceID: "00000000-0000-0000-0000-000000000010"},
		TableID:  "00000000-0000-0000-0000-000000000020",
		RecordID: "00000000-0000-0000-0000-000000000030",
		FieldID:  "00000000-0000-0000-0000-0000000000" + fieldSuffix,
	}
}

func TestNormalizeCellKeysDeduplicatesAndUsesStableFivePartOrder(t *testing.T) {
	first, second, third := deterministicCellKey("01"), deterministicCellKey("02"), deterministicCellKey("03")
	ordered, err := collaboration.NormalizeCellKeys([]collaboration.CellKey{third, first, second, first})
	if err != nil || len(ordered) != 3 || ordered[0] != first || ordered[1] != second || ordered[2] != third {
		t.Fatalf("normalized keys = %#v, %v", ordered, err)
	}
}

func TestCellLeaseIsExclusivePerCellButIndependentAcrossFieldsAndRepositories(t *testing.T) {
	db := testDB(t)
	firstRepository, _ := NewRepository(db)
	secondRepository, _ := NewRepository(db)
	access := &leaseAccessFake{decision: allowedLeaseAccess(4)}
	first, second := leaseService(t, firstRepository, access), leaseService(t, secondRepository, access)
	alice := leasePrincipal(7, "connection-alice", "Authoritative Alice")
	bob := leasePrincipal(8, "connection-bob", "Authoritative Bob")
	key := cellKey()

	aliceLease, err := first.Acquire(context.Background(), alice, key, "editor-alice")
	if err != nil || aliceLease.ActorUserID != alice.UserID || aliceLease.ActorName != alice.DisplayName || aliceLease.FencingToken != 1 {
		t.Fatalf("alice acquire = %#v, %v", aliceLease, err)
	}
	held, err := second.Acquire(context.Background(), bob, key, "editor-bob")
	if !errors.Is(err, collaboration.ErrLeaseHeld) || held.ActorUserID != alice.UserID {
		t.Fatalf("same-cell conflict = %#v, %v", held, err)
	}
	if _, err = first.Acquire(context.Background(), alice, key, "second-tab"); !errors.Is(err, collaboration.ErrLeaseHeld) {
		t.Fatalf("same session different editor must not share a lease: %v", err)
	}

	otherField := key
	otherField.FieldID = uuid.NewString()
	if _, err = second.Acquire(context.Background(), bob, otherField, "editor-bob"); err != nil {
		t.Fatalf("same-row different-field acquire = %v", err)
	}
}

func TestCellLeaseRenewReleaseExpiryTakeoverAndDisconnectPreserveFencing(t *testing.T) {
	db := testDB(t)
	repository, _ := NewRepository(db)
	access := &leaseAccessFake{decision: allowedLeaseAccess(4)}
	service := leaseService(t, repository, access)
	alice := leasePrincipal(7, "connection-alice", "Alice")
	bob := leasePrincipal(8, "connection-bob", "Bob")
	key := cellKey()

	first, err := service.Acquire(context.Background(), alice, key, "editor-alice")
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := service.Renew(context.Background(), alice, collaboration.ProofFor(first))
	if err != nil || renewed.LeaseID != first.LeaseID || renewed.FencingToken != first.FencingToken {
		t.Fatalf("renew = %#v, %v", renewed, err)
	}
	if err = db.Model(&CellLeaseRecord{}).Where("lease_id = ?", first.LeaseID).
		Update("expires_at", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = service.Renew(context.Background(), alice, collaboration.ProofFor(first)); !errors.Is(err, collaboration.ErrLeaseExpired) {
		t.Fatalf("expired renewal = %v", err)
	}
	second, err := service.Acquire(context.Background(), bob, key, "editor-bob")
	if err != nil || second.FencingToken != first.FencingToken+1 {
		t.Fatalf("expired takeover = %#v, %v", second, err)
	}
	if err = service.Release(context.Background(), alice, collaboration.ProofFor(first)); !errors.Is(err, collaboration.ErrStaleLease) {
		t.Fatalf("old release = %v", err)
	}

	otherField := key
	otherField.FieldID = uuid.NewString()
	other, err := service.Acquire(context.Background(), bob, otherField, "editor-bob")
	if err != nil {
		t.Fatal(err)
	}
	released, err := service.Disconnect(context.Background(), bob, key.Room)
	if err != nil || released != 2 {
		t.Fatalf("disconnect release = %d, %v", released, err)
	}
	reacquired, err := service.Acquire(context.Background(), alice, otherField, "editor-alice")
	if err != nil || reacquired.FencingToken != other.FencingToken+1 {
		t.Fatalf("post-disconnect fencing = %#v, %v", reacquired, err)
	}
}

func TestMutationGuardReauthorizesInsideOwningTransaction(t *testing.T) {
	db := testDB(t)
	repository, _ := NewRepository(db)
	access := &leaseAccessFake{decision: allowedLeaseAccess(4)}
	service := leaseService(t, repository, access)
	alice := leasePrincipal(7, "connection-alice", "Alice")
	lease, err := service.Acquire(context.Background(), alice, cellKey(), "editor-alice")
	if err != nil {
		t.Fatal(err)
	}
	verify := func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			guard, err := NewCellMutationGuard(tx, access)
			if err != nil {
				return err
			}
			_, err = guard.VerifyCellMutation(context.Background(), alice, collaboration.ProofFor(lease))
			return err
		})
	}
	if err = verify(); err != nil {
		t.Fatalf("valid mutation guard = %v", err)
	}
	access.setDecision(allowedLeaseAccess(5))
	if err = verify(); !errors.Is(err, collaboration.ErrStalePolicy) {
		t.Fatalf("changed policy revision = %v", err)
	}
	lease, err = service.Renew(context.Background(), alice, collaboration.ProofFor(lease))
	if err != nil || lease.PolicyRevision != 5 {
		t.Fatalf("renew onto fresh policy = %#v, %v", lease, err)
	}
	if err = verify(); err != nil {
		t.Fatalf("guard after policy-aware renewal = %v", err)
	}
	denied := allowedLeaseAccess(4)
	denied.CanEdit = false
	access.setDecision(denied)
	if err = verify(); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("revoked edit = %v", err)
	}
}

func TestBatchAcquireUsesCallerTransactionAndRollsBackPartialConflict(t *testing.T) {
	db := testDB(t)
	repository, _ := NewRepository(db)
	access := &leaseAccessFake{decision: allowedLeaseAccess(4)}
	service := leaseService(t, repository, access)
	alice := leasePrincipal(7, "connection-alice", "Alice")
	bob := leasePrincipal(8, "connection-bob", "Bob")
	first, held, third := deterministicCellKey("01"), deterministicCellKey("02"), deterministicCellKey("03")
	heldLease, err := service.Acquire(context.Background(), bob, held, "editor-bob")
	if err != nil {
		t.Fatal(err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		acquirer, createErr := NewCellLeaseBatchAcquirer(tx, access)
		if createErr != nil {
			return createErr
		}
		_, batchErr := service.AcquireBatch(context.Background(), acquirer, alice, "editor-alice",
			[]collaboration.CellKey{third, first, first, held})
		if !errors.Is(batchErr, collaboration.ErrLeaseHeld) {
			return errors.New("expected held conflict")
		}
		// Deliberately commit the outer transaction: the batch savepoint must still
		// have removed the lease acquired before the conflict.
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var aliceActive int64
	if err = db.Model(&CellLeaseRecord{}).Where("owner_connection_id = ? AND lease_id IS NOT NULL", alice.ConnectionID).
		Count(&aliceActive).Error; err != nil || aliceActive != 0 {
		t.Fatalf("partial batch survived conflict: %d, %v", aliceActive, err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		acquirer, createErr := NewCellLeaseBatchAcquirer(tx, access)
		if createErr != nil {
			return createErr
		}
		leases, batchErr := service.AcquireBatch(context.Background(), acquirer, alice, "editor-alice",
			[]collaboration.CellKey{third, first, first})
		if batchErr != nil {
			return batchErr
		}
		if len(leases) != 2 || leases[0].Key != first || leases[1].Key != third {
			return errors.New("batch output was not normalized")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	wrongConnection := collaboration.ProofFor(heldLease)
	wrongConnection.ConnectionID = "wrong-connection"
	if err = service.Release(context.Background(), bob, wrongConnection); !errors.Is(err, collaboration.ErrInvalidInput) {
		t.Fatalf("wrong connection proof = %v", err)
	}
	if err = service.Release(context.Background(), bob, collaboration.ProofFor(heldLease)); err != nil {
		t.Fatal(err)
	}
	newLease, err := service.Acquire(context.Background(), alice, held, "editor-alice")
	if err != nil {
		t.Fatal(err)
	}
	oldFence := collaboration.ProofFor(newLease)
	oldFence.FencingToken--
	if err = service.Release(context.Background(), alice, oldFence); !errors.Is(err, collaboration.ErrStaleLease) {
		t.Fatalf("old fence proof = %v", err)
	}
}
