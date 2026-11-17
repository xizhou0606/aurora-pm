package documentgorm

import "time"

type Space struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_document_spaces_workspace"`
	Name        string    `gorm:"size:120;not null"`
	IsDefault   bool      `gorm:"not null;default:true"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (Space) TableName() string { return "document_spaces" }

type Node struct {
	DirectoryOwnerUserID *uint  `gorm:"index"`
	ID                   uint   `gorm:"primaryKey"`
	WorkspaceID          uint   `gorm:"not null;index:idx_document_nodes_workspace_space_parent"`
	SpaceID              uint   `gorm:"not null;index:idx_document_nodes_workspace_space_parent"`
	ParentID             *uint  `gorm:"index:idx_document_nodes_workspace_space_parent"`
	Type                 string `gorm:"size:20;not null"`
	Title                string `gorm:"size:220;not null"`
	SortOrder            int64  `gorm:"not null;default:0"`
	ShortcutTargetNodeID *uint  `gorm:"index"`
	SystemManaged        bool   `gorm:"not null;default:false;index"`
	SystemKind           string `gorm:"size:40;not null;default:'';index"`
	SystemOwnerUserID    *uint  `gorm:"index"`
	CreatedBy            uint   `gorm:"not null"`
	UpdatedBy            uint   `gorm:"not null"`
	ArchivedBy           *uint
	ArchivedAt           *time.Time `gorm:"index"`
	CreatedAt            time.Time  `gorm:"not null"`
	UpdatedAt            time.Time  `gorm:"not null"`
}

func (Node) TableName() string { return "document_nodes" }

type Document struct {
	ID                     uint   `gorm:"primaryKey"`
	WorkspaceID            uint   `gorm:"not null;index;uniqueIndex:ux_documents_workspace_node"`
	NodeID                 uint   `gorm:"not null;uniqueIndex:ux_documents_workspace_node"`
	ContentJSON            string `gorm:"column:content_json;type:text;not null"`
	SchemaVersion          uint   `gorm:"not null;default:1"`
	Revision               uint   `gorm:"not null;default:1"`
	ContentHash            string `gorm:"size:64;not null"`
	PlainText              string `gorm:"type:text;not null"`
	OwnerUserID            uint   `gorm:"not null"`
	CollaborationCutoverAt *time.Time
	CreatedBy              uint `gorm:"not null"`
	UpdatedBy              uint `gorm:"not null"`
	ArchivedBy             *uint
	ArchivedAt             *time.Time `gorm:"index"`
	CreatedAt              time.Time  `gorm:"not null"`
	UpdatedAt              time.Time  `gorm:"not null"`
}

func (Document) TableName() string { return "documents" }

type Binding struct {
	ID            uint   `gorm:"primaryKey"`
	WorkspaceID   uint   `gorm:"not null;uniqueIndex:ux_document_primary_requirement,where:archived_at IS NULL"`
	DocumentID    uint   `gorm:"not null;index;index:idx_document_bindings_workspace_document"`
	RequirementID uint   `gorm:"not null;uniqueIndex:ux_document_primary_requirement,where:archived_at IS NULL"`
	Kind          string `gorm:"size:40;not null;uniqueIndex:ux_document_primary_requirement,where:archived_at IS NULL"`
	CreatedBy     uint   `gorm:"not null"`
	ArchivedBy    *uint
	ArchivedAt    *time.Time `gorm:"index"`
	CreatedAt     time.Time  `gorm:"not null"`
}

func (Binding) TableName() string { return "document_bindings" }

type Asset struct {
	ID          uint   `gorm:"primaryKey"`
	WorkspaceID uint   `gorm:"not null;index:idx_document_assets_workspace_document"`
	DocumentID  uint   `gorm:"not null;index:idx_document_assets_workspace_document"`
	ObjectKey   string `gorm:"size:1000;not null;uniqueIndex"`
	FileName    string `gorm:"size:255;not null"`
	ContentType string `gorm:"size:255;not null"`
	SizeBytes   int64  `gorm:"not null"`
	ContentHash string `gorm:"size:64;not null"`
	CreatedBy   uint   `gorm:"not null"`
	ArchivedBy  *uint
	ArchivedAt  *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"not null"`
	UpdatedAt   time.Time  `gorm:"not null"`
}

