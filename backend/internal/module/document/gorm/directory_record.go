package documentgorm

import "time"

type PersonalSpace struct {
	ID          uint   `gorm:"primaryKey"`
	WorkspaceID uint   `gorm:"not null;uniqueIndex:ux_personal_document_space,priority:1"`
	OwnerUserID uint   `gorm:"not null;uniqueIndex:ux_personal_document_space,priority:2"`
	Revision    uint64 `gorm:"not null;default:1"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PersonalSpace) TableName() string { return "personal_document_spaces" }

type DirectoryEntry struct {
	ID         uint   `gorm:"primaryKey"`
	SpaceID    uint   `gorm:"not null;uniqueIndex:ux_personal_directory_entry,priority:1;index:idx_personal_directory_parent"`
	NodeID     uint   `gorm:"not null;uniqueIndex:ux_personal_directory_entry,priority:2"`
	ResourceID string `gorm:"not null;default:''"`
	ParentID   *uint
	SortOrder  int64  `gorm:"not null"`
	Title      string `gorm:"not null;default:''"`
	Removed    bool   `gorm:"not null;default:false"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (DirectoryEntry) TableName() string { return "personal_document_entries" }

type DirectoryResourceEntry struct {
	ID              uint   `gorm:"primaryKey"`
	SpaceID         uint   `gorm:"not null;uniqueIndex:ux_personal_resource_entry,priority:1"`
	ResourceID      string `gorm:"not null;uniqueIndex:ux_personal_resource_entry,priority:2"`
	ParentID        *uint
	Revision        uint64 `gorm:"not null;default:1"`
	LastOperationID string `gorm:"not null;default:''"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (DirectoryResourceEntry) TableName() string { return "personal_document_resource_entries" }
