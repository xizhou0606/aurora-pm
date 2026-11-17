package toolruntime

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ManagementPrincipal struct {
	PrincipalID   uint
	WorkspaceID   uint
	WorkspaceName string
}

type AuthorizationRequestInput struct {
	Channel       DeliveryChannel
	ToolNames     []string
	EffectiveFrom time.Time
	ExpiresAt     time.Time
}

type GrantSummary struct {
	Grant      Grant
	ClientName string
	ToolNames  []string
	Status     GrantStatus
}

type AuthorizationService struct {
	catalog        *Catalog
	requests       AuthorizationRequestRepository
	grants         GrantListRepository
	configurations WorkspaceConfigurationRepository
	now            func() time.Time
	newID          func() (string, error)
	ttl            time.Duration
}

func NewAuthorizationService(
	catalog *Catalog,
	requests AuthorizationRequestRepository,
	grants GrantListRepository,
	configurations WorkspaceConfigurationRepository,
) (*AuthorizationService, error) {
	if catalog == nil {
		return nil, ErrAuthorizationCatalogInvalid
	}
	if requests == nil || grants == nil || configurations == nil {
		return nil, ErrAuthorizationRepositoryAbsent
	}
	return &AuthorizationService{
		catalog: catalog, requests: requests, grants: grants, configurations: configurations,
		now: time.Now, newID: secureAuthorizationRequestID, ttl: DefaultAuthorizationRequestTTL,
	}, nil
}

func (service *AuthorizationService) Catalog() []Definition {
	// The PM management catalog is an inventory view, not an execution
	// manifest. Administrators need the complete canonical inventory so draft
	// tools remain visible and auditable while execution adapters continue to
	// exclude them through Catalog.Manifest/Application.
	return service.catalog.Definitions()
}

func (service *AuthorizationService) ListGrants(ctx context.Context, principal ManagementPrincipal) ([]GrantSummary, error) {
	if err := validateManagementPrincipal(principal); err != nil {
		return nil, err
	}
	grants, err := service.grants.ListForPrincipalWorkspace(ctx, principal.PrincipalID, principal.WorkspaceID)
	if err != nil {
		return nil, err
	}
	now := service.now().UTC()
	summaries := make([]GrantSummary, 0, len(grants))
	for _, grant := range grants {
		_, clientName, clientErr := managementClient(grant.Channel)
		toolNames, toolErr := service.validateGrantEntitlements(grant)
		if clientErr != nil || toolErr != nil {
			return nil, ErrAuthorizationCatalogInvalid
		}
		summaries = append(summaries, GrantSummary{
			Grant: grant, ClientName: clientName, ToolNames: toolNames, Status: grant.Status(now),
		})
	}
	return summaries, nil
}

func (service *AuthorizationService) Start(ctx context.Context, principal ManagementPrincipal, input AuthorizationRequestInput) (AuthorizationRequest, error) {
	clientID, clientName, err := managementClient(input.Channel)
	if err != nil {
		return AuthorizationRequest{}, ErrAuthorizationRequestInvalid
	}
	return service.start(ctx, principal, input, clientID, clientName, nil)
}

