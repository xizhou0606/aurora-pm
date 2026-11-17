package native

import (
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

func moveSortOrder(tx *gorm.DB, s bitable.Scope, row *RowRecord, target *bitable.CreatePosition, after map[string]string) (bitable.CreatePosition, error) {
	before := bitable.CreatePosition{}
	if target == nil || target.RecordID == row.ID {
		return before, bitable.ErrInvalidInput
	}
	var neighbor RowRecord
	e := scopeDB(tx, s).Where("deleted = ? AND id <> ? AND sort_order < ?", false, row.ID, row.SortOrder).Order("sort_order DESC,id DESC").First(&neighbor).Error
	direction := "after"
	if errors.Is(e, gorm.ErrRecordNotFound) {
		direction = "before"
		e = scopeDB(tx, s).Where("deleted = ? AND id <> ? AND sort_order > ?", false, row.ID, row.SortOrder).Order("sort_order,id").First(&neighbor).Error
	}
	if e != nil {
		return before, dbError(e)
	}
	revision := neighbor.Revision
	before = bitable.CreatePosition{RecordID: neighbor.ID, Direction: direction, ExpectedRevision: &revision}
	order, e := createSortOrder(tx, s, target, after)
	if e != nil {
		return before, e
	}
	row.SortOrder = order
	if target.Direction == "after" {
		after[target.RecordID] = row.ID
	}
	return before, nil
}
