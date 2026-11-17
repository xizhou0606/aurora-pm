package collaborationgorm

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CellMutationGuard struct {
	tx     *gorm.DB
	access collaboration.CellLeaseAccessEvaluator
}

type CellLeaseBatchAcquirer struct {
	tx     *gorm.DB
	access collaboration.CellLeaseAccessEvaluator
}

func NewCellMutationGuard(tx *gorm.DB, access collaboration.CellLeaseAccessEvaluator) (*CellMutationGuard, error) {
	if tx == nil || access == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &CellMutationGuard{tx: tx, access: access}, nil
}

func NewCellLeaseBatchAcquirer(tx *gorm.DB, access collaboration.CellLeaseAccessEvaluator) (*CellLeaseBatchAcquirer, error) {
	if tx == nil || access == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &CellLeaseBatchAcquirer{tx: tx, access: access}, nil
}

func validCellKey(key collaboration.CellKey) bool {
	return validRoom(key.Room) && validUUID(key.TableID) && validUUID(key.RecordID) && validUUID(key.FieldID)
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func validLeaseOwner(principal collaboration.Principal, editorInstanceID string) bool {
	name := strings.TrimSpace(principal.DisplayName)
	return principal.WorkspaceID > 0 && principal.UserID > 0 && identifierPattern.MatchString(principal.ConnectionID) &&
		identifierPattern.MatchString(editorInstanceID) && name != "" && utf8.RuneCountInString(name) <= 220
}

func validProof(principal collaboration.Principal, proof collaboration.CellLeaseProof) bool {
	return validCellKey(proof.Key) && principal.WorkspaceID == proof.Key.Room.WorkspaceID && principal.UserID > 0 &&
		proof.FencingToken > 0 && validUUID(proof.LeaseID) && identifierPattern.MatchString(proof.ConnectionID) &&
		identifierPattern.MatchString(proof.EditorInstanceID) && principal.ConnectionID == proof.ConnectionID
}

func validLeaseWindow(policyRevision uint64, accessExpiresAt time.Time, ttl time.Duration) bool {
	return policyRevision > 0 && !accessExpiresAt.IsZero() && ttl > 0 && ttl <= collaboration.DefaultCellLeaseTTL
}

func effectiveLeaseExpiry(now time.Time, ttl time.Duration, accessExpiresAt time.Time) (time.Time, error) {
	expiresAt := now.Add(ttl)
	if accessExpiresAt.Before(expiresAt) {
		expiresAt = accessExpiresAt
	}
	if !expiresAt.After(now) {
		return time.Time{}, collaboration.ErrForbidden
	}
	return expiresAt, nil
}

func (repository *Repository) AcquireCellLease(
	ctx context.Context,
	input collaboration.CellLeaseAcquire,
) (collaboration.CellLease, error) {
	if repository == nil || repository.db == nil || !validCellKey(input.Key) ||
		input.Principal.WorkspaceID != input.Key.Room.WorkspaceID || !validLeaseOwner(input.Principal, input.EditorInstanceID) ||
		!validUUID(input.LeaseID) || !validLeaseWindow(input.PolicyRevision, input.AccessExpiresAt, input.TTL) {
		return collaboration.CellLease{}, collaboration.ErrInvalidInput
	}
	var lease collaboration.CellLease
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		lease, err = acquireCellLeaseTx(tx, input)
		return err
	})
	return lease, err
}

