package aigorm

import "gorm.io/gorm"

// SubjectHasRunsTx reports whether the subject has durable AI execution
// history in the caller's transaction.
func SubjectHasRunsTx(tx *gorm.DB, workspaceID uint, subjectType string, subjectID uint) (bool, error) {
	var count int64
	err := tx.Model(&AIRun{}).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, subjectType, subjectID).
		Count(&count).Error
	return count > 0, err
}
