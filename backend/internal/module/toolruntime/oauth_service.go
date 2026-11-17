package toolruntime

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type OAuthClientRegistrationInput struct {
	Name         string
	RedirectURIs []string
}

type OAuthAuthorizationStartInput struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	PKCEChallenge       string
	PKCEChallengeMethod string
	State               string
	Resource            string
	Scope               string
	Channel             DeliveryChannel
}

type OAuthAuthorizationCompletion struct {
	RedirectURL string
}

type OAuthService struct {
	authorizations *AuthorizationService
	credentials    *CredentialService
	clients        OAuthClientRepository
	configuration  OAuthServerConfiguration
	now            func() time.Time
}

func NewOAuthService(
	authorizations *AuthorizationService,
	credentials *CredentialService,
	clients OAuthClientRepository,
	configuration OAuthServerConfiguration,
) (*OAuthService, error) {
	if authorizations == nil || credentials == nil || clients == nil || configuration.Validate() != nil {
		return nil, ErrOAuthConfigurationInvalid
	}
	return &OAuthService{
		authorizations: authorizations,
		credentials:    credentials,
		clients:        clients,
		configuration:  configuration,
		now:            time.Now,
	}, nil
}

func (service *OAuthService) RegisterClient(
	ctx context.Context,
	input OAuthClientRegistrationInput,
) (OAuthClient, error) {
	if !service.ready() || len(input.RedirectURIs) == 0 || len(input.RedirectURIs) > 16 {
		return OAuthClient{}, ErrOAuthClientInvalid
	}
	redirectURIs := make([]string, 0, len(input.RedirectURIs))
	seen := make(map[string]struct{}, len(input.RedirectURIs))
	for _, raw := range input.RedirectURIs {
		redirectURI, ok := normalizeRedirectURI(raw)
		if !ok {
			return OAuthClient{}, ErrOAuthClientInvalid
		}
		parsed, _ := url.Parse(redirectURI)
		if parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname()) {
			return OAuthClient{}, ErrOAuthClientInvalid
		}
		if _, duplicate := seen[redirectURI]; duplicate {
			return OAuthClient{}, ErrOAuthClientInvalid
		}
		seen[redirectURI] = struct{}{}
		redirectURIs = append(redirectURIs, redirectURI)
	}
	rawID, err := newOpaqueSecret(rand.Reader)
	if err != nil {
		return OAuthClient{}, ErrOAuthClientInvalid
	}
	client := OAuthClient{
		ID:           "pm_mcp_" + rawID,
		Name:         oauthClientDisplayName(input.Name),
		Channel:      DeliveryMCP,
		RedirectURIs: redirectURIs,
		CreatedAt:    service.now().UTC(),
	}
	if err := client.Validate(); err != nil {
		return OAuthClient{}, err
	}
	if err := service.clients.Create(ctx, &client); err != nil {
		return OAuthClient{}, ErrOAuthClientInvalid
	}
	return client, nil
}

func (service *OAuthService) StartAuthorization(
	ctx context.Context,
	principal ManagementPrincipal,
	input OAuthAuthorizationStartInput,
) (AuthorizationRequest, error) {
	if !service.ready() || input.ResponseType != "code" || input.PKCEChallengeMethod != PKCEMethodS256 ||
		strings.TrimSpace(input.ClientID) == "" {
		return AuthorizationRequest{}, ErrOAuthAuthorizationInvalid
	}
	clientID, clientName, channel, err := service.resolveClient(ctx, input.ClientID, input.RedirectURI, input.Channel)
	if err != nil || validateOAuthResource(service.configuration, channel, input.Resource) != nil {
		return AuthorizationRequest{}, ErrOAuthAuthorizationInvalid
	}
	scope, scopeOK := normalizeOAuthScope(input.Scope)
	if !scopeOK {
		return AuthorizationRequest{}, ErrOAuthAuthorizationInvalid
	}
	oauth := &OAuthAuthorizationContext{
		State: input.State, RedirectURI: input.RedirectURI, PKCEChallenge: input.PKCEChallenge,
		Resource: input.Resource, Scope: scope,
	}
	if err := oauth.Validate(); err != nil {
		return AuthorizationRequest{}, err
	}
	return service.authorizations.start(ctx, principal, AuthorizationRequestInput{Channel: channel}, clientID, clientName, oauth)
}

