package native

import "time"

type ResourceRecord struct {
	Revision       uint64 `gorm:"not null;default:1"`
	OwnerUserID    uint   `gorm:"not null"`
	ParentFolderID *uint
	Deleted        bool      `gorm:"not null;default:false"`
	ID             string    `gorm:"type:uuid;primaryKey"`
	WorkspaceID    uint      `gorm:"not null;index:idx_pm_bitable_resources_workspace"`
	Name           string    `gorm:"type:varchar(200);not null"`
	CreatedBy      uint      `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func (ResourceRecord) TableName() string { return "pm_bitable_resources" }

type TableRecord struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	ResourceID     string    `gorm:"type:uuid;not null;index:idx_pm_bitable_tables_scope,priority:2"`
	WorkspaceID    uint      `gorm:"not null;index:idx_pm_bitable_tables_scope,priority:1"`
	Name           string    `gorm:"type:varchar(200);not null"`
	SchemaRevision uint64    `gorm:"not null"`
	DataRevision   uint64    `gorm:"not null"`
	Fields         string    `gorm:"type:jsonb;not null"`
	Views          string    `gorm:"type:jsonb;not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func (TableRecord) TableName() string { return "pm_bitable_tables" }

type RowRecord struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	TableID     string    `gorm:"type:uuid;not null;index:idx_pm_bitable_records_scope,priority:3"`
	ResourceID  string    `gorm:"type:uuid;not null;index:idx_pm_bitable_records_scope,priority:2"`
	WorkspaceID uint      `gorm:"not null;index:idx_pm_bitable_records_scope,priority:1"`
	Revision    uint64    `gorm:"not null"`
	SortOrder   uint64    `gorm:"not null"`
	Values      string    `gorm:"type:jsonb;not null"`
	Deleted     bool      `gorm:"not null;default:false;index:idx_pm_bitable_records_scope,priority:4"`
	CreatedBy   uint      `gorm:"not null"`
	UpdatedBy   uint      `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (RowRecord) TableName() string { return "pm_bitable_records" }

type ReceiptRecord struct {
	ScopeKey    string    `gorm:"type:varchar(160);primaryKey"`
	OperationID string    `gorm:"type:uuid;primaryKey"`
	WorkspaceID uint      `gorm:"not null"`
	ActorID     uint      `gorm:"not null"`
	ResourceID  string    `gorm:"type:uuid;not null"`
	TableID     string    `gorm:"type:uuid;not null"`
	Kind        string    `gorm:"type:varchar(20);not null"`
	RequestHash string    `gorm:"type:char(64);not null"`
	Result      string    `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (ReceiptRecord) TableName() string { return "pm_bitable_receipts" }

type AssetRecord struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	WorkspaceID uint      `gorm:"not null;index:idx_pm_bitable_assets_scope,priority:1"`
	ResourceID  string    `gorm:"type:uuid;not null;index:idx_pm_bitable_assets_scope,priority:2"`
	TableID     string    `gorm:"type:uuid;not null;index:idx_pm_bitable_assets_scope,priority:3"`
	RecordID    string    `gorm:"type:uuid;not null;index:idx_pm_bitable_assets_scope,priority:4"`
	FieldID     string    `gorm:"type:uuid;not null;index:idx_pm_bitable_assets_scope,priority:5"`
	Metadata    string    `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (AssetRecord) TableName() string { return "pm_bitable_assets" }
func MigrationRecords() []any {
	return []any{&ResourceRecord{}, &TableRecord{}, &RowRecord{}, &ReceiptRecord{}, &AssetRecord{}, &CommentRecord{}, &ResourcePreferenceRecord{}, &ShortcutRecord{}}
}
