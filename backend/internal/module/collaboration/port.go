package collaboration

import (
	"context"
	"time"
)

// EventAppender is created with the owning mutation's transaction. Append must
// not open or commit its own transaction.
type EventAppender interface {
	Append(context.Context, AppendInput) (Event, error)
}

type EventReader interface {
	Replay(context.Context, Room, uint64, int) (ReplayPage, error)
}

type AccessEvaluator interface {
	EvaluateCollaborationAccess(context.Context, Principal, Room) (AccessDecision, error)
}

type PresenceStore interface {
	Heartbeat(context.Context, Principal, Room, []byte) (Presence, error)
	ListActive(context.Context, Room) ([]Presence, error)
	Leave(context.Context, Principal, Room) error
	DeleteExpired(context.Context) (int64, error)
}

// CellLeaseStore owns short transactions for lease lifecycle operations. It
// does not authorize callers; CellLeaseService supplies a fresh decision.
type CellLeaseStore interface {
	AcquireCellLease(context.Context, CellLeaseAcquire) (CellLease, error)
	RenewCellLease(context.Context, CellLeaseRenewal) (CellLease, error)
	ReleaseCellLease(context.Context, Principal, CellLeaseProof) error
	ReleaseConnectionCellLeases(context.Context, Principal, Room) (int64, error)
}

// CellLeaseAccessEvaluator is implemented by the canonical resource-access
// adapter. Implementations must return a fresh policy revision, not a UI cache.
type CellLeaseAccessEvaluator interface {
	EvaluateCellEditAccess(context.Context, Principal, CellKey) (AccessDecision, error)
}

// CellMutationGuard is constructed from the caller's mutation transaction.
// A successful verification does not commit; the bitable mutation must use the
// same transaction so the lease row stays locked through commit.
type CellMutationGuard interface {
	VerifyCellMutation(context.Context, Principal, CellLeaseProof) (CellLease, error)
}

// CellLeaseBatchAcquirer is constructed from a caller-owned transaction. The
// implementation normalizes and locks keys in stable order and uses a nested
// savepoint so a conflict cannot leave a partial batch in the outer tx.
type CellLeaseBatchAcquirer interface {
	AcquireCellLeases(context.Context, Principal, string, []CellKey, time.Duration) ([]CellLease, error)
}

type DocumentCollaborationAccessEvaluator interface {
	EvaluateDocumentCollaborationAccess(context.Context, Principal, Room) (AccessDecision, error)
}

// DocumentProjectionValidator is the Go-authority hook for schema-version and
// referenced-resource validation. It must fail closed; Node validation alone is
// not sufficient authority for assets or embedded resources.
type DocumentProjectionValidator interface {
	ValidateDocumentProjection(context.Context, Principal, Room, []byte) error
}

// DocumentCommitGuard is constructed from the same transaction that stores a
// durable document update. It locks and revalidates every authoritative fact
// used by the candidate projection (resource lifecycle, policy revision,
// embedded resources, and document assets) through commit.
type DocumentCommitGuard interface {
	VerifyDocumentCommit(context.Context, Principal, DocumentUpdate) error
}

type DocumentTransactionStore interface {
	SeedDocument(context.Context, DocumentSeed) (DocumentState, error)
	CommitDocumentUpdate(context.Context, Principal, DocumentUpdate) (DocumentCommit, error)
}

type DocumentTransactionRunner interface {
	WithinDocumentTransaction(context.Context, func(DocumentTransactionStore) error) error
}

type DocumentStateReader interface {
	LoadDocumentState(context.Context, Room) (DocumentState, error)
}
