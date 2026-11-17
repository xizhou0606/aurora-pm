package toolruntime

import (
	"context"
	"crypto/rand"
	"strings"
	"time"
)

func NewCredentialService(
	grants GrantRepository,
	requests AuthorizationRequestRepository,
	credentials CredentialRepository,
	members WorkspaceMembershipVerifier,
	policy CredentialPolicy,
) (*CredentialService, error) {
	if grants == nil || requests == nil || credentials == nil || members == nil || !validCredentialPolicy(policy) {
		return nil, ErrCredentialConfigInvalid
	}
	return &CredentialService{
		grants: grants, requests: requests, credentials: credentials, members: members, policy: policy,
		now: time.Now, random: rand.Reader,
	}, nil
}

func (service *CredentialService) IssueAuthorizationCode(
	ctx context.Context,
	input IssueAuthorizationCodeInput,
) (IssuedAuthorizationCode, error) {
	if !service.ready() || input.PrincipalID == 0 || input.WorkspaceID == 0 ||
		!authorizationRequestIDPattern.MatchString(strings.TrimSpace(input.RequestID)) ||
		input.PKCEMethod != PKCEMethodS256 || !pkceChallengePattern.MatchString(input.PKCEChallenge) {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	redirectURI, ok := normalizeRedirectURI(input.RedirectURI)
	if !ok {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	now := service.now().UTC()
	request, err := service.requests.FindByID(ctx, strings.TrimSpace(input.RequestID))
	if err != nil || request.Validate() != nil || request.Decision != AuthorizationAuthorize || request.GrantID == nil ||
		request.PrincipalID != input.PrincipalID || request.WorkspaceID != input.WorkspaceID ||
		!now.Before(request.RequestExpiresAt.UTC()) {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	if request.OAuth != nil && (request.OAuth.PKCEChallenge != input.PKCEChallenge ||
		request.OAuth.RedirectURI != redirectURI) {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	grant, err := service.grants.FindByID(ctx, *request.GrantID)
	if err != nil || !grantUsableForCredential(grant, now, true) || !authorizationRequestMatchesGrant(request, grant) {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	if service.members.VerifyWorkspaceMembership(ctx, grant.PrincipalID, grant.WorkspaceID) != nil {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	audience, err := AudienceForChannel(grant.Channel)
	if err != nil {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	raw, err := newOpaqueSecret(service.random)
	if err != nil {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	digest, ok := credentialDigest(raw)
	if !ok {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	expiresAt := earlierTime(now.Add(service.policy.AuthorizationCodeTTL), grant.ExpiresAt)
	expiresAt = earlierTime(expiresAt, request.RequestExpiresAt)
	code := AuthorizationCodeCredential{
		CodeDigest: digest, AuthorizationRequestID: request.ID,
		Binding: CredentialBinding{
			PrincipalID: grant.PrincipalID, WorkspaceID: grant.WorkspaceID, GrantID: grant.ID,
			ClientID: grant.ClientID, Channel: grant.Channel, Audience: audience,
		},
		PKCEChallenge: input.PKCEChallenge, RedirectURI: redirectURI,
		ExpiresAt: expiresAt, CreatedAt: now,
	}
	if err := service.credentials.CreateAuthorizationCode(ctx, code); err != nil {
		return IssuedAuthorizationCode{}, ErrAuthorizationCodeInvalid
	}
	return IssuedAuthorizationCode{Code: raw, ExpiresAt: expiresAt}, nil
}

func (service *CredentialService) ExchangeAuthorizationCode(
	ctx context.Context,
	input ExchangeAuthorizationCodeInput,
) (IssuedTokenPair, error) {
	if !service.ready() || strings.TrimSpace(input.ClientID) == "" {
		return IssuedTokenPair{}, ErrAuthorizationCodeInvalid
	}
	codeDigest, ok := credentialDigest(input.Code)
	challenge, verifierOK := pkceS256Challenge(input.PKCEVerifier)
	redirectURI, redirectOK := normalizeRedirectURI(input.RedirectURI)
	audience, audienceErr := AudienceForChannel(input.Channel)
	if !ok || !verifierOK || !redirectOK || audienceErr != nil {
		return IssuedTokenPair{}, ErrAuthorizationCodeInvalid
	}
	code, err := service.credentials.FindAuthorizationCode(ctx, codeDigest)
	now := service.now().UTC()
	if err != nil || code.ConsumedAt != nil || !now.Before(code.ExpiresAt) ||
		!constantTimeTextEqual(code.PKCEChallenge, challenge) || code.RedirectURI != redirectURI ||
		!bindingMatchesClient(code.Binding, input.ClientID, input.Channel, audience) {
		return IssuedTokenPair{}, ErrAuthorizationCodeInvalid
	}
	grant, err := service.boundGrant(ctx, code.Binding, now, true)
	if err != nil || service.members.VerifyWorkspaceMembership(ctx, grant.PrincipalID, grant.WorkspaceID) != nil {
		return IssuedTokenPair{}, ErrAuthorizationCodeInvalid
	}
	pair, family, access, refresh, err := service.newTokenPair(code.Binding, grant.ExpiresAt, now)
	if err != nil {
		return IssuedTokenPair{}, ErrAuthorizationCodeInvalid
	}
	if err := service.credentials.ExchangeAuthorizationCode(ctx, codeDigest, now, family, access, refresh); err != nil {
		return IssuedTokenPair{}, ErrAuthorizationCodeInvalid
	}
	return pair, nil
}

func (service *CredentialService) newTokenPair(
	binding CredentialBinding,
	grantExpiresAt time.Time,
	now time.Time,
) (IssuedTokenPair, CredentialFamily, AccessCredential, RefreshCredential, error) {
	accessRaw, err := newOpaqueSecret(service.random)
	if err != nil {
		return IssuedTokenPair{}, CredentialFamily{}, AccessCredential{}, RefreshCredential{}, err
	}
	refreshRaw, err := newOpaqueSecret(service.random)
	if err != nil {
		return IssuedTokenPair{}, CredentialFamily{}, AccessCredential{}, RefreshCredential{}, err
	}
	familyID, err := newOpaqueSecret(service.random)
	if err != nil {
		return IssuedTokenPair{}, CredentialFamily{}, AccessCredential{}, RefreshCredential{}, err
	}
	accessDigest, accessOK := credentialDigest(accessRaw)
	refreshDigest, refreshOK := credentialDigest(refreshRaw)
	if !accessOK || !refreshOK {
		return IssuedTokenPair{}, CredentialFamily{}, AccessCredential{}, RefreshCredential{}, ErrCredentialConfigInvalid
	}
	absoluteExpiry := earlierTime(now.Add(service.policy.RefreshAbsoluteTTL), grantExpiresAt)
	accessExpiry := earlierTime(now.Add(service.policy.AccessTokenTTL), grantExpiresAt)
	refreshExpiry := earlierTime(now.Add(service.policy.RefreshTokenTTL), absoluteExpiry)
	if !now.Before(accessExpiry) || !now.Before(refreshExpiry) {
		return IssuedTokenPair{}, CredentialFamily{}, AccessCredential{}, RefreshCredential{}, ErrCredentialConfigInvalid
	}
	family := CredentialFamily{
		ID: familyID, Binding: binding, AbsoluteExpiresAt: absoluteExpiry, CreatedAt: now, UpdatedAt: now,
	}
	access := AccessCredential{
		TokenDigest: accessDigest, FamilyID: familyID, Binding: binding, ExpiresAt: accessExpiry, CreatedAt: now,
	}
	refresh := RefreshCredential{
		TokenDigest: refreshDigest, FamilyID: familyID, Binding: binding, ExpiresAt: refreshExpiry, CreatedAt: now,
	}
	pair := IssuedTokenPair{
		AccessToken: accessRaw, AccessExpiresAt: accessExpiry,
		RefreshToken: refreshRaw, RefreshExpiresAt: refreshExpiry,
	}
	return pair, family, access, refresh, nil
}

func (service *CredentialService) ready() bool {
	return service != nil && service.grants != nil && service.requests != nil && service.credentials != nil && service.members != nil &&
		service.now != nil && service.random != nil && validCredentialPolicy(service.policy)
}

func authorizationRequestMatchesGrant(request AuthorizationRequest, grant Grant) bool {
	return request.GrantID != nil && *request.GrantID == grant.ID &&
		request.PrincipalID == grant.PrincipalID && request.WorkspaceID == grant.WorkspaceID &&
		request.ClientID == grant.ClientID && request.Channel == grant.Channel &&
		request.EffectiveFrom.Equal(grant.EffectiveFrom) && request.ExpiresAt.Equal(grant.ExpiresAt) &&
		equalTools(canonicalTools(request.ToolNames), canonicalTools(grant.AllowedTools)) &&
		equalScopes(canonicalScopes(request.GrantScopes), canonicalScopes(grant.AllowedScopes))
}

func validCredentialPolicy(policy CredentialPolicy) bool {
	return policy.AuthorizationCodeTTL > 0 && policy.AccessTokenTTL > 0 && policy.RefreshTokenTTL > 0 &&
		policy.RefreshAbsoluteTTL > 0 && policy.RefreshTokenTTL <= policy.RefreshAbsoluteTTL
}

func earlierTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left.UTC()
	}
	return right.UTC()
}

func bindingMatchesClient(binding CredentialBinding, clientID string, channel DeliveryChannel, audience TokenAudience) bool {
	return binding.PrincipalID != 0 && binding.WorkspaceID != 0 && binding.GrantID != 0 &&
		binding.ClientID == strings.TrimSpace(clientID) && binding.Channel == channel && binding.Audience == audience
}

func grantUsableForCredential(grant Grant, now time.Time, allowScheduled bool) bool {
	if grant.Validate() != nil || grant.ID == 0 || now.IsZero() || grant.RevokedAt != nil || !now.Before(grant.ExpiresAt) {
		return false
	}
	return allowScheduled || grant.Status(now) == GrantActive
}

func (service *CredentialService) boundGrant(
	ctx context.Context,
	binding CredentialBinding,
	now time.Time,
	allowScheduled bool,
) (Grant, error) {
	grant, err := service.grants.FindByID(ctx, binding.GrantID)
	if err != nil || !grantUsableForCredential(grant, now, allowScheduled) {
		return Grant{}, ErrExternalTokenInvalid
	}
	audience, audienceErr := AudienceForChannel(grant.Channel)
	if audienceErr != nil || grant.PrincipalID != binding.PrincipalID || grant.WorkspaceID != binding.WorkspaceID ||
		grant.ClientID != binding.ClientID || grant.Channel != binding.Channel || audience != binding.Audience {
		return Grant{}, ErrExternalTokenInvalid
	}
	return grant, nil
}
