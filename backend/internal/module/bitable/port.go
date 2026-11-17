package bitable

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

type Authorizer interface {
	RequireAction(context.Context, Principal, accesscatalog.ActionCode) error
}
type ResourceStore interface {
	Get(context.Context, uint, string) (Resource, error)
	List(context.Context, uint) ([]Resource, error)
}

// ResourceCreator commits the resource, initial table/view and operation receipt
// in one native transaction. The service owns input and permission decisions.
type ResourceCreator interface {
	Create(context.Context, Principal, string, string) (Resource, error)
}

// ResourceAccessGate is the bitable-owned application port for the unified
// document resource resolver and policy. Route IDs are authoritative unified
// resource IDs; NativeResourceID is only the private bitable payload binding.
type ResourceAccessGate interface {
	RequireBitableResource(context.Context, Principal, documentlibrary.ResourceID, documentlibrary.Operation) (ResourceBinding, error)
	FilterBitableResources(context.Context, Principal, []string, documentlibrary.Operation) (map[string]ResourceBinding, error)
	AuthorizeBitableResource(context.Context, Principal, ResourceBinding, documentlibrary.Operation) error
	BatchAuthorizeBitableResources(context.Context, Principal, []ResourceBinding, []documentlibrary.Operation) (map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool, error)
	RequireBitableCreate(context.Context, Principal) error
}

type ResourceBinding struct {
	ResourceID       documentlibrary.ResourceID
	NativeResourceID string
	OwnerUserID      uint
	// PolicyRevision is the fresh revision used by the authorization decision
	// that admitted this request. Realtime-enabled mutations reject zero.
	PolicyRevision uint64
}

// PrivateResourceCoordinator must create the unified resource, private policy,
// native bitable payload and receipt in one caller-owned transaction. A service
// without this coordinator fails closed; post-commit policy repair is forbidden.
type PrivateResourceCoordinator interface {
	CreatePrivateBitable(context.Context, Principal, string, string) (Resource, ResourceBinding, error)
	DuplicatePrivateBitable(context.Context, Principal, ResourceBinding, ResourceMutation) (Resource, ResourceBinding, error)
	MutateBitableResource(context.Context, Principal, ResourceBinding, string, ResourceMutation) (Resource, ResourceBinding, error)
}

// DataProvider owns the authoritative records and view storage.
type DataProvider interface {
	Bootstrap(context.Context, Scope) (Bootstrap, error)
	Query(context.Context, Scope, Query) (RecordPage, error)
	Search(context.Context, Scope, Search) (SearchPage, error)
	Execute(context.Context, Scope, CommandBatch) (Operation, error)
	Operation(context.Context, Scope, string) (Operation, error)
	Export(context.Context, Scope, ExportQuery) (ExportSnapshot, error)
}

// CellMutationProof is built from a trusted Scope plus a client CellContract.
// It deliberately does not reuse collaboration domain types, keeping bitable's
// persistence boundary independent from collaboration GORM records.
type CellMutationProof struct {
	WorkspaceID      uint
	ResourceID       string
	TableID          string
	RecordID         string
	FieldID          string
	LeaseID          string
	FencingToken     uint64
	ConnectionID     string
	EditorInstanceID string
}

type CellMutationGuard interface {
	VerifyCellMutation(context.Context, Principal, CellMutationProof) error
}

// CellLeaseConflictScope is a trusted Cartesian target range. The adapter must
// materialize a durable placeholder when necessary, then lock/check every
// RecordIDs × FieldIDs cell in stable order using the caller's mutation
// transaction. Locks stay held through commit so a lease cannot be acquired
// after the check. Any valid lease is a conflict.
type CellLeaseConflictScope struct {
	WorkspaceID uint
	ResourceID  string
	TableID     string
	RecordIDs   []string
	FieldIDs    []string
}

type CellLeaseConflictGuard interface {
	RequireNoConflictingCellLeases(context.Context, Principal, []CellLeaseConflictScope) error
}

type CommittedEventAppender interface {
	AppendBitableCommittedEvent(context.Context, CommittedEvent) error
}

// Execute must atomically validate the schema revision, all row revisions, and
// all values, then commit the entire batch and its idempotency result together.
// Same operation ID and identical content replays its result. Reusing an ID for
// different content conflicts. Transport failures do not authorize a retry.
// Export returns one bounded, consistent snapshot, never stitched read pages.
