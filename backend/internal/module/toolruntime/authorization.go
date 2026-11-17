package toolruntime

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const DefaultAuthorizationRequestTTL = 10 * time.Minute

type AuthorizationDecision string

const (
	AuthorizationPending   AuthorizationDecision = "pending"
	AuthorizationAuthorize AuthorizationDecision = "authorize"
	AuthorizationDeny      AuthorizationDecision = "deny"
)

var (
	ErrAuthorizationRequestInvalid   = errors.New("external authorization request is invalid")
	ErrAuthorizationRequestNotFound  = errors.New("external authorization request not found")
	ErrAuthorizationRequestExpired   = errors.New("external authorization request expired")
	ErrAuthorizationRequestDecided   = errors.New("external authorization request already decided")
	ErrAuthorizationDecisionInvalid  = errors.New("external authorization decision is invalid")
	ErrAuthorizationCatalogInvalid   = errors.New("external authorization catalog is invalid")
	ErrAuthorizationRepositoryAbsent = errors.New("external authorization repository is required")

	authorizationRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32,128}$`)
)

// AuthorizationRequest is a short-lived, single-decision approval fact. Tool
// names are retained for the approval screen; grant scopes are resolved from
// the canonical Catalog and never accepted directly from the browser.
type AuthorizationRequest struct {
	ID                   string
	PrincipalID          uint
	WorkspaceID          uint
	WorkspaceName        string
	ClientID             string
	ClientName           string
	Channel              DeliveryChannel
	ConfigurationMode    WorkspaceConfigurationMode
	ConfigurationVersion uint64
	ToolNames            []string
	GrantScopes          []string
	EffectiveFrom        time.Time
	ExpiresAt            time.Time
	RequestExpiresAt     time.Time
	OAuth                *OAuthAuthorizationContext
	Decision             AuthorizationDecision
	DecidedAt            *time.Time
	GrantID              *uint
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AuthorizationRequestRepository interface {
	Create(context.Context, *AuthorizationRequest) error
	FindByID(context.Context, string) (AuthorizationRequest, error)
	Decide(context.Context, AuthorizationDecisionCommand) (AuthorizationRequest, error)
}

type AuthorizationDecisionCommand struct {
	RequestID   string
	PrincipalID uint
	WorkspaceID uint
	Decision    AuthorizationDecision
	DecidedAt   time.Time
	Grant       *Grant
}

type GrantListRepository interface {
	ListForPrincipalWorkspace(context.Context, uint, uint) ([]Grant, error)
}

func (request AuthorizationRequest) Validate() error {
	if !authorizationRequestIDPattern.MatchString(request.ID) || request.PrincipalID == 0 || request.WorkspaceID == 0 {
		return fmt.Errorf("%w: id, principal, and workspace are required", ErrAuthorizationRequestInvalid)
	}
	if strings.TrimSpace(request.WorkspaceName) == "" || strings.TrimSpace(request.ClientID) == "" || strings.TrimSpace(request.ClientName) == "" {
		return fmt.Errorf("%w: display facts are required", ErrAuthorizationRequestInvalid)
	}
	if _, err := AudienceForChannel(request.Channel); err != nil {
		return fmt.Errorf("%w: channel is unsupported", ErrAuthorizationRequestInvalid)
	}
	if request.ConfigurationVersion == 0 ||
		(request.ConfigurationMode != WorkspaceConfigurationPermanent && request.ConfigurationMode != WorkspaceConfigurationWindow) {
		return fmt.Errorf("%w: configuration snapshot is invalid", ErrAuthorizationRequestInvalid)
	}
	if len(canonicalNames(request.ToolNames)) == 0 || len(canonicalScopes(request.GrantScopes)) == 0 {
		return fmt.Errorf("%w: tools and grant scopes are required", ErrAuthorizationRequestInvalid)
	}
	if request.EffectiveFrom.IsZero() || request.ExpiresAt.IsZero() || !request.EffectiveFrom.Before(request.ExpiresAt) {
		return fmt.Errorf("%w: grant window is invalid", ErrAuthorizationRequestInvalid)
	}
	if request.CreatedAt.IsZero() || request.UpdatedAt.IsZero() || request.RequestExpiresAt.IsZero() || !request.CreatedAt.Before(request.RequestExpiresAt) {
		return fmt.Errorf("%w: request window is invalid", ErrAuthorizationRequestInvalid)
	}
	if request.Decision != AuthorizationPending && request.Decision != AuthorizationAuthorize && request.Decision != AuthorizationDeny {
		return fmt.Errorf("%w: decision is unsupported", ErrAuthorizationRequestInvalid)
	}
	if request.Decision == AuthorizationPending && (request.DecidedAt != nil || request.GrantID != nil) {
		return fmt.Errorf("%w: pending request contains decision facts", ErrAuthorizationRequestInvalid)
	}
	if request.Decision != AuthorizationPending && request.DecidedAt == nil {
		return fmt.Errorf("%w: decided request is missing decided_at", ErrAuthorizationRequestInvalid)
	}
	if request.Decision == AuthorizationAuthorize && request.GrantID == nil {
		return fmt.Errorf("%w: authorized request is missing its grant", ErrAuthorizationRequestInvalid)
	}
	if request.Decision == AuthorizationDeny && request.GrantID != nil {
		return fmt.Errorf("%w: denied request cannot reference a grant", ErrAuthorizationRequestInvalid)
	}
	if request.OAuth != nil {
		if request.Channel != DeliveryMCP && request.Channel != DeliveryCLI {
			return fmt.Errorf("%w: OAuth channel is unsupported", ErrAuthorizationRequestInvalid)
		}
		if err := request.OAuth.Validate(); err != nil {
			return fmt.Errorf("%w: OAuth context is invalid", ErrAuthorizationRequestInvalid)
		}
	}
	return nil
}

func (request AuthorizationRequest) EnsurePending(now time.Time) error {
	if err := request.Validate(); err != nil || now.IsZero() {
		return ErrAuthorizationRequestInvalid
	}
	if request.Decision != AuthorizationPending {
		return ErrAuthorizationRequestDecided
	}
	if !now.UTC().Before(request.RequestExpiresAt.UTC()) {
		return ErrAuthorizationRequestExpired
	}
	return nil
}

func canonicalNames(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}
