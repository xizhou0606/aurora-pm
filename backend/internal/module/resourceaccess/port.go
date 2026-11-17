package resourceaccess

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

// GlobalOperationAuthorizer supplies the canonical Action decision for one
// registered resource type. Resource policy can only narrow this result.
type GlobalOperationAuthorizer interface {
	AuthorizeDocumentResourceOperation(
		context.Context, Principal, documentlibrary.Resource, documentlibrary.Operation,
	) (bool, error)
}

// LegacyPolicyEvaluator is the explicit compatibility seam for migrated
// resources. It is never used for new explicit policies.
type LegacyPolicyEvaluator interface {
	EvaluateLegacyDocumentResource(
		context.Context, Principal, documentlibrary.Resource, documentlibrary.Operation,
	) (bool, error)
}

type Clock interface{ Now() time.Time }

type LinkExchangeLimiter interface {
	AllowDocumentShareLinkExchange(context.Context, Principal) (bool, error)
}

type PolicyMutation struct {
	Principal        Principal
	ResourceID       documentlibrary.ResourceID
	Mode             Mode
	WorkspaceLevel   *Level
	Rules            []Rule
	ExpectedRevision uint64
	IdempotencyKey   string
	InputDigest      string
	Now              time.Time
}

// PolicyRevisionEvent is appended from the same transaction that advances the
// policy revision. IdempotencyKey is an input to the collaboration adapter's
// stable operation id; it must never be persisted as event payload.
type PolicyRevisionEvent struct {
	WorkspaceID      uint
	ResourceID       documentlibrary.ResourceID
	ActorUserID      uint
	IdempotencyKey   string
	PreviousRevision uint64
	PolicyRevision   uint64
	Mode             Mode
}

// PolicyRevisionEventAppender is transaction-bound. Implementations append to
// the canonical collaboration stream and must not open or commit a transaction.
type PolicyRevisionEventAppender interface {
	AppendPolicyRevisionEvent(context.Context, PolicyRevisionEvent) error
}

type PolicyStore interface {
	GetPolicy(context.Context, uint, documentlibrary.ResourceID) (Policy, error)
	ListPolicies(context.Context, uint, []documentlibrary.ResourceID) (map[documentlibrary.ResourceID]Policy, error)
	ReplacePolicy(context.Context, PolicyMutation) (Policy, error)
	ResolveLinkAccess(
		context.Context, Principal, []documentlibrary.ResourceID, string, time.Time,
	) (map[documentlibrary.ResourceID]LinkAccessDecision, error)
}

type LinkMutation struct {
	Principal      Principal
	ResourceID     documentlibrary.ResourceID
	LinkID         string
	SecretHash     string
	Level          Level
	ExpiresAt      *time.Time
	IdempotencyKey string
	InputDigest    string
	Now            time.Time
}

type LinkStore interface {
	CreateShareLink(context.Context, LinkMutation) (ShareLink, error)
	ListShareLinks(context.Context, uint, documentlibrary.ResourceID) ([]ShareLink, error)
	RevokeShareLink(context.Context, LinkMutation) error
	RotateShareLink(context.Context, LinkMutation) (ShareLink, error)
	FindShareLinkBySecretHash(context.Context, string) (uint, ShareLink, error)
	CreateLinkAccessContext(
		context.Context, Principal, ShareLink, string, string, time.Time, time.Time,
	) error
}