func acquireCellLeaseTx(tx *gorm.DB, input collaboration.CellLeaseAcquire) (collaboration.CellLease, error) {
	createdAt, err := databaseTime(tx)
	if err != nil {
		return collaboration.CellLease{}, err
	}
	record := CellLeaseRecord{
		WorkspaceID: input.Key.Room.WorkspaceID, ResourceID: input.Key.Room.ResourceID,
		TableID: input.Key.TableID, RecordID: input.Key.RecordID, FieldID: input.Key.FieldID, UpdatedAt: createdAt,
	}
	if err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error; err != nil {
		return collaboration.CellLease{}, err
	}
	if err = lockCellLease(tx, input.Key, &record); err != nil {
		return collaboration.CellLease{}, err
	}
	now, err := databaseTime(tx)
	if err != nil {
		return collaboration.CellLease{}, err
	}
	expiresAt, err := effectiveLeaseExpiry(now, input.TTL, input.AccessExpiresAt)
	if err != nil {
		return collaboration.CellLease{}, err
	}
	if activeLease(record, now) {
		lease := toCellLease(record)
		if sameLeaseOwner(record, input.Principal, input.EditorInstanceID) {
			updates := activeLeaseUpdates(input.LeaseID, record.FencingToken, input.Principal, input.EditorInstanceID,
				input.PolicyRevision, expiresAt, now)
			// Preserve the original lease identity on an idempotent acquire.
			updates["lease_id"] = record.LeaseID
			if err = updateCellLease(tx, input.Key, record.FencingToken, updates); err != nil {
				return collaboration.CellLease{}, err
			}
			if err = lockCellLease(tx, input.Key, &record); err != nil {
				return collaboration.CellLease{}, err
			}
			return toCellLease(record), nil
		}
		return lease, collaboration.ErrLeaseHeld
	}
	if record.FencingToken >= math.MaxInt64 {
		return collaboration.CellLease{}, collaboration.ErrConflict
	}
	nextFence := record.FencingToken + 1
	updates := activeLeaseUpdates(input.LeaseID, nextFence, input.Principal, input.EditorInstanceID,
		input.PolicyRevision, expiresAt, now)
	if err = updateCellLease(tx, input.Key, record.FencingToken, updates); err != nil {
		return collaboration.CellLease{}, err
	}
	record.LeaseID = &input.LeaseID
	record.FencingToken = nextFence
	record.OwnerConnectionID = &input.Principal.ConnectionID
	record.OwnerEditorInstanceID = &input.EditorInstanceID
	record.ActorUserID = &input.Principal.UserID
	name := strings.TrimSpace(input.Principal.DisplayName)
	record.ActorDisplayName = &name
	record.PolicyRevision = &input.PolicyRevision
	record.ExpiresAt, record.UpdatedAt = &expiresAt, now
	return toCellLease(record), nil
}

func (acquirer *CellLeaseBatchAcquirer) AcquireCellLeases(
	ctx context.Context,
	principal collaboration.Principal,
	editorInstanceID string,
	keys []collaboration.CellKey,
	ttl time.Duration,
) ([]collaboration.CellLease, error) {
	if acquirer == nil || acquirer.tx == nil || acquirer.access == nil ||
		!validLeaseOwner(principal, editorInstanceID) || ttl <= 0 || ttl > collaboration.DefaultCellLeaseTTL {
		return nil, collaboration.ErrInvalidInput
	}
	ordered, err := collaboration.NormalizeCellKeys(keys)
	if err != nil {
		return nil, err
	}
	inputs := make([]collaboration.CellLeaseAcquire, 0, len(ordered))
	for _, key := range ordered {
		if !validCellKey(key) || key.Room.WorkspaceID != principal.WorkspaceID {
			return nil, collaboration.ErrInvalidInput
		}
		decision, evaluateErr := acquirer.access.EvaluateCellEditAccess(ctx, principal, key)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		if !decision.CanEdit || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(time.Now()) {
			return nil, collaboration.ErrForbidden
		}
		inputs = append(inputs, collaboration.CellLeaseAcquire{
			Principal: principal, Key: key, LeaseID: uuid.NewString(), EditorInstanceID: editorInstanceID,
			PolicyRevision: decision.PolicyRevision, AccessExpiresAt: decision.ExpiresAt, TTL: ttl,
		})
	}
	leases := make([]collaboration.CellLease, 0, len(inputs))
	// With a caller-owned transaction GORM uses a savepoint here. It protects the
	// outer transaction from a partial batch even if the caller handles the error.
	err = acquirer.tx.WithContext(ctx).Transaction(func(savepoint *gorm.DB) error {
		for _, input := range inputs {
			lease, acquireErr := acquireCellLeaseTx(savepoint, input)
			if acquireErr != nil {
				return acquireErr
			}
			leases = append(leases, lease)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return leases, nil
}

func (repository *Repository) RenewCellLease(
	ctx context.Context,
	input collaboration.CellLeaseRenewal,
) (collaboration.CellLease, error) {
	if repository == nil || repository.db == nil || !validProof(input.Principal, input.Proof) ||
		!validLeaseOwner(input.Principal, input.Proof.EditorInstanceID) ||
		!validLeaseWindow(input.PolicyRevision, input.AccessExpiresAt, input.TTL) {
		return collaboration.CellLease{}, collaboration.ErrInvalidInput
	}
	var lease collaboration.CellLease
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record CellLeaseRecord
		if err := lockCellLease(tx, input.Proof.Key, &record); err != nil {
			return err
		}
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		if err = verifyLease(record, input.Principal, input.Proof, now); err != nil {
			return err
		}
		expiresAt, err := effectiveLeaseExpiry(now, input.TTL, input.AccessExpiresAt)
		if err != nil {
			return err
		}
		updates := activeLeaseUpdates(input.Proof.LeaseID, input.Proof.FencingToken, input.Principal,
			input.Proof.EditorInstanceID, input.PolicyRevision, expiresAt, now)
		if err = updateCellLease(tx, input.Proof.Key, record.FencingToken, updates); err != nil {
			return err
		}
		record.PolicyRevision = &input.PolicyRevision
		record.ExpiresAt, record.UpdatedAt = &expiresAt, now
		lease = toCellLease(record)
		return nil
	})
	return lease, err
}

func (repository *Repository) ReleaseCellLease(
	ctx context.Context,
	principal collaboration.Principal,
	proof collaboration.CellLeaseProof,
) error {
	if repository == nil || repository.db == nil || !validProof(principal, proof) {
		return collaboration.ErrInvalidInput
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record CellLeaseRecord
		if err := lockCellLease(tx, proof.Key, &record); err != nil {
			return err
		}
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		if err = verifyLeaseIdentity(record, principal, proof); err != nil {
			return err
		}
		return clearCellLease(tx, proof.Key, record.FencingToken, now)
	})
}

