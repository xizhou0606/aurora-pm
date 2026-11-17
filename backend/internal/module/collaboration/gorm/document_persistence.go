package collaborationgorm

import (
	"context"
	"errors"
	"reflect"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DocumentTransaction struct {
	tx          *gorm.DB
	commitGuard collaboration.DocumentCommitGuard
}

func NewDocumentTransaction(tx *gorm.DB) (*DocumentTransaction, error) {
	if tx == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &DocumentTransaction{tx: tx}, nil
}

func (repository *Repository) WithinDocumentTransaction(
	ctx context.Context,
	work func(collaboration.DocumentTransactionStore) error,
) error {
	if repository == nil || repository.db == nil || work == nil {
		return collaboration.ErrInvalidInput
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store, err := NewDocumentTransaction(tx)
		if err != nil {
			return err
		}
		if repository.documentCommitGuardFactory != nil {
			store.commitGuard, err = repository.documentCommitGuardFactory(tx)
			if err != nil || store.commitGuard == nil {
				if err != nil {
					return err
				}
				return collaboration.ErrForbidden
			}
		}
		return work(store)
	})
}

func validateSeed(input collaboration.DocumentSeed) error {
	if !validRoom(input.Room) || collaboration.ValidateDocumentSeed(input) != nil {
		return collaboration.ErrInvalidInput
	}
	return nil
}

func validateDocumentUpdate(input collaboration.DocumentUpdate) error {
	if !validRoom(input.Room) || collaboration.ValidateDocumentUpdate(input) != nil {
		return collaboration.ErrInvalidInput
	}
	return nil
}

func (transaction *DocumentTransaction) SeedDocument(
	ctx context.Context,
	input collaboration.DocumentSeed,
) (collaboration.DocumentState, error) {
	if transaction == nil || transaction.tx == nil || validateSeed(input) != nil {
		return collaboration.DocumentState{}, collaboration.ErrInvalidInput
	}
	tx := transaction.tx.WithContext(ctx)
	now, err := databaseTime(tx)
	if err != nil {
		return collaboration.DocumentState{}, err
	}
	candidate := DocumentStateRecord{
		WorkspaceID: input.Room.WorkspaceID, ResourceID: input.Room.ResourceID,
		SourceRevision: input.SourceRevision, SourceHash: input.SourceHash, DurableRevision: 1,
		Snapshot: cloneBytes(input.Snapshot), SnapshotHash: input.SnapshotHash,
		StateVector: cloneBytes(input.StateVector), StateVectorHash: input.StateVectorHash,
		Projection: cloneBytes(input.Projection), ProjectionHash: input.ProjectionHash,
		PolicyRevision: input.PolicyRevision, SeededAt: now, UpdatedAt: now,
	}
	if err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return collaboration.DocumentState{}, err
	}
	var stored DocumentStateRecord
	if err = lockDocumentState(tx, input.Room, &stored); err != nil {
		return collaboration.DocumentState{}, err
	}
	if !sameSeed(stored, input) {
		return collaboration.DocumentState{}, collaboration.ErrConflict
	}
	return documentState(stored), nil
}

func (transaction *DocumentTransaction) CommitDocumentUpdate(
	ctx context.Context,
	principal collaboration.Principal,
	input collaboration.DocumentUpdate,
) (collaboration.DocumentCommit, error) {
	if transaction == nil || transaction.tx == nil || validateDocumentUpdate(input) != nil {
		return collaboration.DocumentCommit{}, collaboration.ErrInvalidInput
	}
	tx := transaction.tx.WithContext(ctx)
	if transaction.commitGuard != nil {
		if err := transaction.commitGuard.VerifyDocumentCommit(ctx, principal, input); err != nil {
			return collaboration.DocumentCommit{}, err
		}
	}
	var state DocumentStateRecord
	if err := lockDocumentState(tx, input.Room, &state); err != nil {
		return collaboration.DocumentCommit{}, err
	}
	if existing, found, err := findDocumentOperation(tx, input.Room, input.OperationID); err != nil {
		return collaboration.DocumentCommit{}, err
	} else if found {
		if !sameDocumentOperation(existing, input) {
			return collaboration.DocumentCommit{}, collaboration.ErrConflict
		}
		return documentCommit(existing), nil
	}
	if state.DurableRevision != input.ExpectedDurableRevision || state.StateVectorHash != input.BaseStateVectorHash {
		return collaboration.DocumentCommit{}, collaboration.ErrConflict
	}
	if state.SnapshotHash == input.SnapshotHash && state.StateVectorHash == input.StateVectorHash {
		return collaboration.DocumentCommit{}, collaboration.ErrInvalidInput
	}
	now, err := databaseTime(tx)
	if err != nil {
		return collaboration.DocumentCommit{}, err
	}
	record := DocumentUpdateRecord{
		WorkspaceID: input.Room.WorkspaceID, ResourceID: input.Room.ResourceID,
		DurableRevision: state.DurableRevision + 1, BaseDurableRevision: state.DurableRevision,
		OperationID: input.OperationID, ActorUserID: input.ActorUserID,
		Update: cloneBytes(input.Update), UpdateHash: input.UpdateHash, BaseStateVectorHash: input.BaseStateVectorHash,
		ResultStateVectorHash: input.StateVectorHash, ResultSnapshotHash: input.SnapshotHash,
		ResultProjectionHash: input.ProjectionHash, PolicyRevision: input.PolicyRevision, CommittedAt: now,
	}
	if err = tx.Create(&record).Error; err != nil {
		return collaboration.DocumentCommit{}, err
	}
	result := tx.Model(&DocumentStateRecord{}).
		Where("workspace_id = ? AND resource_id = ? AND durable_revision = ? AND state_vector_hash = ?",
			input.Room.WorkspaceID, input.Room.ResourceID, state.DurableRevision, input.BaseStateVectorHash).
		Updates(map[string]any{
			"durable_revision": record.DurableRevision,
			"snapshot":         cloneBytes(input.Snapshot), "snapshot_hash": input.SnapshotHash,
			"state_vector": cloneBytes(input.StateVector), "state_vector_hash": input.StateVectorHash,
			"projection": cloneBytes(input.Projection), "projection_hash": input.ProjectionHash,
			"policy_revision": input.PolicyRevision, "updated_at": now,
		})
	if result.Error != nil {
		return collaboration.DocumentCommit{}, result.Error
	}
	if result.RowsAffected != 1 {
		return collaboration.DocumentCommit{}, collaboration.ErrConflict
	}
	return documentCommit(record), nil
}