func (Asset) TableName() string { return "document_assets" }

type AssetReference struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_document_asset_references_scope,priority:1;index"`
	DocumentID  uint      `gorm:"not null;uniqueIndex:ux_document_asset_references_scope,priority:2;index"`
	AssetID     uint      `gorm:"not null;uniqueIndex:ux_document_asset_references_scope,priority:3;index"`
	CreatedBy   uint      `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (AssetReference) TableName() string { return "document_asset_references" }

type UserPreference struct {
	ID          uint       `gorm:"primaryKey"`
	WorkspaceID uint       `gorm:"not null;uniqueIndex:ux_document_user_preferences_scope,priority:1;index"`
	UserID      uint       `gorm:"not null;uniqueIndex:ux_document_user_preferences_scope,priority:2;index"`
	NodeID      uint       `gorm:"not null;uniqueIndex:ux_document_user_preferences_scope,priority:3;index"`
	PinnedAt    *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"not null"`
	UpdatedAt   time.Time  `gorm:"not null"`
}

func (UserPreference) TableName() string { return "document_user_preferences" }

type CommentThread struct {
	ID                uint   `gorm:"primaryKey"`
	WorkspaceID       uint   `gorm:"not null;index:idx_document_comment_threads_scope,priority:1"`
	DocumentID        uint   `gorm:"not null;index:idx_document_comment_threads_scope,priority:2"`
	AnchorKind        string `gorm:"size:40;not null"`
	AnchorPayloadJSON string `gorm:"column:anchor_payload_json;type:text;not null"`
	DocumentRevision  uint   `gorm:"not null;default:0"`
	Status            string `gorm:"size:20;not null;index"`
	CreatedBy         uint   `gorm:"not null;index"`
	ResolvedBy        *uint
	ResolvedAt        *time.Time
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

func (CommentThread) TableName() string { return "document_comment_threads" }

type Comment struct {
	ID              uint   `gorm:"primaryKey"`
	WorkspaceID     uint   `gorm:"not null;index:idx_document_comments_scope,priority:1"`
	DocumentID      uint   `gorm:"not null;index:idx_document_comments_scope,priority:2"`
	ThreadID        uint   `gorm:"not null;index:idx_document_comments_scope,priority:3;index"`
	ParentCommentID *uint  `gorm:"index"`
	AuthorUserID    uint   `gorm:"not null;index"`
	BodyJSON        string `gorm:"column:body_json;type:text;not null"`
	DeletedBy       *uint
	DeletedAt       *time.Time `gorm:"index"`
	CreatedAt       time.Time  `gorm:"not null"`
	UpdatedAt       time.Time  `gorm:"not null"`
}

func (Comment) TableName() string { return "document_comments" }

type CommentMention struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_document_comment_mentions_scope,priority:1;index"`
	CommentID   uint      `gorm:"not null;uniqueIndex:ux_document_comment_mentions_scope,priority:2;index"`
	UserID      uint      `gorm:"not null;uniqueIndex:ux_document_comment_mentions_scope,priority:3;index"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (CommentMention) TableName() string { return "document_comment_mentions" }

func MigrationRecords() []any {
	return []any{
		&PersonalSpace{}, &DirectoryEntry{}, &DirectoryResourceEntry{}, &Space{}, &Node{}, &Document{}, &Binding{}, &Asset{}, &AssetReference{}, &UserPreference{},
		&CommentThread{}, &Comment{}, &CommentMention{}, &ToolMutation{},
	}
}