func (repository *Repository) ReleaseConnectionCellLeases(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
) (int64, error) {
	if repository == nil || repository.db == nil || principal.WorkspaceID == 0 || principal.UserID == 0 ||
		principal.WorkspaceID != room.WorkspaceID || !validRoom(room) || !identifierPattern.MatchString(principal.ConnectionID) {
		return 0, collaboration.ErrInvalidInput
	}
	now, err := databaseTime(repository.db.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	result := repository.db.WithContext(ctx).Model(&CellLeaseRecord{}).
		Where(
			"workspace_id = ? AND resource_id = ? AND owner_connection_id = ? AND actor_user_id = ? AND lease_id IS NOT NULL",
			room.WorkspaceID, room.ResourceID, principal.ConnectionID, principal.UserID,
		).
		Updates(clearLeaseUpdates(now))
	return result.RowsAffected, result.Error
}

func (guard *CellMutationGuard) VerifyCellMutation(
	ctx context.Context,
	principal collaboration.Principal,
	proof collaboration.CellLeaseProof,
) (collaboration.CellLease, error) {
	if guard == nil || guard.tx == nil || guard.access == nil || !validProof(principal, proof) {
		return collaboration.CellLease{}, collaboration.ErrInvalidInput
	}
	tx := guard.tx.WithContext(ctx)
	var record CellLeaseRecord
	if err := lockCellLease(tx, proof.Key, &record); err != nil {
		return collaboration.CellLease{}, err
	}
	now, err := databaseTime(tx)
	if err != nil {
		return collaboration.CellLease{}, err
	}
	if err = verifyLease(record, principal, proof, now); err != nil {
		return collaboration.CellLease{}, err
	}
	// This evaluator is expected to be the caller's transaction-bound canonical
	// resource-access adapter. The check intentionally happens for every commit.
	decision, err := guard.access.EvaluateCellEditAccess(ctx, principal, proof.Key)
	if err != nil {
		return collaboration.CellLease{}, err
	}
	if !decision.CanEdit || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(now) {
		return collaboration.CellLease{}, collaboration.ErrForbidden
	}
	if record.PolicyRevision == nil || *record.PolicyRevision != decision.PolicyRevision {
		return collaboration.CellLease{}, collaboration.ErrStalePolicy
	}
	return toCellLease(record), nil
}

func lockCellLease(tx *gorm.DB, key collaboration.CellKey, record *CellLeaseRecord) error {
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND resource_id = ? AND table_id = ? AND record_id = ? AND field_id = ?",
		key.Room.WorkspaceID, key.Room.ResourceID, key.TableID, key.RecordID, key.FieldID,
	).First(record).Error
	return mapError(err)
}

func updateCellLease(tx *gorm.DB, key collaboration.CellKey, expectedFence uint64, updates map[string]any) error {
	result := tx.Model(&CellLeaseRecord{}).Where(
		"workspace_id = ? AND resource_id = ? AND table_id = ? AND record_id = ? AND field_id = ? AND fencing_token = ?",
		key.Room.WorkspaceID, key.Room.ResourceID, key.TableID, key.RecordID, key.FieldID, expectedFence,
	).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return collaboration.ErrStaleLease
	}
	return nil
}

