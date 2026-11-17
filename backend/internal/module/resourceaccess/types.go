package resourceaccess

import (
	"errors"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type Mode string
type Level string
type LinkDuration string

const (
	ModePrivate    Mode = "private"
	ModeRestricted Mode = "restricted"
	ModeWorkspace  Mode = "workspace"
	ModeLink       Mode = "link"
	ModeLegacy     Mode = "legacy"
)

const (
	LevelViewer Level = "viewer"
	LevelEditor Level = "editor"
)

const (
	LinkDuration24Hours LinkDuration = "24h"
	LinkDuration7Days   LinkDuration = "7d"
	LinkDurationNever   LinkDuration = "never"
)

var (
	ErrInvalidInput        = errors.New("resource access input is invalid")
	ErrNotFound            = errors.New("resource access not found")
	ErrForbidden           = errors.New("resource access is forbidden")
	ErrConflict            = errors.New("resource access changed concurrently")
	ErrIdempotencyConflict = errors.New("resource access idempotency key was reused")
	ErrUnavailable         = errors.New("resource access dependency is unavailable")
	ErrLinkInvalid         = errors.New("document share link is invalid or expired")
	ErrRateLimited         = errors.New("document share link exchange is rate limited")
)

type Principal struct {
	WorkspaceID uint
	UserID      uint
	// LinkAccessSecret is a short-lived, in-memory bearer context minted by a
	// successful link exchange. Transports must never accept it from a URL or
	// persist it in logs, cookies, or server-side identity records.
	LinkAccessSecret string
	// ActorFactUserID is a compatibility-only participant key for the legacy
	// rich-text evaluator. Owner, subject, link and global Action checks must
	// continue to use the authoritative PM UserID above.
	ActorFactUserID uint
}

type Rule struct {
	SubjectKind        identity.SubjectKind `json:"subject_kind"`
	SubjectID          string               `json:"subject_id"`
	Level              Level                `json:"level"`
	IncludeDescendants bool                 `json:"include_descendants,omitempty"`
}

func (rule Rule) Subject() identity.SubjectRef {
	return identity.SubjectRef{
		Kind: rule.SubjectKind, ID: rule.SubjectID, IncludeDescendants: rule.IncludeDescendants,
	}
}

type Policy struct {
	WorkspaceID    uint                       `json:"-"`
	ResourceID     documentlibrary.ResourceID `json:"resource_id"`
	Mode           Mode                       `json:"mode"`
	WorkspaceLevel *Level                     `json:"workspace_level,omitempty"`
	Rules          []Rule                     `json:"rules"`
	Revision       uint64                     `json:"policy_revision"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

type PolicyView struct {
	Policy
	CanManageAccess     bool                                                    `json:"can_manage_access"`
	EffectiveLevel      *Level                                                  `json:"effective_level"`
	EffectiveSources    []string                                                `json:"effective_sources"`
	SubjectCapabilities map[identity.SubjectKind]identity.SubjectKindCapability `json:"subject_capabilities"`
}

type ReplacePolicyInput struct {
	Mode                   Mode
	WorkspaceLevel         *Level
	Rules                  []Rule
	ExpectedPolicyRevision uint64
	IdempotencyKey         string
}

type EvaluationRequest struct {
	ResourceID       documentlibrary.ResourceID
	Operation        documentlibrary.Operation
	LinkAccessSecret string
}

type Evaluation struct {
	ResourceID     documentlibrary.ResourceID `json:"resource_id"`
	Operation      documentlibrary.Operation  `json:"operation"`
	Allowed        bool                       `json:"allowed"`
	EffectiveLevel *Level                     `json:"effective_level,omitempty"`
	PolicyRevision uint64                     `json:"policy_revision,omitempty"`
	Source         string                     `json:"source,omitempty"`
	Sources        []string                   `json:"sources,omitempty"`
	ExpiresAt      *time.Time                 `json:"expires_at,omitempty"`
}

type LinkAccessDecision struct {
	Level     Level
	ExpiresAt time.Time
}

type ShareLink struct {
	ID         string                     `json:"id"`
	ResourceID documentlibrary.ResourceID `json:"resource_id"`
	Level      Level                      `json:"level"`
	Version    uint64                     `json:"version"`
	ExpiresAt  *time.Time                 `json:"expires_at,omitempty"`
	RevokedAt  *time.Time                 `json:"revoked_at,omitempty"`
	CreatedBy  uint                       `json:"created_by_user_id"`
	CreatedAt  time.Time                  `json:"created_at"`
	UpdatedAt  time.Time                  `json:"updated_at"`
}

type CreatedShareLink struct {
	ShareLink
	Secret string `json:"secret"`
}

type CreateShareLinkInput struct {
	Level          Level
	Duration       LinkDuration
	IdempotencyKey string
}

type ExchangeResult struct {
	ResourceID   documentlibrary.ResourceID `json:"resource_id"`
	AccessSecret string                     `json:"access_secret"`
	Level        Level                      `json:"level"`
	ExpiresAt    time.Time                  `json:"expires_at"`
}

func (mode Mode) ValidStored() bool {
	switch mode {
	case ModePrivate, ModeRestricted, ModeWorkspace, ModeLink, ModeLegacy:
		return true
	default:
		return false
	}
}

// Links are an independent explicit grant; legacy policies remain fail-closed.
func (mode Mode) AllowsLinks() bool {
	return mode == ModePrivate || mode == ModeRestricted || mode == ModeWorkspace || mode == ModeLink
}

func (level Level) Valid() bool { return level == LevelViewer || level == LevelEditor }

func (duration LinkDuration) Valid() bool {
	return duration == LinkDuration24Hours || duration == LinkDuration7Days || duration == LinkDurationNever
}
