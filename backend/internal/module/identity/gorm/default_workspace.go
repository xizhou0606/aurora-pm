package identitygorm

import (
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureUserInDefaultWorkspace grants a newly provisioned user baseline
// workspace access. Callers must invoke it in the same transaction that
// creates the user so a missing or ambiguous default cannot leave an orphaned
// account behind.
func EnsureUserInDefaultWorkspace(tx *gorm.DB, userID uint) error {
	if userID == 0 {
		return identity.ErrDefaultWorkspaceUnavailable
	}
	var defaults []Workspace
	if err := tx.Where("is_default = ? AND enabled = ?", true, true).
		Order("id ASC").Limit(2).Find(&defaults).Error; err != nil {
		return err
	}
	if len(defaults) != 1 {
		return identity.ErrDefaultWorkspaceUnavailable
	}
	membership := WorkspaceMember{
		WorkspaceID: defaults[0].ID,
		UserID:      userID,
		Role:        "member",
		Enabled:     true,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(&membership).Error
}