func clearCellLease(tx *gorm.DB, key collaboration.CellKey, expectedFence uint64, now time.Time) error {
	return updateCellLease(tx, key, expectedFence, clearLeaseUpdates(now))
}

func activeLeaseUpdates(
	leaseID string,
	fence uint64,
	principal collaboration.Principal,
	editorInstanceID string,
	policyRevision uint64,
	expiresAt time.Time,
	now time.Time,
) map[string]any {
	return map[string]any{
		"lease_id": leaseID, "fencing_token": fence, "owner_connection_id": principal.ConnectionID,
		"owner_editor_instance_id": editorInstanceID, "actor_user_id": principal.UserID,
		"actor_display_name": strings.TrimSpace(principal.DisplayName), "policy_revision": policyRevision,
		"expires_at": expiresAt, "updated_at": now,
	}
}

func clearLeaseUpdates(now time.Time) map[string]any {
	return map[string]any{
		"lease_id": nil, "owner_connection_id": nil, "owner_editor_instance_id": nil,
		"actor_user_id": nil, "actor_display_name": nil, "policy_revision": nil,
		"expires_at": nil, "updated_at": now,
	}
}

func activeLease(record CellLeaseRecord, now time.Time) bool {
	return record.LeaseID != nil && record.ExpiresAt != nil && record.ExpiresAt.After(now)
}

func sameLeaseOwner(record CellLeaseRecord, principal collaboration.Principal, editorInstanceID string) bool {
	return record.OwnerConnectionID != nil && record.OwnerEditorInstanceID != nil && record.ActorUserID != nil &&
		*record.OwnerConnectionID == principal.ConnectionID && *record.OwnerEditorInstanceID == editorInstanceID &&
		*record.ActorUserID == principal.UserID
}

func verifyLeaseIdentity(record CellLeaseRecord, principal collaboration.Principal, proof collaboration.CellLeaseProof) error {
	if record.LeaseID == nil || record.OwnerConnectionID == nil || record.OwnerEditorInstanceID == nil || record.ActorUserID == nil ||
		*record.LeaseID != proof.LeaseID || record.FencingToken != proof.FencingToken ||
		*record.OwnerConnectionID != proof.ConnectionID || *record.OwnerEditorInstanceID != proof.EditorInstanceID ||
		*record.ActorUserID != principal.UserID {
		return collaboration.ErrStaleLease
	}
	return nil
}

func verifyLease(
	record CellLeaseRecord,
	principal collaboration.Principal,
	proof collaboration.CellLeaseProof,
	now time.Time,
) error {
	if err := verifyLeaseIdentity(record, principal, proof); err != nil {
		return err
	}
	if record.ExpiresAt == nil || !record.ExpiresAt.After(now) {
		return collaboration.ErrLeaseExpired
	}
	return nil
}

func toCellLease(record CellLeaseRecord) collaboration.CellLease {
	lease := collaboration.CellLease{
		Key: collaboration.CellKey{
			Room:    collaboration.Room{WorkspaceID: record.WorkspaceID, ResourceID: record.ResourceID},
			TableID: record.TableID, RecordID: record.RecordID, FieldID: record.FieldID,
		},
		FencingToken: record.FencingToken, UpdatedAt: record.UpdatedAt,
	}
	if record.LeaseID != nil {
		lease.LeaseID = *record.LeaseID
	}
	if record.OwnerConnectionID != nil {
		lease.ConnectionID = *record.OwnerConnectionID
	}
	if record.OwnerEditorInstanceID != nil {
		lease.EditorInstanceID = *record.OwnerEditorInstanceID
	}
	if record.ActorUserID != nil {
		lease.ActorUserID = *record.ActorUserID
	}
	if record.ActorDisplayName != nil {
		lease.ActorName = *record.ActorDisplayName
	}
	if record.PolicyRevision != nil {
		lease.PolicyRevision = *record.PolicyRevision
	}
	if record.ExpiresAt != nil {
		lease.ExpiresAt = *record.ExpiresAt
	}
	return lease
}

var _ collaboration.CellLeaseStore = (*Repository)(nil)
var _ collaboration.CellMutationGuard = (*CellMutationGuard)(nil)
var _ collaboration.CellLeaseBatchAcquirer = (*CellLeaseBatchAcquirer)(nil)
