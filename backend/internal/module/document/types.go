package document

import (
	"encoding/json"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

const (
	DefaultSpaceName = "文档"
	SchemaVersionV1  = 1
	SchemaVersionV2  = 2
	SchemaVersionV3  = 3
)

type NodeType string

const (
	NodeFolder   NodeType = "folder"
	NodeDocument NodeType = "document"
	NodeShortcut NodeType = "shortcut"
)

type NodeSystemKind string

const (
	NodeSystemNone              NodeSystemKind = ""
	NodeSystemFavoritesFolder   NodeSystemKind = "favorites_folder"
	NodeSystemFavoriteReference NodeSystemKind = "favorite_reference"
)

type BindingKind string

const BindingPrimaryProductDoc BindingKind = "primary_product_doc"

type Space struct {
	CollaborationEnabled bool      `json:"collaboration_enabled"`
	ID                   uint      `json:"id"`
	WorkspaceID          uint      `json:"workspace_id"`
	Name                 string    `json:"name"`
	IsDefault            bool      `json:"is_default"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type Node struct {
	Creator              *identity.UserDisplay `json:"creator,omitempty"`
	DirectoryOwnerUserID *uint                 `json:"-"`
	DirectoryRevision    uint64                `json:"directory_revision,omitempty"`
	CanMove              bool                  `json:"can_move"`
	ID                   uint                  `json:"id"`
	ResourceID           string                `json:"resource_id,omitempty"`
	WorkspaceID          uint                  `json:"workspace_id"`
	SpaceID              uint                  `json:"space_id"`
	ParentID             *uint                 `json:"parent_id,omitempty"`
	Type                 NodeType              `json:"type"`
	Title                string                `json:"title"`
	SortOrder            int64                 `json:"sort_order"`
	DocumentID           *uint                 `json:"document_id,omitempty"`
	ShortcutTargetNodeID *uint                 `json:"shortcut_target_node_id,omitempty"`
	SystemManaged        bool                  `json:"system_managed"`
	SystemKind           NodeSystemKind        `json:"system_kind,omitempty"`
	SystemOwnerUserID    *uint                 `json:"-"`
	PinnedAt             *time.Time            `json:"pinned_at,omitempty"`
	Favorited            bool                  `json:"favorited"`
	CreatedBy            uint                  `json:"created_by_user_id"`
	UpdatedBy            uint                  `json:"updated_by_user_id"`
	ArchivedAt           *time.Time            `json:"archived_at,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

type Document struct {
	ID                     uint            `json:"id"`
	ResourceID             string          `json:"resource_id,omitempty"`
	WorkspaceID            uint            `json:"workspace_id"`
	NodeID                 uint            `json:"node_id"`
	Content                json.RawMessage `json:"content"`
	SchemaVersion          uint            `json:"schema_version"`
	Revision               uint            `json:"revision"`
	ContentHash            string          `json:"content_hash"`
	PlainText              string          `json:"plain_text"`
	OwnerUserID            uint            `json:"owner_user_id"`
	CreatedBy              uint            `json:"created_by_user_id"`
	UpdatedBy              uint            `json:"updated_by_user_id"`
	CollaborationCutoverAt *time.Time      `json:"collaboration_cutover_at,omitempty"`
	ArchivedAt             *time.Time      `json:"archived_at,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	AllowedActions         []string        `json:"allowed_actions"`
}

type Binding struct {
	ID            uint        `json:"id"`
	WorkspaceID   uint        `json:"workspace_id"`
	DocumentID    uint        `json:"document_id"`
	RequirementID uint        `json:"requirement_id"`
	Kind          BindingKind `json:"kind"`
	CreatedBy     uint        `json:"created_by_user_id"`
	ArchivedAt    *time.Time  `json:"archived_at,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
}

type Asset struct {
	ID           uint       `json:"id"`
	WorkspaceID  uint       `json:"workspace_id"`
	DocumentID   uint       `json:"document_id"`
	ObjectKey    string     `json:"-"`
	FileName     string     `json:"file_name"`
	ContentType  string     `json:"content_type"`
	SizeBytes    int64      `json:"size_bytes"`
	ContentHash  string     `json:"content_hash"`
	CreatedBy    uint       `json:"created_by_user_id"`
	ArchivedAt   *time.Time `json:"archived_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	IsReference  bool       `json:"-"`
	RetainObject bool       `json:"-"`
}

type Principal struct {
	UserID            uint
	ParticipantUserID uint
	WorkspaceID       uint
	LinkAccessSecret  string
}

func (principal Principal) requirementParticipantID() uint {
	if principal.ParticipantUserID > 0 {
		return principal.ParticipantUserID
	}
	return principal.UserID
}

type Tree struct {
	Space Space  `json:"space"`
	Nodes []Node `json:"nodes"`
}

type CreateNodeInput struct {
	directoryOwner *uint
	Collaborative  bool
	ParentID       *uint
	Type           NodeType
	Title          string
}

type CreateNodeResult struct {
	Node     Node      `json:"node"`
	Document *Document `json:"document,omitempty"`
}

type MoveNodeInput struct {
	BeforeNodeID              *uint
	AfterNodeID               *uint
	ExpectedDirectoryRevision *uint64
	ParentID                  *uint
	SortOrder                 *int64
}

type ArchiveImpact struct {
	NodeID               uint  `json:"node_id"`
	DescendantCount      int64 `json:"descendant_count"`
	DocumentCount        int64 `json:"document_count"`
	RequiresConfirmation bool  `json:"requires_confirmation"`
}

type ArchiveConfirmation struct {
	ConfirmNonEmpty         bool
	ExpectedDescendantCount *int64
	ExpectedDocumentCount   *int64
}

type BindingPreview struct {
	RequirementID    uint   `json:"requirement_id"`
	WorkspaceID      uint   `json:"workspace_id"`
	Version          uint   `json:"version"`
	EditorUserIDs    []uint `json:"editor_user_ids"`
	CommenterUserIDs []uint `json:"commenter_user_ids"`
}