func (repository *Repository) LoadDocumentState(ctx context.Context, room collaboration.Room) (collaboration.DocumentState, error) {
	if repository == nil || repository.db == nil || !validRoom(room) {
		return collaboration.DocumentState{}, collaboration.ErrInvalidInput
	}
	var record DocumentStateRecord
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND resource_id = ?", room.WorkspaceID, room.ResourceID).
		First(&record).Error; err != nil {
		return collaboration.DocumentState{}, mapError(err)
	}
	if !validStoredDocumentState(record) {
		return collaboration.DocumentState{}, collaboration.ErrDataCorrupt
	}
	return documentState(record), nil
}

func lockDocumentState(tx *gorm.DB, room collaboration.Room, record *DocumentStateRecord) error {
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND resource_id = ?", room.WorkspaceID, room.ResourceID).First(record).Error
	return mapError(err)
}

func findDocumentOperation(
	tx *gorm.DB,
	room collaboration.Room,
	operationID string,
) (DocumentUpdateRecord, bool, error) {
	var record DocumentUpdateRecord
	err := tx.Where("workspace_id = ? AND resource_id = ? AND operation_id = ?", room.WorkspaceID, room.ResourceID, operationID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, false, nil
	}
	return record, err == nil, err
}

func sameSeed(record DocumentStateRecord, input collaboration.DocumentSeed) bool {
	// Yjs client IDs make separately generated binary seeds differ even when the
	// canonical source JSON is identical. The immutable source revision/hash is
	// the idempotency key; replay returns the first authoritative binary seed.
	return record.SourceRevision == input.SourceRevision && record.SourceHash == input.SourceHash &&
		record.ProjectionHash == input.ProjectionHash
}

func sameDocumentOperation(record DocumentUpdateRecord, input collaboration.DocumentUpdate) bool {
	return record.BaseDurableRevision == input.ExpectedDurableRevision && record.ActorUserID == input.ActorUserID &&
		record.UpdateHash == input.UpdateHash && record.BaseStateVectorHash == input.BaseStateVectorHash &&
		record.ResultStateVectorHash == input.StateVectorHash && record.ResultSnapshotHash == input.SnapshotHash &&
		record.ResultProjectionHash == input.ProjectionHash && reflect.DeepEqual(record.Update, input.Update)
}

func validStoredDocumentState(record DocumentStateRecord) bool {
	projectionHash, err := collaboration.CanonicalJSONHash(record.Projection)
	return err == nil && record.SourceRevision > 0 && record.DurableRevision > 0 &&
		collaboration.IsSHA256Hex(record.SourceHash) && record.PolicyRevision > 0 &&
		record.SnapshotHash == collaboration.SHA256Hex(record.Snapshot) &&
		record.StateVectorHash == collaboration.SHA256Hex(record.StateVector) && record.ProjectionHash == projectionHash
}

func documentState(record DocumentStateRecord) collaboration.DocumentState {
	return collaboration.DocumentState{
		Room:           collaboration.Room{WorkspaceID: record.WorkspaceID, ResourceID: record.ResourceID},
		SourceRevision: record.SourceRevision, SourceHash: record.SourceHash, DurableRevision: record.DurableRevision,
		Snapshot: cloneBytes(record.Snapshot), SnapshotHash: record.SnapshotHash,
		StateVector: cloneBytes(record.StateVector), StateVectorHash: record.StateVectorHash,
		Projection: cloneBytes(record.Projection), ProjectionHash: record.ProjectionHash,
		PolicyRevision: record.PolicyRevision, SeededAt: record.SeededAt, UpdatedAt: record.UpdatedAt,
	}
}

func documentCommit(record DocumentUpdateRecord) collaboration.DocumentCommit {
	return collaboration.DocumentCommit{
		Room:        collaboration.Room{WorkspaceID: record.WorkspaceID, ResourceID: record.ResourceID},
		OperationID: record.OperationID, DurableRevision: record.DurableRevision,
		SnapshotHash: record.ResultSnapshotHash, StateVectorHash: record.ResultStateVectorHash,
		ProjectionHash: record.ResultProjectionHash, PolicyRevision: record.PolicyRevision, CommittedAt: record.CommittedAt,
	}
}

func cloneBytes(value []byte) []byte { return append([]byte(nil), value...) }

var _ collaboration.DocumentTransactionStore = (*DocumentTransaction)(nil)
var _ collaboration.DocumentTransactionRunner = (*Repository)(nil)
var _ collaboration.DocumentStateReader = (*Repository)(nil)
