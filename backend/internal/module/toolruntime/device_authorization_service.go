package toolruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"io"
	"net/url"
	"strings"
	"time"
)

func NewDeviceAuthorizationService(
	authorizations *AuthorizationService,
	credentials *CredentialService,
	repository DeviceAuthorizationRepository,
	configuration OAuthServerConfiguration,
	policy DeviceAuthorizationPolicy,
) (*DeviceAuthorizationService, error) {
	if authorizations == nil || credentials == nil || repository == nil || configuration.Validate() != nil ||
		policy.TTL <= 0 || policy.PollInterval <= 0 {
		return nil, ErrDeviceAuthorizationInvalid
	}
	return &DeviceAuthorizationService{
		authorizations: authorizations, credentials: credentials, repository: repository,
		configuration: configuration, policy: policy, now: time.Now, random: rand.Reader,
	}, nil
}

func (service *DeviceAuthorizationService) Start(
	ctx context.Context,
	input StartDeviceAuthorizationInput,
) (StartedDeviceAuthorization, error) {
	scope, scopeOK := normalizeOAuthScope(input.Scope)
	if !service.ready() || input.ClientID != OAuthClientCLI || input.WorkspaceID == 0 ||
		validateOAuthResource(service.configuration, DeliveryCLI, input.Resource) != nil || !scopeOK {
		return StartedDeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	deviceCode, err := newOpaqueSecret(service.random)
	if err != nil {
		return StartedDeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	userCode, err := newDeviceUserCode(service.random)
	if err != nil {
		return StartedDeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	deviceDigest, deviceOK := credentialDigest(deviceCode)
	userDigest, userOK := deviceUserCodeDigest(userCode)
	if !deviceOK || !userOK {
		return StartedDeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	now := service.now().UTC()
	record := DeviceAuthorization{
		UserCode:         userCode,
		DeviceCodeDigest: deviceDigest, UserCodeDigest: userDigest, WorkspaceID: input.WorkspaceID,
		ClientID: OAuthClientCLI, Channel: DeliveryCLI, Audience: AudienceCLI,
		Resource: service.configuration.CLIResource, Scope: scope, Status: DeviceAuthorizationPending,
		ExpiresAt: now.Add(service.policy.TTL), PollInterval: service.policy.PollInterval,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := service.repository.Create(ctx, record); err != nil {
		return StartedDeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	verificationURI := service.configuration.DeviceVerificationEndpoint
	complete, err := url.Parse(verificationURI)
	if err != nil {
		return StartedDeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	query := complete.Query()
	query.Set("user_code", userCode)
	complete.RawQuery = query.Encode()
	return StartedDeviceAuthorization{
		DeviceCode: deviceCode, UserCode: userCode, VerificationURI: verificationURI,
		VerificationURIComplete: complete.String(), ExpiresIn: service.policy.TTL, Interval: service.policy.PollInterval,
	}, nil
}

func (service *DeviceAuthorizationService) Review(
	ctx context.Context,
	principal ManagementPrincipal,
	userCode string,
) (DeviceAuthorizationReview, error) {
	authorization, err := service.boundByUserCode(ctx, principal, userCode)
	if err != nil {
		return DeviceAuthorizationReview{}, err
	}
	now := service.now().UTC()
	status := visibleDeviceStatus(authorization, now)
	configuration, configErr := service.authorizations.configurationForAuthorization(ctx, principal.WorkspaceID, now)
	if configErr != nil && status == DeviceAuthorizationPending {
		return DeviceAuthorizationReview{}, configErr
	}
	var tools []Definition
	var mode WorkspaceConfigurationMode
	var effectiveFrom, grantExpiresAt time.Time
	if configErr == nil {
		mode = configuration.Mode
		effectiveFrom, grantExpiresAt, _ = configuration.GrantWindow(DeliveryCLI, now)
		for _, name := range configuration.AllowedTools {
			definition, ok := service.authorizations.catalog.Find(name)
			if !ok || definition.Lifecycle == LifecycleDraft {
				return DeviceAuthorizationReview{}, ErrAuthorizationCatalogInvalid
			}
			tools = append(tools, definition)
		}
	}
	return DeviceAuthorizationReview{
		UserCode: normalizeDeviceUserCode(userCode), Status: status, ClientID: OAuthClientCLI, ClientName: "PM CLI",
		WorkspaceID: principal.WorkspaceID, WorkspaceName: principal.WorkspaceName, Mode: mode, Tools: tools,
		EffectiveFrom: effectiveFrom, GrantExpiresAt: grantExpiresAt, RequestExpiresAt: authorization.ExpiresAt,
	}, nil
}

func (service *DeviceAuthorizationService) Decide(
	ctx context.Context,
	principal ManagementPrincipal,
	userCode string,
	decision AuthorizationDecision,
) (DeviceAuthorizationReview, error) {
	if decision != AuthorizationAuthorize && decision != AuthorizationDeny {
		return DeviceAuthorizationReview{}, ErrAuthorizationDecisionInvalid
	}
	digest, ok := deviceUserCodeDigest(userCode)
	if !ok || validateManagementPrincipal(principal) != nil {
		return DeviceAuthorizationReview{}, ErrDeviceAuthorizationInvalid
	}
	now := service.now().UTC()
	claimed, err := service.repository.ClaimDecision(ctx, digest, principal.WorkspaceID, decision, now)
	if err != nil {
		return DeviceAuthorizationReview{}, err
	}
	if decision == AuthorizationDeny {
		return service.Review(ctx, principal, userCode)
	}
	request, err := service.authorizations.Start(ctx, principal, AuthorizationRequestInput{Channel: DeliveryCLI})
	if err != nil {
		_ = service.repository.FailApproval(ctx, digest, now)
		return DeviceAuthorizationReview{}, err
	}
	request, err = service.authorizations.Decide(ctx, principal, request.ID, AuthorizationAuthorize)
	if err != nil || request.GrantID == nil {
		_ = service.repository.FailApproval(ctx, digest, now)
		return DeviceAuthorizationReview{}, ErrDeviceAuthorizationInvalid
	}
	binding := CredentialBinding{
		PrincipalID: principal.PrincipalID, WorkspaceID: principal.WorkspaceID, GrantID: *request.GrantID,
		ClientID: OAuthClientCLI, Channel: DeliveryCLI, Audience: AudienceCLI,
	}
	if _, err := service.repository.CompleteApproval(ctx, claimed.UserCodeDigest, binding, request.ID, service.now().UTC()); err != nil {
		_ = service.repository.FailApproval(ctx, digest, service.now().UTC())
		return DeviceAuthorizationReview{}, err
	}
	return service.Review(ctx, principal, userCode)
}

func (service *DeviceAuthorizationService) Exchange(
	ctx context.Context,
	deviceCode string,
	clientID string,
) (IssuedTokenPair, error) {
	digest, ok := credentialDigest(deviceCode)
	if !service.ready() || !ok || strings.TrimSpace(clientID) != OAuthClientCLI {
		return IssuedTokenPair{}, ErrDeviceAuthorizationInvalid
	}
	now := service.now().UTC()
	authorization, err := service.repository.Poll(ctx, digest, now)
	if err != nil {
		return IssuedTokenPair{}, err
	}
	if authorization.Status != DeviceAuthorizationApproved || authorization.PrincipalID == 0 || authorization.GrantID == 0 {
		return IssuedTokenPair{}, ErrDeviceAuthorizationInvalid
	}
	binding := CredentialBinding{
		PrincipalID: authorization.PrincipalID, WorkspaceID: authorization.WorkspaceID, GrantID: authorization.GrantID,
		ClientID: authorization.ClientID, Channel: authorization.Channel, Audience: authorization.Audience,
	}
	grant, err := service.credentials.boundGrant(ctx, binding, now, true)
	if err != nil || service.credentials.members.VerifyWorkspaceMembership(ctx, grant.PrincipalID, grant.WorkspaceID) != nil {
		return IssuedTokenPair{}, ErrDeviceAuthorizationInvalid
	}
	pair, family, access, refresh, err := service.credentials.newTokenPair(binding, grant.ExpiresAt, now)
	if err != nil {
		return IssuedTokenPair{}, ErrDeviceAuthorizationInvalid
	}
	if err := service.repository.Consume(ctx, digest, now, family, access, refresh); err != nil {
		return IssuedTokenPair{}, err
	}
	return pair, nil
}

func (service *DeviceAuthorizationService) boundByUserCode(ctx context.Context, principal ManagementPrincipal, userCode string) (DeviceAuthorization, error) {
	digest, ok := deviceUserCodeDigest(userCode)
	if !service.ready() || !ok || validateManagementPrincipal(principal) != nil {
		return DeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	authorization, err := service.repository.FindByUserCode(ctx, digest)
	if err != nil || authorization.WorkspaceID != principal.WorkspaceID {
		return DeviceAuthorization{}, ErrDeviceAuthorizationInvalid
	}
	return authorization, nil
}

func (service *DeviceAuthorizationService) ready() bool {
	return service != nil && service.authorizations != nil && service.credentials != nil && service.repository != nil &&
		service.now != nil && service.random != nil && service.configuration.Validate() == nil &&
		service.policy.TTL > 0 && service.policy.PollInterval > 0
}

func newDeviceUserCode(random io.Reader) (string, error) {
	raw := make([]byte, 5)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return encoded[:4] + "-" + encoded[4:], nil
}

func normalizeDeviceUserCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	if len(value) != 8 {
		return ""
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < '2' || character > '7') {
			return ""
		}
	}
	return value[:4] + "-" + value[4:]
}

func deviceUserCodeDigest(value string) (string, bool) {
	normalized := normalizeDeviceUserCode(value)
	if normalized == "" {
		return "", false
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), true
}

func visibleDeviceStatus(value DeviceAuthorization, now time.Time) DeviceAuthorizationStatus {
	if !now.Before(value.ExpiresAt) && value.Status != DeviceAuthorizationConsumed {
		return DeviceAuthorizationExpired
	}
	if value.Status == deviceAuthorizationApproving {
		return DeviceAuthorizationPending
	}
	return value.Status
}
