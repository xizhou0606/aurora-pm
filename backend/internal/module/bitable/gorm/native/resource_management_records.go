package native

import "time"

type ResourcePreferenceRecord struct {
	ResourceID  string    `gorm:"type:uuid;primaryKey"`
	ActorID     uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null"`
	Revision    uint64    `gorm:"not null"`
	Pinned      bool      `gorm:"not null"`
	Favorited   bool      `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (ResourcePreferenceRecord) TableName() string { return "pm_bitable_resource_preferences" }

type ShortcutRecord struct {
	ID               string `gorm:"type:uuid;primaryKey"`
	TargetResourceID string `gorm:"type:uuid;not null;index"`
	WorkspaceID      uint   `gorm:"not null;index"`
	ParentFolderID   *uint
	Revision         uint64    `gorm:"not null"`
	CreatedBy        uint      `gorm:"not null"`
	Deleted          bool      `gorm:"not null;default:false"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

func (ShortcutRecord) TableName() string { return "pm_bitable_shortcuts" }