func (service *OAuthService) CompleteAuthorization(
	ctx context.Context,
	principal ManagementPrincipal,
	requestID string,
	decision AuthorizationDecision,
) (OAuthAuthorizationCompletion, error) {
	if !service.ready() || (decision != AuthorizationAuthorize && decision != AuthorizationDeny) {
		return OAuthAuthorizationCompletion{}, ErrOAuthAuthorizationInvalid
	}
	request, err := service.authorizations.Decide(ctx, principal, requestID, decision)
	if err != nil || request.OAuth == nil || request.OAuth.Validate() != nil {
		return OAuthAuthorizationCompletion{}, ErrOAuthAuthorizationInvalid
	}
	redirect, err := url.Parse(request.OAuth.RedirectURI)
	if err != nil {
		return OAuthAuthorizationCompletion{}, ErrOAuthAuthorizationInvalid
	}
	query := redirect.Query()
	query.Set("state", request.OAuth.State)
	query.Set("iss", service.configuration.Issuer)
	if decision == AuthorizationDeny {
		query.Set("error", "access_denied")
		redirect.RawQuery = query.Encode()
		return OAuthAuthorizationCompletion{RedirectURL: redirect.String()}, nil
	}
	issued, err := service.credentials.IssueAuthorizationCode(ctx, IssueAuthorizationCodeInput{
		PrincipalID:   principal.PrincipalID,
		WorkspaceID:   principal.WorkspaceID,
		RequestID:     request.ID,
		PKCEMethod:    PKCEMethodS256,
		PKCEChallenge: request.OAuth.PKCEChallenge,
		RedirectURI:   request.OAuth.RedirectURI,
	})
	if err != nil || issued.Code == "" {
		return OAuthAuthorizationCompletion{}, fmt.Errorf("%w: authorization code issuance failed", ErrOAuthAuthorizationInvalid)
	}
	query.Set("code", issued.Code)
	redirect.RawQuery = query.Encode()
	return OAuthAuthorizationCompletion{RedirectURL: redirect.String()}, nil
}

func (service *OAuthService) ResolveClientChannel(
	ctx context.Context,
	clientID string,
	resource string,
) (DeliveryChannel, error) {
	if clientID == OAuthClientCLI {
		if validateOAuthResource(service.configuration, DeliveryCLI, resource) != nil {
			return "", ErrOAuthClientInvalid
		}
		return DeliveryCLI, nil
	}
	client, err := service.clients.FindByID(ctx, strings.TrimSpace(clientID))
	if err != nil || client.Validate() != nil || validateOAuthResource(service.configuration, client.Channel, resource) != nil {
		return "", ErrOAuthClientInvalid
	}
	return client.Channel, nil
}

func (service *OAuthService) Configuration() OAuthServerConfiguration {
	return service.configuration
}

func (service *OAuthService) resolveClient(
	ctx context.Context,
	clientID string,
	redirectURI string,
	requestedChannel DeliveryChannel,
) (string, string, DeliveryChannel, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == OAuthClientCLI {
		redirect, ok := normalizeRedirectURI(redirectURI)
		if !ok || redirect != redirectURI || requestedChannel != DeliveryCLI {
			return "", "", "", ErrOAuthClientInvalid
		}
		parsed, _ := url.Parse(redirect)
		if parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname()) {
			return "", "", "", ErrOAuthClientInvalid
		}
		return OAuthClientCLI, "PM CLI", DeliveryCLI, nil
	}
	client, err := service.clients.FindByID(ctx, clientID)
	if err != nil || client.Validate() != nil || (requestedChannel != "" && requestedChannel != DeliveryMCP) {
		return "", "", "", ErrOAuthClientInvalid
	}
	for _, registered := range client.RedirectURIs {
		if redirectURIMatches(registered, redirectURI) {
			return client.ID, client.Name, client.Channel, nil
		}
	}
	return "", "", "", ErrOAuthClientInvalid
}

func (service *OAuthService) ready() bool {
	return service != nil && service.authorizations != nil && service.credentials != nil && service.clients != nil &&
		service.now != nil && service.configuration.Validate() == nil
}
