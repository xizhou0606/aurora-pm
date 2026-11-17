package bitable

import (
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

var (
	ErrInvalidInput      = errors.New("bitable input is invalid")
	ErrForbidden         = errors.New("bitable operation is forbidden")
	ErrNotFound          = errors.New("bitable resource not found")
	ErrConflict          = errors.New("bitable revision conflict")
	ErrCellLeaseLost     = errors.New("bitable cell lease is no longer valid")
	ErrCellLeaseConflict = errors.New("bitable mutation conflicts with an active cell lease")
	ErrUnavailable       = errors.New("bitable provider unavailable")
	ErrUnknownOutcome    = errors.New("bitable operation outcome is unknown")
	ErrBudgetExceeded    = errors.New("bitable resource budget exceeded")
	ErrRecordFilteredOut = errors.New("bitable record is excluded by the current view")
)

const (
	MaxPageRows        = 200
	MaxCommandRows     = 100
	MaxCommandCells    = 2000
	MaxExportRows      = 10000
	MaxExportCells     = 100000
	MaxTableValueBytes = 32 << 20
)

type Principal struct {
	UserID, WorkspaceID uint
	// ActorFactUserID is forwarded only to the legacy rich-text/requirement
	// compatibility evaluator. Unified owner and subject checks use UserID.
	ActorFactUserID uint
	// LinkAccessSecret is an opaque request credential forwarded only to the
	// canonical resource-access evaluator. It is never a subject or owner key.
	LinkAccessSecret string
}

// Scope is constructed after PM authorization; it is never bound from JSON.
type Scope struct {
	Principal          Principal
	ResourceID         string
	DocumentResourceID string
	PolicyRevision     uint64
	TableID            string
}

type FieldType string

const (
	FieldText             FieldType = "text"
	FieldLongText         FieldType = "long_text"
	FieldNumber           FieldType = "number"
	FieldCheckbox         FieldType = "checkbox"
	FieldSingleSelect     FieldType = "single_select"
	FieldMultipleSelect   FieldType = "multiple_select"
	FieldDate             FieldType = "date"
	FieldUser             FieldType = "user"
	FieldAttachment       FieldType = "attachment"
	FieldCreatedBy        FieldType = "created_by"
	FieldLastModifiedBy   FieldType = "last_modified_by"
	FieldCreatedTime      FieldType = "created_time"
	FieldLastModifiedTime FieldType = "last_modified_time"
)

type Choice struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}
type FieldOptions struct {
	Choices      []Choice `json:"choices,omitempty"`
	Multiple     bool     `json:"multiple,omitempty"`
	DefaultValue any      `json:"defaultValue,omitempty"`
}
type Field struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     FieldType    `json:"type"`
	Options  FieldOptions `json:"options"`
	ReadOnly bool         `json:"readOnly"`
}
type Record struct {
	ID             string            `json:"id"`
	Revision       uint64            `json:"revision"`
	FieldRevisions map[string]uint64 `json:"fieldRevisions"`
	Values         map[string]any    `json:"values"`
}
type Filter struct {
	FieldID  string `json:"fieldId"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}
type Sort struct {
	FieldID   string `json:"fieldId"`
	Direction string `json:"direction"`
}
type ViewConfig struct {
	HiddenFieldIDs []string       `json:"hiddenFieldIds"`
	ColumnOrder    []string       `json:"columnOrder"`
	ColumnWidths   map[string]int `json:"columnWidths"`
	RowHeight      int            `json:"rowHeight"`
	Filters        []Filter       `json:"filters"`
	Sorts          []Sort         `json:"sorts"`
	GroupFieldID   string         `json:"groupFieldId,omitempty"`
}
type View struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Kind     string     `json:"kind"`
	Revision uint64     `json:"revision"`
	Config   ViewConfig `json:"config"`
}
type Query struct {
	ViewID         string   `json:"viewId"`
	Offset         int      `json:"offset"`
	Limit          int      `json:"limit"`
	Filters        []Filter `json:"filters,omitempty"`
	Sorts          []Sort   `json:"sorts,omitempty"`
	LocateRecordID string   `json:"locateRecordId,omitempty"`
}
type RecordPage struct {
	Records []Record `json:"records"`
	Total   int      `json:"total"`
	Offset  int      `json:"offset"`
}
type Search struct {
	Query
	Text string `json:"text"`
}
type SearchMatch struct {
	RecordID string `json:"recordId"`
	FieldID  string `json:"fieldId"`
	Ordinal  int    `json:"ordinal"`
}
type SearchPage struct {
	Matches []SearchMatch `json:"matches"`
	Total   int           `json:"total"`
}
type Command struct {
	Kind             string           `json:"kind"`
	RecordID         string           `json:"recordId,omitempty"`
	ExpectedRevision *uint64          `json:"expectedRevision,omitempty"`
	Values           map[string]any   `json:"values,omitempty"`
	CellContracts    []CellContract   `json:"cellContracts,omitempty"`
	CellLeaseProofs  []CellLeaseProof `json:"cellLeaseProofs,omitempty"`
	Position         *CreatePosition  `json:"position,omitempty"`
}

// CellLeaseProof is used by row-wide operations whose concurrency authority is
// rowRevision. Unlike CellContract it intentionally has no field base revision.
type CellLeaseProof struct {
	RecordID         string `json:"recordId"`
	FieldID          string `json:"fieldId"`
	LeaseID          string `json:"leaseId"`
	FencingToken     uint64 `json:"fencingToken"`
	ConnectionID     string `json:"connectionId"`
	EditorInstanceID string `json:"editorInstanceId"`
}

// CellContract is the single concurrency contract for an existing cell. The
// scope key is rebuilt from trusted route state; clients provide only the
// record/field target, its authoritative base revision and the opaque lease
// proof returned by the collaboration service.
type CellContract struct {
	RecordID         string `json:"recordId"`
	FieldID          string `json:"fieldId"`
	BaseRevision     uint64 `json:"baseRevision"`
	LeaseID          string `json:"leaseId"`
	FencingToken     uint64 `json:"fencingToken"`
	ConnectionID     string `json:"connectionId"`
	EditorInstanceID string `json:"editorInstanceId"`
}
type CreatePosition struct {
	RecordID         string  `json:"recordId"`
	Direction        string  `json:"direction"`
	ExpectedRevision *uint64 `json:"expectedRevision"`
}
type CommandBatch struct {
	OperationID            string    `json:"operationId"`
	ExpectedSchemaRevision uint64    `json:"expectedSchemaRevision"`
	Commands               []Command `json:"commands"`
	Atomic                 bool      `json:"atomic"`
	Importing              bool      `json:"-"`
}
type Operation struct {
	Moves            []MoveBefore               `json:"moves,omitempty"`
	ID               string                     `json:"operationId"`
	State            string                     `json:"state"`
	Records          []Record                   `json:"records"`
	DeletedRecordIDs []string                   `json:"deletedRecordIds"`
	RequiredActions  []accesscatalog.ActionCode `json:"-"`
}

type CommittedEvent struct {
	WorkspaceID    uint
	ResourceID     string
	TableID        string
	OperationID    string
	ActorUserID    uint
	Type           string
	Payload        json.RawMessage
	PolicyRevision uint64
}

type MoveBefore struct {
	RecordID string         `json:"recordId"`
	Before   CreatePosition `json:"before"`
}
type Capabilities struct {
	CanComment           bool     `json:"canComment"`
	CanRead              bool     `json:"canRead"`
	CanCreateRecords     bool     `json:"canCreateRecords"`
	CanUpdateRecords     bool     `json:"canUpdateRecords"`
	CanDeleteRecords     bool     `json:"canDeleteRecords"`
	CanManageFields      bool     `json:"canManageFields"`
	CanManageViews       bool     `json:"canManageViews"`
	CanImport            bool     `json:"canImport"`
	CanExport            bool     `json:"canExport"`
	CanUploadAttachments bool     `json:"canUploadAttachments"`
	WritableFieldIDs     []string `json:"writableFieldIds"`
}
type Table struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Resource struct {
	Revision           uint64   `json:"revision"`
	OwnerUserID        uint     `json:"ownerUserId"`
	ParentFolderID     *uint    `json:"parentFolderId"`
	Pinned             bool     `json:"pinned"`
	Favorited          bool     `json:"favorited"`
	PreferenceRevision uint64   `json:"preferenceRevision"`
	DirectoryRevision  uint64   `json:"directoryRevision"`
	ID                 string   `json:"id"`
	ResourceID         string   `json:"resource_id"`
	NativeResourceID   string   `json:"-"`
	Name               string   `json:"name"`
	WorkspaceID        uint     `json:"workspaceId"`
	Tables             []Table  `json:"tables"`
	AllowedActions     []string `json:"allowedActions"`
}
type Bootstrap struct {
	Resource       Resource     `json:"resource"`
	TableID        string       `json:"tableId"`
	SchemaRevision uint64       `json:"schemaRevision"`
	Fields         []Field      `json:"fields"`
	Views          []View       `json:"views"`
	Capabilities   Capabilities `json:"capabilities"`
}
type ExportSnapshot struct {
	SnapshotID     string   `json:"snapshotId"`
	Fields         []Field  `json:"fields"`
	Records        []Record `json:"records"`
	SchemaRevision uint64   `json:"schemaRevision"`
}

type ExportQuery struct {
	Query
	All bool `json:"all"`
}
