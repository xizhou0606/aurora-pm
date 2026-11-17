package requirementgorm

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// MarkCoverDeletedTx owns the durable cover-worker cleanup transition. Both
// legacy HTTP deletion and protocol-neutral Tool sagas reuse this contract.
func MarkCoverDeletedTx(tx *gorm.DB, workspaceID, requirementID uint) error {
	var row RequirementCover
	err := tx.Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Model(&RequirementCover{}).Where("id = ? AND generation = ?", row.ID, row.Generation).Updates(map[string]any{
		"source_hash": "", "generation": row.Generation + 1, "status": "pending", "attempts": 0,
		"next_attempt_at": time.Now().UTC(), "claimed_at": nil, "claimed_by": "", "last_error": "", "generated_at": nil,
	}).Error
}
