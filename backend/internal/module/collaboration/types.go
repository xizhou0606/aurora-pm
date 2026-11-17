package collaboration

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalidInput  = errors.New("collaboration input is invalid")
	ErrConflict      = errors.New("collaboration operation conflicts with existing event")
	ErrNotFound      = errors.New("collaboration resource stream not found")
	ErrForbidden     = errors.New("collaboration access is forbidden")
	ErrResetRequired = errors.New("collaboration replay requires a full resync")
	ErrLeaseHeld     = errors.New("collaboration cell lease is held by another editor")
	ErrLeaseExpired  = errors.New("collaboration cell lease has expired")
	ErrStaleLease    = errors.New("collaboration cell lease proof is stale")
	ErrStalePolicy   = errors.New("collaboration cell lease policy revision is stale")
	ErrDataCorrupt   = errors.New("collaboration durable data is corrupt")
)

type Room struct {
	WorkspaceID uint   `json:"workspace_id"`
	ResourceID  string `json:"resource_id"`
}

type Principal struct {
	WorkspaceID  uint
	UserID       uint
	DisplayName  string
	ConnectionID string
	// LinkAccessSecret is the short-lived, resource-scoped bearer context
	// forwarded from the authenticated browser request. It is never persisted
	// as identity or presence state.
	LinkAccessSecret string
}

type Event struct {
	Room           Room            `json:"room"`
	Sequence       uint64          `json:"sequence"`
	OperationID    string          `json:"operation_id"`
	ActorUserID    uint            `json:"actor_user_id"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	PolicyRevision uint64          `json:"policy_revision"`
	CommittedAt    time.Time       `json:"committed_at"`
}

type AppendInput struct {
	Room           Room
	OperationID    string
	ActorUserID    uint
	Type           string
	Payload        json.RawMessage
	PolicyRevision uint64
}

type ReplayPage struct {
	Events               []Event `json:"events"`
	HeadSequence         uint64  `json:"head_sequence"`
	RetainedFromSequence uint64  `json:"retained_from_sequence"`
	ResetRequired        bool    `json:"reset_required"`
	ResetReason          string  `json:"reset_reason,omitempty"`
}

type AccessDecision struct {
	CanView        bool
	CanEdit        bool
	PolicyRevision uint64
	ExpiresAt      time.Time
}

type Presence struct {
	Room         Room            `json:"room"`
	ConnectionID string          `json:"connection_id"`
	ActorUserID  uint            `json:"actor_user_id"`
	ActorName    string          `json:"actor_display_name"`
	State        json.RawMessage `json:"state"`
	ExpiresAt    time.Time       `json:"expires_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
