package native

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

// The table lock is already held by Execute. Shift tombstones too: restoring a
// deleted row must retain its place relative to rows inserted while it was gone.
// Position is storage order, independent from transient view sort/group rules.
func createSortOrder(tx *gorm.DB, s bitable.Scope, position *bitable.CreatePosition, after map[string]string) (uint64, error) {
	var max uint64
	if err := scopeDB(tx, s).Model(&RowRecord{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&max).Error; err != nil {
		return 0, err
	}
	if max >= bitable.MaxSafeRevision {
		return 0, bitable.ErrBudgetExceeded
	}
	if position == nil {
		return max + 1, nil
	}
	var anchor RowRecord
	if err := scopeDB(tx, s).Where("id = ? AND deleted = ?", position.RecordID, false).First(&anchor).Error; err != nil {
		return 0, dbError(err)
	}
	if position.ExpectedRevision == nil || anchor.Revision != *position.ExpectedRevision {
		return 0, bitable.ErrConflict
	}
	order := anchor.SortOrder
	if position.Direction == "after" {
		if prior := after[position.RecordID]; prior != "" {
			var previous RowRecord
			if err := scopeDB(tx, s).Where("id = ?", prior).First(&previous).Error; err != nil {
				return 0, dbError(err)
			}
			order = previous.SortOrder
		}
		order++
	}
	if err := scopeDB(tx, s).Model(&RowRecord{}).Where("sort_order >= ?", order).UpdateColumn("sort_order", gorm.Expr("sort_order + 1")).Error; err != nil {
		return 0, err
	}
	return order, nil
}
