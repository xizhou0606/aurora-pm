package designreviewgorm

import (
	"gorm.io/gorm"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

const maxActiveDeepJobs = 5

func ensureReviewMutable(tx *gorm.DB, review Review) error {
	if review.ChainRootID == nil {
		return designreview.ErrConflict
	}
	var newer int64
	if err := tx.Model(&Review{}).
		Where("workspace_id = ? AND chain_root_id = ? AND round_number > ? AND deleting_at IS NULL AND abandoned_at IS NULL", review.WorkspaceID, *review.ChainRootID, review.RoundNumber).
		Count(&newer).Error; err != nil {
		return err
	}
	if newer > 0 {
		return designreview.ErrConflict
	}
	return nil
}

func ensureDeepJobCapacity(tx *gorm.DB, workspaceID uint, reviewID string) error {
	var active int64
	if err := tx.Model(&DeepJob{}).
		Where("workspace_id = ? AND review_id = ? AND status IN ?", workspaceID, reviewID, []string{"pending", "running"}).
		Count(&active).Error; err != nil {
		return err
	}
	if active >= maxActiveDeepJobs {
		return designreview.ErrConflict
	}
	return nil
}
