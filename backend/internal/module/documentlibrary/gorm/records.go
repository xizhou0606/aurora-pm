package documentlibrarygorm

import "time"

type ResourceRecord struct {
	ID          string     `gorm:"primaryKey;size:64;uniqueIndex:ux_document_resources_workspace_id,priority:2"`
	WorkspaceID uint       `gorm:"not null;uniqueIndex:ux_document_resources_workspace_id,priority:1;index:idx_document_resources_workspace_lifecycle,priority:1"`
	TypeKey     string     `gorm:"size:64;not null;index"`
	Name        string     `gorm:"size:220;not null"`
	ParentRef   *string    `gorm:"size:128;index"`
	OwnerUserID uint       `gorm:"not null;index"`
	CreatedBy   uint       `gorm:"not null"`
	Revision    uint64     `gorm:"not null;default:1"`
	Lifecycle   string     `gorm:"size:20;not null;default:active;index:idx_document_resources_workspace_lifecycle,priority:2"`
	ArchivedAt  *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"not null"`
	UpdatedAt   time.Time  `gorm:"not null"`
}

func (ResourceRecord) TableName() string { return "document_resources" }

type ContentBindingRecord struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_document_resource_payload,priority:1;index"`
	ResourceID  string    `gorm:"size:64;not null;uniqueIndex:ux_document_resource_binding_resource"`
	TypeKey     string    `gorm:"size:64;not null;uniqueIndex:ux_document_resource_payload,priority:2"`
	PayloadRef  string    `gorm:"size:128;not null;uniqueIndex:ux_document_resource_payload,priority:3"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (ContentBindingRecord) TableName() string { return "document_resource_bindings" }

func MigrationRecords() []any { return []any{&ResourceRecord{}, &ContentBindingRecord{}} }
