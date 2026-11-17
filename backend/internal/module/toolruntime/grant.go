package toolruntime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type GrantStatus string

const (
	GrantScheduled GrantStatus = "scheduled"
	GrantActive    GrantStatus = "active"
	GrantExpired   GrantStatus = "expired"
	GrantRevoked   GrantStatus = "revoked"
)

type TokenAudience string

const (
	AudienceMCP TokenAudience = "pm-tool-mcp"
	AudienceCLI TokenAudience = "pm-tool-cli"
)

var (
	ErrGrantInvalid           = errors.New("external tool grant is invalid")
	ErrGrantDenied            = errors.New("external tool grant denied access")
	ErrGrantIDMismatch        = errors.New("external tool grant id mismatch")
	ErrGrantNotActive         = errors.New("external tool grant is not active")
	ErrGrantAudienceMismatch  = errors.New("external tool grant audience mismatch")
	ErrGrantChannelMismatch   = errors.New("external tool grant channel mismatch")
	ErrGrantWorkspaceMismatch = errors.New("external tool grant workspace mismatch")
	ErrGrantToolDenied        = errors.New("external tool grant tool denied")
	ErrGrantScopeDenied       = errors.New("external tool grant scope denied")
	ErrGrantPrincipalMismatch = errors.New("external tool grant principal mismatch")
	ErrGrantClientMismatch    = errors.New("external tool grant client mismatch")
)

// Grant is an external delegation boundary. It deliberately contains no PM
// role, menu, module, or object-permission facts.
type Grant struct {
	ID            uint
	PrincipalID   uint
	WorkspaceID   uint
	ClientID      string
	Channel       DeliveryChannel
	AllowedTools  []string
	AllowedScopes []string
	EffectiveFrom time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type GrantRequest struct {
	PrincipalID   uint
	WorkspaceID   uint
	ClientID      string
	Channel       DeliveryChannel
	AllowedTools  []string
	AllowedScopes []string
	EffectiveFrom time.Time
	ExpiresAt     time.Time
}

// AccessRequest contains verified request facts presented to the grant gate.
// The caller must still establish identity and legal workspace membership.
type AccessRequest struct {
	GrantID     uint
	PrincipalID uint
	WorkspaceID uint
	ClientID    string
	Channel     DeliveryChannel
	Audience    TokenAudience
	Tool        string
	Scope       string
}

type GrantRepository interface {
	Create(context.Context, *Grant) error
	FindByID(context.Context, uint) (Grant, error)
	Revoke(context.Context, uint, time.Time) (Grant, error)
}

func NewGrant(request GrantRequest, createdAt time.Time) (Grant, error) {
	request, err := normalizeGrantRequest(request)
	if err != nil {
		return Grant{}, err
	}
	if createdAt.IsZero() {
		return Grant{}, fmt.Errorf("%w: created_at is required", ErrGrantInvalid)
	}
	createdAt = createdAt.UTC()
	return Grant{
		PrincipalID: request.PrincipalID, WorkspaceID: request.WorkspaceID,
		ClientID: request.ClientID, Channel: request.Channel,
		AllowedTools: request.AllowedTools, AllowedScopes: request.AllowedScopes, EffectiveFrom: request.EffectiveFrom,
		ExpiresAt: request.ExpiresAt, CreatedAt: createdAt, UpdatedAt: createdAt,
	}, nil
}

func (grant Grant) Validate() error {
	_, err := normalizeGrant(grant)
	return err
}

func (grant Grant) Status(now time.Time) GrantStatus {
	if grant.RevokedAt != nil {
		return GrantRevoked
	}
	if now.Before(grant.EffectiveFrom) {
		return GrantScheduled
	}
	if !now.Before(grant.ExpiresAt) {
		return GrantExpired
	}
	return GrantActive
}

func (grant Grant) Authorize(request AccessRequest, now time.Time) error {
	grant, err := normalizeGrant(grant)
	if err != nil || now.IsZero() {
		return deny(ErrGrantInvalid)
	}
	if grant.ID == 0 || request.GrantID == 0 || request.GrantID != grant.ID {
		return deny(ErrGrantIDMismatch)
	}
	if request.PrincipalID == 0 || request.PrincipalID != grant.PrincipalID {
		return deny(ErrGrantPrincipalMismatch)
	}
	if request.WorkspaceID == 0 || request.WorkspaceID != grant.WorkspaceID {
		return deny(ErrGrantWorkspaceMismatch)
	}
	if request.ClientID == "" || request.ClientID != grant.ClientID {
		return deny(ErrGrantClientMismatch)
	}
	if request.Channel != grant.Channel {
		return deny(ErrGrantChannelMismatch)
	}
	audience, audienceErr := AudienceForChannel(grant.Channel)
	if audienceErr != nil || request.Audience != audience {
		return deny(ErrGrantAudienceMismatch)
	}
	if grant.Status(now.UTC()) != GrantActive {
		return deny(ErrGrantNotActive)
	}
	if !grantAllowsTool(grant.AllowedTools, request.Tool) {
		return deny(ErrGrantToolDenied)
	}
	if !grantAllowsScope(grant.AllowedScopes, request.Scope) {
		return deny(ErrGrantScopeDenied)
	}
	return nil
}

func (grant *Grant) Revoke(at time.Time) error {
	if grant == nil || at.IsZero() {
		return fmt.Errorf("%w: revocation time is required", ErrGrantInvalid)
	}
	if err := grant.Validate(); err != nil {
		return err
	}
	if grant.RevokedAt != nil {
		return nil
	}
	revokedAt := at.UTC()
	if revokedAt.Before(grant.UpdatedAt) {
		return fmt.Errorf("%w: revocation cannot predate the latest grant update", ErrGrantInvalid)
	}
	grant.RevokedAt = &revokedAt
	grant.UpdatedAt = revokedAt
	return nil
}

func AudienceForChannel(channel DeliveryChannel) (TokenAudience, error) {
	switch channel {
	case DeliveryMCP:
		return AudienceMCP, nil
	case DeliveryCLI:
		return AudienceCLI, nil
	default:
		return "", ErrDeliveryInvalid
	}
}

// RequireAuthorizationPage prevents silent grant widening. An inactive grant
// also requires fresh authorization even when the requested facts are equal.
func RequireAuthorizationPage(current *Grant, requested GrantRequest, now time.Time) (bool, error) {
	requested, err := normalizeGrantRequest(requested)
	if err != nil || now.IsZero() {
		return true, fmt.Errorf("%w: invalid authorization request", ErrGrantInvalid)
	}
	if current == nil || current.ID == 0 {
		return true, nil
	}
	grant, err := normalizeGrant(*current)
	if err != nil {
		return true, err
	}
	if grant.Status(now.UTC()) != GrantActive {
		return true, nil
	}
	changed := grant.PrincipalID != requested.PrincipalID ||
		grant.WorkspaceID != requested.WorkspaceID || grant.ClientID != requested.ClientID ||
		grant.Channel != requested.Channel || !grant.EffectiveFrom.Equal(requested.EffectiveFrom) ||
		!grant.ExpiresAt.Equal(requested.ExpiresAt) || !equalTools(grant.AllowedTools, requested.AllowedTools) ||
		!equalScopes(grant.AllowedScopes, requested.AllowedScopes)
	return changed, nil
}

func deny(reason error) error {
	return fmt.Errorf("%w: %w", ErrGrantDenied, reason)
}