func (service *AuthorizationService) start(
	ctx context.Context,
	principal ManagementPrincipal,
	input AuthorizationRequestInput,
	clientID string,
	clientName string,
	oauth *OAuthAuthorizationContext,
) (AuthorizationRequest, error) {
	if err := validateManagementPrincipal(principal); err != nil {
		return AuthorizationRequest{}, err
	}
	now := service.now().UTC()
	configuration, err := service.configurationForAuthorization(ctx, principal.WorkspaceID, now)
	if err != nil {
		return AuthorizationRequest{}, err
	}
	effectiveFrom, expiresAt, ok := configuration.GrantWindow(input.Channel, now)
	if !ok {
		return AuthorizationRequest{}, ErrWorkspaceConfigurationInvalid
	}
	if len(input.ToolNames) > 0 {
		tools, scopes, resolveErr := service.resolveTools(input.ToolNames)
		if resolveErr != nil || !equalTools(tools, configuration.AllowedTools) || !equalScopes(scopes, configuration.AllowedScopes) {
			return AuthorizationRequest{}, ErrAuthorizationRequestInvalid
		}
	}
	if (!input.EffectiveFrom.IsZero() && !input.EffectiveFrom.UTC().Equal(effectiveFrom)) ||
		(!input.ExpiresAt.IsZero() && !input.ExpiresAt.UTC().Equal(expiresAt)) {
		return AuthorizationRequest{}, ErrAuthorizationRequestInvalid
	}
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientName) == "" {
		return AuthorizationRequest{}, ErrAuthorizationRequestInvalid
	}
	id, err := service.newID()
	if err != nil || !authorizationRequestIDPattern.MatchString(id) {
		return AuthorizationRequest{}, fmt.Errorf("%w: secure id generation failed", ErrAuthorizationRequestInvalid)
	}
	request := AuthorizationRequest{
		ID: id, PrincipalID: principal.PrincipalID, WorkspaceID: principal.WorkspaceID,
		WorkspaceName: strings.TrimSpace(principal.WorkspaceName), ClientID: clientID, ClientName: clientName,
		Channel: input.Channel, ConfigurationMode: configuration.Mode, ConfigurationVersion: configuration.Version,
		ToolNames: configuration.AllowedTools, GrantScopes: configuration.AllowedScopes,
		EffectiveFrom: effectiveFrom, ExpiresAt: expiresAt,
		RequestExpiresAt: now.Add(service.ttl), OAuth: oauth,
		Decision: AuthorizationPending, CreatedAt: now, UpdatedAt: now,
	}
	if err := request.Validate(); err != nil {
		return AuthorizationRequest{}, err
	}
	if err := service.requests.Create(ctx, &request); err != nil {
		return AuthorizationRequest{}, err
	}
	return request, nil
}

func (service *AuthorizationService) Get(ctx context.Context, principal ManagementPrincipal, id string) (AuthorizationRequest, error) {
	request, err := service.boundRequest(ctx, principal, id)
	if err != nil {
		return AuthorizationRequest{}, err
	}
	if err := request.EnsurePending(service.now()); err != nil {
		return AuthorizationRequest{}, err
	}
	return request, nil
}

func (service *AuthorizationService) Decide(ctx context.Context, principal ManagementPrincipal, id string, decision AuthorizationDecision) (AuthorizationRequest, error) {
	if decision != AuthorizationAuthorize && decision != AuthorizationDeny {
		return AuthorizationRequest{}, ErrAuthorizationDecisionInvalid
	}
	request, err := service.boundRequest(ctx, principal, id)
	if err != nil {
		return AuthorizationRequest{}, err
	}
	now := service.now().UTC()
	if err := request.EnsurePending(now); err != nil {
		return AuthorizationRequest{}, err
	}
	var grant *Grant
	if decision == AuthorizationAuthorize {
		configuration, configurationErr := service.configurationForAuthorization(ctx, principal.WorkspaceID, now)
		if configurationErr != nil {
			return AuthorizationRequest{}, configurationErr
		}
		effectiveFrom, expiresAt, windowOK := configuration.GrantWindow(request.Channel, request.CreatedAt)
		if !windowOK || request.ConfigurationMode != configuration.Mode ||
			request.ConfigurationVersion != configuration.Version ||
			!equalTools(request.ToolNames, configuration.AllowedTools) ||
			!equalScopes(request.GrantScopes, configuration.AllowedScopes) ||
			!request.EffectiveFrom.UTC().Equal(effectiveFrom) ||
			!request.ExpiresAt.UTC().Equal(expiresAt) {
			return AuthorizationRequest{}, ErrAuthorizationRequestInvalid
		}
		if !now.Before(request.ExpiresAt.UTC()) {
			return AuthorizationRequest{}, ErrAuthorizationRequestExpired
		}
		value, grantErr := NewGrant(GrantRequest{
			PrincipalID: request.PrincipalID, WorkspaceID: request.WorkspaceID,
			ClientID: request.ClientID, Channel: request.Channel,
			AllowedTools: request.ToolNames, AllowedScopes: request.GrantScopes,
			EffectiveFrom: request.EffectiveFrom, ExpiresAt: request.ExpiresAt,
		}, now)
		if grantErr != nil {
			return AuthorizationRequest{}, grantErr
		}
		grant = &value
	}
	return service.requests.Decide(ctx, AuthorizationDecisionCommand{
		RequestID: request.ID, PrincipalID: principal.PrincipalID, WorkspaceID: principal.WorkspaceID,
		Decision: decision, DecidedAt: now, Grant: grant,
	})
}

