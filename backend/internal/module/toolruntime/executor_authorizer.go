package toolruntime

import (
	"context"
	"errors"
	"time"
)

// GrantAuthorizer resolves and verifies one persisted grant per request. The
// returned exact tool and scope sets are reused for discovery and invocation.
type GrantAuthorizer struct {
	grants         GrantRepository
	configurations WorkspaceConfigurationRepository
	now            func() time.Time
}

func NewGrantAuthorizer(
	grants GrantRepository,
	now func() time.Time,
	configurations WorkspaceConfigurationRepository,
) (*GrantAuthorizer, error) {
	if grants == nil || now == nil || configurations == nil {
		return nil, ErrExecutorInvalid
	}
	return &GrantAuthorizer{grants: grants, configurations: configurations, now: now}, nil
}

func (authorizer *GrantAuthorizer) AuthorizeGrant(ctx context.Context, authorization ExecutionAuthorization) (AuthorizedGrant, error) {
	if authorizer == nil || authorizer.grants == nil || authorizer.configurations == nil || authorizer.now == nil {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	grant, err := authorizer.grants.FindByID(ctx, authorization.GrantID)
	if err != nil {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	tools, scopes := canonicalTools(grant.AllowedTools), canonicalScopes(grant.AllowedScopes)
	if len(tools) == 0 || len(scopes) == 0 {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	now := authorizer.now().UTC()
	err = grant.Authorize(AccessRequest{
		GrantID: authorization.GrantID, PrincipalID: authorization.PrincipalID,
		WorkspaceID: authorization.WorkspaceID, ClientID: authorization.ClientID,
		Channel: authorization.Channel, Audience: authorization.Audience, Tool: tools[0], Scope: scopes[0],
	}, now)
	if err != nil {
		if errors.Is(err, ErrGrantNotActive) {
			return AuthorizedGrant{}, ErrToolUATBlocked
		}
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	configuration, err := authorizer.configurations.FindByWorkspace(ctx, authorization.WorkspaceID)
	if err != nil || configuration.WorkspaceID != authorization.WorkspaceID || configuration.Validate() != nil {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	if !configuration.RuntimeAllows(now) {
		return AuthorizedGrant{}, ErrToolUATBlocked
	}
	effectiveTools := intersectEntitlements(tools, configuration.AllowedTools)
	effectiveScopes := intersectEntitlements(scopes, configuration.AllowedScopes)
	// Current-user discovery is the minimum identity projection of an already
	// validated Grant. It is server-injected so pre-user.me workspace configs
	// and Grants do not force users through a second authorization cycle.
	effectiveTools = append(effectiveTools, PMCurrentUserToolName)
	effectiveScopes = append(effectiveScopes, PMCurrentUserReadScope)
	result, err := NewAuthorizedGrant(effectiveTools, effectiveScopes)
	if err != nil {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	return result, nil
}

func intersectEntitlements(left, right []string) []string {
	allowed := make(map[string]struct{}, len(right))
	for _, value := range right {
		allowed[value] = struct{}{}
	}
	result := make([]string, 0, len(left))
	for _, value := range left {
		if _, ok := allowed[value]; ok {
			result = append(result, value)
		}
	}
	return result
}
