package httpapi

import (
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"gorm.io/gorm"
)

func deleteQualityBugsForRequirement(
	tx *gorm.DB,
	workspaceID uint,
	requirementID uint,
) error {
	return deleteQualityBugsByScope(
		tx, workspaceID, "requirement_id = ?", requirementID,
	)
}

func deleteQualityBugsForProjectSet(
	tx *gorm.DB,
	workspaceID uint,
	projectSetID uint,
) error {
	return deleteQualityBugsByScope(
		tx, workspaceID, "project_set_id = ?", projectSetID,
	)
}

func deleteQualityBugsByScope(
	tx *gorm.DB,
	workspaceID uint,
	condition string,
	value uint,
) error {
	var bugIDs []uint
	if err := tx.Unscoped().Model(&qualitygorm.QualityBug{}).
		Where("workspace_id = ?", workspaceID).
		Where(condition, value).
		Pluck("id", &bugIDs).Error; err != nil {
		return err
	}
	if len(bugIDs) > 0 {
		if err := tx.Where("workspace_id = ? AND quality_bug_id IN ?", workspaceID, bugIDs).
			Delete(&qualitygorm.QualityBugComment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND quality_bug_id IN ?", workspaceID, bugIDs).
			Delete(&qualitygorm.QualityBugActivity{}).Error; err != nil {
			return err
		}
	}
	return tx.Unscoped().Where("workspace_id = ?", workspaceID).
		Where(condition, value).
		Delete(&qualitygorm.QualityBug{}).Error
}
