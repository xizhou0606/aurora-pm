package database

import (
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Default rows only. Resource visibility is evaluated later by the document service.
func bootstrapPersonalDocumentSpaces(db *gorm.DB) error {
	if !db.Migrator().HasTable(&documentgorm.PersonalSpace{}) {
		return nil
	}
	var scopes []struct {
		WorkspaceID uint
		OwnerUserID uint
	}
	if err := db.Table("workspace_members AS wm").Select("wm.workspace_id, wm.user_id AS owner_user_id").
		Joins("JOIN users u ON u.id = wm.user_id").Joins("JOIN workspaces w ON w.id = wm.workspace_id").
		Where("wm.enabled = ? AND u.active = ? AND w.enabled = ?", true, true, true).Scan(&scopes).Error; err != nil {
		return err
	}
	rows := make([]documentgorm.PersonalSpace, 0, len(scopes))
	now := time.Now().UTC()
	for _, scope := range scopes {
		rows = append(rows, documentgorm.PersonalSpace{WorkspaceID: scope.WorkspaceID, OwnerUserID: scope.OwnerUserID, Revision: 1, CreatedAt: now, UpdatedAt: now})
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 200).Error
}