func (service *AuthorizationService) DefinitionsFor(request AuthorizationRequest) ([]Definition, error) {
	definitions := make([]Definition, 0, len(request.ToolNames))
	for _, name := range request.ToolNames {
		definition, ok := service.catalog.Find(name)
		if !ok || definition.Lifecycle == LifecycleDraft {
			return nil, ErrAuthorizationCatalogInvalid
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

func (service *AuthorizationService) boundRequest(ctx context.Context, principal ManagementPrincipal, id string) (AuthorizationRequest, error) {
	if err := validateManagementPrincipal(principal); err != nil || !authorizationRequestIDPattern.MatchString(strings.TrimSpace(id)) {
		return AuthorizationRequest{}, ErrAuthorizationRequestNotFound
	}
	request, err := service.requests.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return AuthorizationRequest{}, err
	}
	if request.PrincipalID != principal.PrincipalID || request.WorkspaceID != principal.WorkspaceID {
		return AuthorizationRequest{}, ErrAuthorizationRequestNotFound
	}
	return request, nil
}

func (service *AuthorizationService) resolveTools(names []string) ([]string, []string, error) {
	requested := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, nil, ErrAuthorizationRequestInvalid
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, nil, ErrAuthorizationRequestInvalid
		}
		seen[name] = struct{}{}
		requested = append(requested, name)
	}
	if len(requested) == 0 {
		return nil, nil, ErrAuthorizationRequestInvalid
	}
	sort.Strings(requested)
	scopes := make([]string, 0, len(requested))
	for _, name := range requested {
		definition, ok := service.catalog.Find(name)
		if !ok || definition.Lifecycle == LifecycleDraft {
			return nil, nil, ErrAuthorizationCatalogInvalid
		}
		scopes = append(scopes, definition.Authorization.GrantScope)
	}
	return requested, canonicalScopes(scopes), nil
}

func (service *AuthorizationService) validateGrantEntitlements(grant Grant) ([]string, error) {
	tools := canonicalTools(grant.AllowedTools)
	if len(tools) == 0 {
		return nil, ErrAuthorizationCatalogInvalid
	}
	derivedScopes := make([]string, 0, len(tools))
	for _, tool := range tools {
		definition, ok := service.catalog.Find(tool)
		if !ok || definition.Lifecycle == LifecycleDraft {
			return nil, ErrAuthorizationCatalogInvalid
		}
		derivedScopes = append(derivedScopes, definition.Authorization.GrantScope)
	}
	if !equalScopes(canonicalScopes(derivedScopes), canonicalScopes(grant.AllowedScopes)) {
		return nil, ErrAuthorizationCatalogInvalid
	}
	return tools, nil
}

func validateManagementPrincipal(principal ManagementPrincipal) error {
	if principal.PrincipalID == 0 || principal.WorkspaceID == 0 || strings.TrimSpace(principal.WorkspaceName) == "" {
		return ErrAuthorizationRequestInvalid
	}
	return nil
}

func managementClient(channel DeliveryChannel) (string, string, error) {
	switch channel {
	case DeliveryMCP:
		return "pm-mcp", "PM MCP", nil
	case DeliveryCLI:
		return "pm-cli", "PM CLI", nil
	default:
		return "", "", ErrDeliveryInvalid
	}
}

func secureAuthorizationRequestID() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
