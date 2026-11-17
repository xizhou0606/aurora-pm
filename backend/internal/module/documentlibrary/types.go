package documentlibrary

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput         = errors.New("document resource input is invalid")
	ErrForbidden            = errors.New("document resource operation is forbidden")
	ErrNotFound             = errors.New("document resource not found")
	ErrConflict             = errors.New("document resource revision conflict")
	ErrTypeUnavailable      = errors.New("document resource type is unavailable")
	ErrOperationUnsupported = errors.New("document resource operation is unsupported")
)

type ResourceID string
type TypeKey string
type Operation string

const (
	TypeRichText TypeKey = "rich_text"
	TypeBitable  TypeKey = "bitable"
)

const (
	OperationView         Operation = "view"
	OperationEdit         Operation = "edit"
	OperationViewAccess   Operation = "view_access"
	OperationManageAccess Operation = "manage_access"
	OperationViewLinks    Operation = "view_links"
	OperationManageLinks  Operation = "manage_links"
	OperationExchangeLink Operation = "exchange_link"
	OperationMove         Operation = "move"
	OperationCopy         Operation = "copy"
	OperationDelete       Operation = "delete"
	OperationExport       Operation = "export"
	OperationAssetRead    Operation = "asset_read"
	OperationAssetWrite   Operation = "asset_write"
	OperationComment      Operation = "comment"
	OperationStructure    Operation = "structure"
	OperationCollaborate  Operation = "collaborate"
	OperationCellLease    Operation = "cell_lease"
)

type Principal struct {
	UserID      uint
	WorkspaceID uint
}

type Resource struct {
	ID          ResourceID `json:"resource_id"`
	WorkspaceID uint       `json:"workspace_id"`
	TypeKey     TypeKey    `json:"type_key"`
	Name        string     `json:"name"`
	ParentRef   *string    `json:"parent_ref,omitempty"`
	OwnerUserID uint       `json:"owner_user_id"`
	CreatedBy   uint       `json:"created_by_user_id"`
	Revision    uint64     `json:"revision"`
	Lifecycle   string     `json:"lifecycle"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

type ContentBinding struct {
	WorkspaceID uint
	ResourceID  ResourceID
	TypeKey     TypeKey
	PayloadRef  string
}

type Capabilities struct {
	Operations map[Operation]bool
}

func (capabilities Capabilities) Supports(operation Operation) bool {
	return capabilities.Operations != nil && capabilities.Operations[operation]
}

type TypeDescriptor struct {
	TypeKey      TypeKey
	Capabilities Capabilities
}
