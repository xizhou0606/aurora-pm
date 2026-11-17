package collaborationgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CellLeaseConflictGuard locks every addressed lease row in the caller-owned
// mutation transaction. Empty placeholder rows are intentional: they close the
// gap where a concurrent lease acquire could otherwise insert after a
// delete/structure mutation had checked for existing leases.
type CellLeaseConflictGuard struct{ tx *gorm.DB }

func NewCellLeaseConflictGuard(tx *gorm.DB) (*CellLeaseConflictGuard, error) {
	if tx == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &CellLeaseConflictGuard{tx: tx}, nil
}

func (guard *CellLeaseConflictGuard) RequireNoActiveCellLeases(
	ctx context.Context,
	keys []collaboration.CellKey,
) error {
	if guard == nil || guard.tx == nil {
		return collaboration.ErrInvalidInput
	}
	ordered, err := collaboration.NormalizeCellKeys(keys)
	if err != nil {
		return err
	}
	tx := guard.tx.WithContext(ctx)
	for _, key := range ordered {
		if !validCellKey(key) {
			return collaboration.ErrInvalidInput
		}
		createdAt, timeErr := databaseTime(tx)
		if timeErr != nil {
			return timeErr
		}
		placeholder := CellLeaseRecord{
			WorkspaceID: key.Room.WorkspaceID,
			ResourceID:  key.Room.ResourceID,
			TableID:     key.TableID,
			RecordID:    key.RecordID,
			FieldID:     key.FieldID,
			UpdatedAt:   createdAt,
		}
		if err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&placeholder).Error; err != nil {
			return err
		}
		var record CellLeaseRecord
		if err = lockCellLease(tx, key, &record); err != nil {
			return err
		}
		now, timeErr := databaseTime(tx)
		if timeErr != nil {
			return timeErr
		}
		if activeLease(record, now) {
			return collaboration.ErrLeaseHeld
		}
	}
	return nil
}

var _ interface {
	RequireNoActiveCellLeases(context.Context, []collaboration.CellKey) error
} = (*CellLeaseConflictGuard)(nil)
