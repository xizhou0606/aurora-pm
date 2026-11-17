package workbenchgorm

import "time"

// AccessMenu is the workbench-owned menu registry. Role grants live in the
// identity module, so this record never embeds role or user references.
type AccessMenu struct {
	ID            uint   `gorm:"primaryKey"`
	Code          string `gorm:"size:96;not null;uniqueIndex:ux_access_menu_code"`
	ParentID      *uint  `gorm:"index"`
	Name          string `gorm:"size:128;not null"`
	MenuType      string `gorm:"size:16;not null"`
	Path          string `gorm:"size:1024"`
	Icon          string `gorm:"size:128"`
	HostKey       string `gorm:"size:16;not null;index"`
	SortOrder     int    `gorm:"not null;default:0"`
	Enabled       bool   `gorm:"not null;default:true;index"`
	SystemManaged bool   `gorm:"not null;default:false"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
