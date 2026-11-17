package toolruntime

import (
	"fmt"
	"sort"
	"strings"
)

func normalizeGrant(grant Grant) (Grant, error) {
	request, err := normalizeGrantRequest(GrantRequest{
		PrincipalID: grant.PrincipalID, WorkspaceID: grant.WorkspaceID,
		ClientID: grant.ClientID, Channel: grant.Channel, AllowedTools: grant.AllowedTools, AllowedScopes: grant.AllowedScopes,
		EffectiveFrom: grant.EffectiveFrom, ExpiresAt: grant.ExpiresAt,
	})
	if err != nil {
		return Grant{}, err
	}
	if grant.CreatedAt.IsZero() || grant.UpdatedAt.IsZero() {
		return Grant{}, fmt.Errorf("%w: timestamps are required", ErrGrantInvalid)
	}
	grant.ClientID, grant.Channel = request.ClientID, request.Channel
	grant.AllowedTools, grant.AllowedScopes = request.AllowedTools, request.AllowedScopes
	grant.EffectiveFrom, grant.ExpiresAt = request.EffectiveFrom, request.ExpiresAt
	grant.CreatedAt, grant.UpdatedAt = grant.CreatedAt.UTC(), grant.UpdatedAt.UTC()
	if grant.UpdatedAt.Before(grant.CreatedAt) {
		return Grant{}, fmt.Errorf("%w: updated_at cannot predate created_at", ErrGrantInvalid)
	}
	if grant.RevokedAt != nil {
		value := grant.RevokedAt.UTC()
		if value.Before(grant.CreatedAt) {
			return Grant{}, fmt.Errorf("%w: revoked_at cannot predate created_at", ErrGrantInvalid)
		}
		grant.RevokedAt = &value
	}
	return grant, nil
}

func normalizeGrantRequest(request GrantRequest) (GrantRequest, error) {
	request.ClientID = strings.TrimSpace(request.ClientID)
	if request.PrincipalID == 0 || request.WorkspaceID == 0 || request.ClientID == "" {
		return GrantRequest{}, fmt.Errorf("%w: principal, workspace, and client are required", ErrGrantInvalid)
	}
	if len(request.ClientID) > 255 {
		return GrantRequest{}, fmt.Errorf("%w: client_id is too long", ErrGrantInvalid)
	}
	if _, err := AudienceForChannel(request.Channel); err != nil {
		return GrantRequest{}, fmt.Errorf("%w: unsupported channel", ErrGrantInvalid)
	}
	if request.EffectiveFrom.IsZero() || request.ExpiresAt.IsZero() || !request.EffectiveFrom.Before(request.ExpiresAt) {
		return GrantRequest{}, fmt.Errorf("%w: effective window is invalid", ErrGrantInvalid)
	}
	request.EffectiveFrom, request.ExpiresAt = request.EffectiveFrom.UTC(), request.ExpiresAt.UTC()
	request.AllowedTools = canonicalTools(request.AllowedTools)
	if len(request.AllowedTools) == 0 {
		return GrantRequest{}, fmt.Errorf("%w: at least one tool is required", ErrGrantInvalid)
	}
	for _, tool := range request.AllowedTools {
		if !toolNamePattern.MatchString(tool) {
			return GrantRequest{}, fmt.Errorf("%w: invalid tool %q", ErrGrantInvalid, tool)
		}
	}
	request.AllowedScopes = canonicalScopes(request.AllowedScopes)
	if len(request.AllowedScopes) == 0 {
		return GrantRequest{}, fmt.Errorf("%w: at least one scope is required", ErrGrantInvalid)
	}
	for _, scope := range request.AllowedScopes {
		if !grantScopePattern.MatchString(scope) {
			return GrantRequest{}, fmt.Errorf("%w: invalid scope %q", ErrGrantInvalid, scope)
		}
	}
	return request, nil
}

func canonicalTools(tools []string) []string {
	seen := make(map[string]struct{}, len(tools))
	result := make([]string, 0, len(tools))
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		if _, exists := seen[tool]; !exists {
			seen[tool] = struct{}{}
			result = append(result, tool)
		}
	}
	sort.Strings(result)
	return result
}

func canonicalScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; !exists {
			seen[scope] = struct{}{}
			result = append(result, scope)
		}
	}
	sort.Strings(result)
	return result
}

func grantAllowsScope(scopes []string, requested string) bool {
	if !grantScopePattern.MatchString(requested) {
		return false
	}
	index := sort.SearchStrings(scopes, requested)
	return index < len(scopes) && scopes[index] == requested
}

func grantAllowsTool(tools []string, requested string) bool {
	if !toolNamePattern.MatchString(requested) {
		return false
	}
	index := sort.SearchStrings(tools, requested)
	return index < len(tools) && tools[index] == requested
}

func equalTools(left, right []string) bool {
	return equalStrings(left, right)
}

func equalScopes(left, right []string) bool {
	return equalStrings(left, right)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
