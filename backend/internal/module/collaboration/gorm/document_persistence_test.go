package collaborationgorm

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
)

func syntheticProjection(text string) json.RawMessage {
	return json.RawMessage(`{"content":[{"content":[{"text":` + string(mustJSON(text)) + `,"type":"text"}],"type":"paragraph"}],"type":"doc"}`)
}

func mustJSON(value string) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func syntheticSeed(room collaboration.Room, marker string) collaboration.DocumentSeed {
	projection := syntheticProjection(marker)
	projectionHash, _ := collaboration.CanonicalJSONHash(projection)
	snapshot := []byte("synthetic-yjs-snapshot-" + marker)
	vector := []byte("synthetic-yjs-vector-" + marker)
	return collaboration.DocumentSeed{
		Room: room, SourceRevision: 7, SourceHash: projectionHash,
		Snapshot: snapshot, SnapshotHash: collaboration.SHA256Hex(snapshot),
		StateVector: vector, StateVectorHash: collaboration.SHA256Hex(vector),
		Projection: projection, ProjectionHash: projectionHash, ActorUserID: 7, PolicyRevision: 3,
	}
}

func syntheticUpdate(state collaboration.DocumentState, operationID, marker string) collaboration.DocumentUpdate {
	projection := syntheticProjection(marker)
	projectionHash, _ := collaboration.CanonicalJSONHash(projection)
	update := []byte("synthetic-yjs-update-" + marker)
	snapshot := []byte("synthetic-yjs-snapshot-" + marker)
	vector := []byte("synthetic-yjs-vector-" + marker)
	return collaboration.DocumentUpdate{
		Room: state.Room, OperationID: operationID, ExpectedDurableRevision: state.DurableRevision,
		BaseStateVectorHash: state.StateVectorHash, Update: update, UpdateHash: collaboration.SHA256Hex(update),
		Snapshot: snapshot, SnapshotHash: collaboration.SHA256Hex(snapshot),
		StateVector: vector, StateVectorHash: collaboration.SHA256Hex(vector),
		Projection: projection, ProjectionHash: projectionHash, ActorUserID: 7, PolicyRevision: 3,
	}
}

func seedInTransaction(t *testing.T, db *gorm.DB, input collaboration.DocumentSeed) (collaboration.DocumentState, error) {
	t.Helper()
	var state collaboration.DocumentState
	err := db.Transaction(func(tx *gorm.DB) error {
		store, err := NewDocumentTransaction(tx)
		if err != nil {
			return err
		}
		state, err = store.SeedDocument(context.Background(), input)
		return err
	})
	return state, err
}

func commitInTransaction(t *testing.T, db *gorm.DB, input collaboration.DocumentUpdate) (collaboration.DocumentCommit, error) {
	t.Helper()
	var commit collaboration.DocumentCommit
	err := db.Transaction(func(tx *gorm.DB) error {
		store, err := NewDocumentTransaction(tx)
		if err != nil {
			return err
		}
		commit, err = store.CommitDocumentUpdate(context.Background(), collaboration.Principal{WorkspaceID: input.Room.WorkspaceID, UserID: input.ActorUserID}, input)
		return err
	})
	return commit, err
}

func TestDocumentSeedIsSourceIdempotentAndConflictsOnRevisionOrHash(t *testing.T) {
	db := testDB(t)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}
	input := syntheticSeed(room, "seed")
	first, err := seedInTransaction(t, db, input)
	if err != nil || first.DurableRevision != 1 {
		t.Fatalf("first seed = %#v, %v", first, err)
	}

	regenerated := input
	regenerated.Snapshot = []byte("different-yjs-client-id")
	regenerated.SnapshotHash = collaboration.SHA256Hex(regenerated.Snapshot)
	regenerated.StateVector = []byte("different-state-vector")
	regenerated.StateVectorHash = collaboration.SHA256Hex(regenerated.StateVector)
	replayed, err := seedInTransaction(t, db, regenerated)
	if err != nil || replayed.SnapshotHash != first.SnapshotHash || replayed.StateVectorHash != first.StateVectorHash {
		t.Fatalf("idempotent regenerated seed = %#v, %v", replayed, err)
	}

	changedRevision := input
	changedRevision.SourceRevision++
	if _, err = seedInTransaction(t, db, changedRevision); !errors.Is(err, collaboration.ErrConflict) {
		t.Fatalf("source revision conflict = %v", err)
	}
	changedHash := syntheticSeed(room, "other-source")
	if _, err = seedInTransaction(t, db, changedHash); !errors.Is(err, collaboration.ErrConflict) {
		t.Fatalf("source hash conflict = %v", err)
	}
}

