package toolruntime

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (service *CredentialService) RefreshAccessToken(
	ctx context.Context,
	input RefreshAccessTokenInput,
) (IssuedTokenPair, error) {
	if !service.ready() || strings.TrimSpace(input.ClientID) == "" {
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	digest, ok := credentialDigest(input.RefreshToken)
	audience, audienceErr := AudienceForChannel(input.Channel)
	if !ok || audienceErr != nil {
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	refresh, err := service.credentials.FindRefreshCredential(ctx, digest)
	now := service.now().UTC()
	if err != nil || !bindingMatchesClient(refresh.Binding, input.ClientID, input.Channel, audience) {
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	if refresh.ConsumedAt != nil {
		_ = service.credentials.RevokeRefreshFamily(ctx, digest, now)
		return IssuedTokenPair{}, ErrRefreshTokenReplay
	}
	if refresh.RevokedAt != nil || !now.Before(refresh.ExpiresAt) {
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	family, err := service.credentials.FindCredentialFamily(ctx, refresh.FamilyID)
	if err != nil || family.RevokedAt != nil || !now.Before(family.AbsoluteExpiresAt) ||
		!equalCredentialBindings(family.Binding, refresh.Binding) {
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	grant, err := service.boundGrant(ctx, refresh.Binding, now, false)
	if err != nil || service.members.VerifyWorkspaceMembership(ctx, grant.PrincipalID, grant.WorkspaceID) != nil {
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	pair, access, next, err := service.newRotatedTokenPair(refresh.Binding, family, grant.ExpiresAt, now)
	if err != nil {
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	if err := service.credentials.RotateRefreshToken(ctx, digest, now, access, next); err != nil {
		if errors.Is(err, ErrRefreshTokenReplay) {
			return IssuedTokenPair{}, ErrRefreshTokenReplay
		}
		return IssuedTokenPair{}, ErrRefreshTokenInvalid
	}
	return pair, nil
}

func (service *CredentialService) RevokeRefreshToken(ctx context.Context, input RevokeRefreshTokenInput) error {
	if !service.ready() || strings.TrimSpace(input.ClientID) == "" {
		return ErrRefreshTokenInvalid
	}
	digest, ok := credentialDigest(input.RefreshToken)
	audience, audienceErr := AudienceForChannel(input.Channel)
	if !ok || audienceErr != nil {
		return ErrRefreshTokenInvalid
	}
	refresh, err := service.credentials.FindRefreshCredential(ctx, digest)
	if err != nil || !bindingMatchesClient(refresh.Binding, input.ClientID, input.Channel, audience) {
		return ErrRefreshTokenInvalid
	}
	if err := service.credentials.RevokeRefreshFamily(ctx, digest, service.now().UTC()); err != nil {
		return ErrRefreshTokenInvalid
	}
	return nil
}

func (service *CredentialService) VerifyExternalToken(
	ctx context.Context,
	rawToken string,
	channel DeliveryChannel,
	audience TokenAudience,
) (ExecutionAuthorization, error) {
	if !service.ready() {
		return ExecutionAuthorization{}, ErrExternalTokenInvalid
	}
	digest, ok := credentialDigest(rawToken)
	expectedAudience, audienceErr := AudienceForChannel(channel)
	if !ok || audienceErr != nil || audience != expectedAudience {
		return ExecutionAuthorization{}, ErrExternalTokenInvalid
	}
	access, err := service.credentials.FindAccessCredential(ctx, digest)
	now := service.now().UTC()
	if err != nil || !now.Before(access.ExpiresAt) || access.Binding.Channel != channel ||
		access.Binding.Audience != audience {
		return ExecutionAuthorization{}, ErrExternalTokenInvalid
	}
	family, err := service.credentials.FindCredentialFamily(ctx, access.FamilyID)
	if err != nil || family.RevokedAt != nil || !now.Before(family.AbsoluteExpiresAt) ||
		!equalCredentialBindings(family.Binding, access.Binding) {
		return ExecutionAuthorization{}, ErrExternalTokenInvalid
	}
	grant, err := service.boundGrant(ctx, access.Binding, now, false)
	if err != nil {
		return ExecutionAuthorization{}, ErrExternalTokenInvalid
	}
	if service.members.VerifyWorkspaceMembership(ctx, grant.PrincipalID, grant.WorkspaceID) != nil {
		return ExecutionAuthorization{}, ExternalWorkspaceMembershipError{WorkspaceID: grant.WorkspaceID}
	}
	authorization := ExecutionAuthorization{
		PrincipalID: access.Binding.PrincipalID, WorkspaceID: access.Binding.WorkspaceID,
		GrantID: access.Binding.GrantID, ClientID: access.Binding.ClientID,
		Channel: access.Binding.Channel, Audience: access.Binding.Audience,
	}
	if ValidateExternalAuthorization(authorization, channel, audience) != nil {
		return ExecutionAuthorization{}, ErrExternalTokenInvalid
	}
	return authorization, nil
}

func (service *CredentialService) newRotatedTokenPair(
	binding CredentialBinding,
	family CredentialFamily,
	grantExpiresAt time.Time,
	now time.Time,
) (IssuedTokenPair, AccessCredential, RefreshCredential, error) {
	accessRaw, err := newOpaqueSecret(service.random)
	if err != nil {
		return IssuedTokenPair{}, AccessCredential{}, RefreshCredential{}, err
	}
	refreshRaw, err := newOpaqueSecret(service.random)
	if err != nil {
		return IssuedTokenPair{}, AccessCredential{}, RefreshCredential{}, err
	}
	accessDigest, accessOK := credentialDigest(accessRaw)
	refreshDigest, refreshOK := credentialDigest(refreshRaw)
	if !accessOK || !refreshOK {
		return IssuedTokenPair{}, AccessCredential{}, RefreshCredential{}, ErrCredentialConfigInvalid
	}
	accessExpiry := earlierTime(now.Add(service.policy.AccessTokenTTL), grantExpiresAt)
	refreshExpiry := earlierTime(now.Add(service.policy.RefreshTokenTTL), family.AbsoluteExpiresAt)
	refreshExpiry = earlierTime(refreshExpiry, grantExpiresAt)
	if !now.Before(accessExpiry) || !now.Before(refreshExpiry) {
		return IssuedTokenPair{}, AccessCredential{}, RefreshCredential{}, ErrCredentialConfigInvalid
	}
	access := AccessCredential{
		TokenDigest: accessDigest, FamilyID: family.ID, Binding: binding, ExpiresAt: accessExpiry, CreatedAt: now,
	}
	refresh := RefreshCredential{
		TokenDigest: refreshDigest, FamilyID: family.ID, Binding: binding, ExpiresAt: refreshExpiry, CreatedAt: now,
	}
	pair := IssuedTokenPair{
		AccessToken: accessRaw, AccessExpiresAt: accessExpiry,
		RefreshToken: refreshRaw, RefreshExpiresAt: refreshExpiry,
	}
	return pair, access, refresh, nil
}

func equalCredentialBindings(left, right CredentialBinding) bool {
	return left == right && left.PrincipalID != 0 && left.WorkspaceID != 0 && left.GrantID != 0 && left.ClientID != ""
}

var _ ExternalTokenVerifier = (*CredentialService)(nil)