func TestDocumentUpdateIsIdempotentAndChecksRevisionVectorAndPayload(t *testing.T) {
	db := testDB(t)
	repository, _ := NewRepository(db)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}
	seed, err := seedInTransaction(t, db, syntheticSeed(room, "seed"))
	if err != nil {
		t.Fatal(err)
	}
	input := syntheticUpdate(seed, "operation-1", "committed")
	first, err := commitInTransaction(t, db, input)
	if err != nil || first.DurableRevision != 2 {
		t.Fatalf("first update = %#v, %v", first, err)
	}
	replayed, err := commitInTransaction(t, db, input)
	if err != nil || replayed != first {
		t.Fatalf("idempotent update = %#v, %v", replayed, err)
	}
	mismatch := input
	mismatch.Update = []byte("different-update")
	mismatch.UpdateHash = collaboration.SHA256Hex(mismatch.Update)
	if _, err = commitInTransaction(t, db, mismatch); !errors.Is(err, collaboration.ErrConflict) {
		t.Fatalf("operation mismatch = %v", err)
	}

	stale := syntheticUpdate(seed, "operation-2", "stale")
	if _, err = commitInTransaction(t, db, stale); !errors.Is(err, collaboration.ErrConflict) {
		t.Fatalf("stale revision/vector = %v", err)
	}
	loaded, err := repository.LoadDocumentState(context.Background(), room)
	if err != nil || loaded.DurableRevision != 2 || loaded.SnapshotHash != input.SnapshotHash || loaded.ProjectionHash != input.ProjectionHash {
		t.Fatalf("loaded committed state = %#v, %v", loaded, err)
	}

	tooLarge := syntheticUpdate(loaded, "operation-large", "large")
	tooLarge.Update = make([]byte, collaboration.MaxDocumentUpdateBytes+1)
	tooLarge.UpdateHash = collaboration.SHA256Hex(tooLarge.Update)
	if _, err = commitInTransaction(t, db, tooLarge); !errors.Is(err, collaboration.ErrInvalidInput) {
		t.Fatalf("oversized update = %v", err)
	}
	badHash := syntheticUpdate(loaded, "operation-hash", "hash")
	badHash.SnapshotHash = input.SnapshotHash
	if _, err = commitInTransaction(t, db, badHash); !errors.Is(err, collaboration.ErrInvalidInput) {
		t.Fatalf("snapshot hash mismatch = %v", err)
	}
}

func TestDocumentCommitRollsBackAtomicallyAndDiskReopenRecoversLastCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collaboration.sqlite")
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	if err := db.AutoMigrate(&DocumentStateRecord{}, &DocumentUpdateRecord{}); err != nil {
		t.Fatal(err)
	}
	room := collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}
	seed, err := seedInTransaction(t, db, syntheticSeed(room, "seed"))
	if err != nil {
		t.Fatal(err)
	}
	failed := syntheticUpdate(seed, "operation-failed", "must-rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		store, _ := NewDocumentTransaction(tx)
		if _, commitErr := store.CommitDocumentUpdate(context.Background(), collaboration.Principal{WorkspaceID: failed.Room.WorkspaceID, UserID: failed.ActorUserID}, failed); commitErr != nil {
			return commitErr
		}
		return errors.New("synthetic owning failure")
	})
	if err == nil {
		t.Fatal("expected synthetic rollback")
	}
	repository, _ := NewRepository(db)
	state, err := repository.LoadDocumentState(context.Background(), room)
	if err != nil || state.DurableRevision != 1 {
		t.Fatalf("state after rollback = %#v, %v", state, err)
	}
	var updateCount int64
	if err = db.Model(&DocumentUpdateRecord{}).Count(&updateCount).Error; err != nil || updateCount != 0 {
		t.Fatalf("updates after rollback = %d, %v", updateCount, err)
	}
	committedInput := syntheticUpdate(state, "operation-committed", "durable")
	if _, err = commitInTransaction(t, db, committedInput); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	if err = sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := open()
	recoveredRepository, _ := NewRepository(reopened)
	recovered, err := recoveredRepository.LoadDocumentState(context.Background(), room)
	if err != nil || recovered.DurableRevision != 2 || recovered.SnapshotHash != committedInput.SnapshotHash ||
		string(recovered.Projection) != string(committedInput.Projection) {
		t.Fatalf("recovered state = %#v, %v", recovered, err)
	}
	if err = reopened.Model(&DocumentStateRecord{}).Where("resource_id = ?", room.ResourceID).
		Update("snapshot", []byte("corrupt")).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = recoveredRepository.LoadDocumentState(context.Background(), room); !errors.Is(err, collaboration.ErrDataCorrupt) {
		t.Fatalf("corruption detection = %v", err)
	}
}

type documentAccessFake struct{ decision collaboration.AccessDecision }

func (fake documentAccessFake) EvaluateDocumentCollaborationAccess(
	context.Context,
	collaboration.Principal,
	collaboration.Room,
) (collaboration.AccessDecision, error) {
	return fake.decision, nil
}

type projectionValidatorFake struct{ reject bool }

type documentCommitGuardFake struct {
	calls *int
	err   error
}

func (fake documentCommitGuardFake) VerifyDocumentCommit(
	context.Context, collaboration.Principal, collaboration.DocumentUpdate,
) error {
	*fake.calls++
	return fake.err
}

func (fake projectionValidatorFake) ValidateDocumentProjection(
	context.Context,
	collaboration.Principal,
	collaboration.Room,
	[]byte,
) error {
	if fake.reject {
		return collaboration.ErrForbidden
	}
	return nil
}

func TestDocumentServiceUsesGoAccessAndProjectionAuthority(t *testing.T) {
	db := testDB(t)
	repository, _ := NewRepository(db)
	decision := collaboration.AccessDecision{
		CanView: true, CanEdit: true, PolicyRevision: 9, ExpiresAt: time.Now().Add(time.Minute),
	}
	service, err := collaboration.NewDocumentCollaborationService(repository, repository,
		documentAccessFake{decision: decision}, projectionValidatorFake{})
	if err != nil {
		t.Fatal(err)
	}
	principal := collaboration.Principal{WorkspaceID: 11, UserID: 44}
	input := syntheticSeed(collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}, "service")
	state, err := service.Seed(context.Background(), principal, input)
	if err != nil || state.PolicyRevision != 9 {
		t.Fatalf("authority-owned seed = %#v, %v", state, err)
	}
	denied, _ := collaboration.NewDocumentCollaborationService(repository, repository,
		documentAccessFake{decision: collaboration.AccessDecision{CanView: true, ExpiresAt: time.Now().Add(time.Minute), PolicyRevision: 9}},
		projectionValidatorFake{})
	if _, err = denied.Commit(context.Background(), principal, syntheticUpdate(state, "operation-denied", "denied")); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("viewer commit = %v", err)
	}
	rejected, _ := collaboration.NewDocumentCollaborationService(repository, repository,
		documentAccessFake{decision: decision}, projectionValidatorFake{reject: true})
	if _, err = rejected.Commit(context.Background(), principal, syntheticUpdate(state, "operation-ref", "ref")); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("projection authority rejection = %v", err)
	}
}

func TestDocumentCommitPolicyGuardRunsInsideOwningTransaction(t *testing.T) {
	db := testDB(t)
	repository, _ := NewRepository(db)
	calls := 0
	repository.ConfigureDocumentCommitGuardFactory(func(*gorm.DB) (collaboration.DocumentCommitGuard, error) {
		return documentCommitGuardFake{calls: &calls, err: collaboration.ErrStalePolicy}, nil
	})
	decision := collaboration.AccessDecision{
		CanView: true, CanEdit: true, PolicyRevision: 9, ExpiresAt: time.Now().Add(time.Minute),
	}
	service, err := collaboration.NewDocumentCollaborationService(
		repository, repository, documentAccessFake{decision: decision}, projectionValidatorFake{},
	)
	if err != nil {
		t.Fatal(err)
	}
	principal := collaboration.Principal{WorkspaceID: 11, UserID: 44}
	state, err := service.Seed(
		context.Background(), principal,
		syntheticSeed(collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}, "policy-guard"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Commit(
		context.Background(), principal, syntheticUpdate(state, "policy-race", "rejected"),
	); !errors.Is(err, collaboration.ErrStalePolicy) {
		t.Fatalf("commit error=%v", err)
	}
	if calls != 1 {
		t.Fatalf("guard calls=%d", calls)
	}
	loaded, err := repository.LoadDocumentState(context.Background(), state.Room)
	if err != nil || loaded.DurableRevision != 1 {
		t.Fatalf("state=%+v err=%v", loaded, err)
	}
}
