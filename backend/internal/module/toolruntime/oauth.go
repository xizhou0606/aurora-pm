package toolruntime

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	OAuthScopeTools         = "pm:tools"
	OAuthScopeOfflineAccess = "offline_access"
	OAuthClientCLI          = "pm-cli"
)

var (
	ErrOAuthConfigurationInvalid = errors.New("external OAuth configuration is invalid")
	ErrOAuthClientInvalid        = errors.New("external OAuth client is invalid")
	ErrOAuthAuthorizationInvalid = errors.New("external OAuth authorization is invalid")
)

// OAuthClient is a public client registration. PM never issues or persists a
// client secret: authorization-code exchange is protected by S256 PKCE.
type OAuthClient struct {
	ID           string
	Name         string
	Channel      DeliveryChannel
	RedirectURIs []string
	CreatedAt    time.Time
}

type OAuthClientRepository interface {
	Create(context.Context, *OAuthClient) error
	FindByID(context.Context, string) (OAuthClient, error)
}

type OAuthAuthorizationContext struct {
	State         string
	RedirectURI   string
	PKCEChallenge string
	Resource      string
	Scope         string
}

func (oauth OAuthAuthorizationContext) Validate() error {
	if !validOAuthState(oauth.State) || !pkceChallengePattern.MatchString(oauth.PKCEChallenge) {
		return ErrOAuthAuthorizationInvalid
	}
	scope, scopeOK := normalizeOAuthScope(oauth.Scope)
	if !scopeOK || scope != oauth.Scope {
		return ErrOAuthAuthorizationInvalid
	}
	redirectURI, redirectOK := normalizeRedirectURI(oauth.RedirectURI)
	resource, resourceOK := normalizeOAuthResource(oauth.Resource)
	if !redirectOK || redirectURI != oauth.RedirectURI || !resourceOK || resource != oauth.Resource {
		return ErrOAuthAuthorizationInvalid
	}
	return nil
}

func (client OAuthClient) Validate() error {
	if strings.TrimSpace(client.ID) == "" || len(client.ID) > 255 || strings.TrimSpace(client.Name) == "" ||
		len(client.Name) > 255 || client.Channel != DeliveryMCP || client.CreatedAt.IsZero() ||
		len(client.RedirectURIs) == 0 || len(client.RedirectURIs) > 16 {
		return ErrOAuthClientInvalid
	}
	seen := make(map[string]struct{}, len(client.RedirectURIs))
	for _, raw := range client.RedirectURIs {
		redirectURI, ok := normalizeRedirectURI(raw)
		if !ok || redirectURI != raw {
			return ErrOAuthClientInvalid
		}
		parsed, _ := url.Parse(redirectURI)
		if parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname()) {
			return ErrOAuthClientInvalid
		}
		if _, duplicate := seen[redirectURI]; duplicate {
			return ErrOAuthClientInvalid
		}
		seen[redirectURI] = struct{}{}
	}
	return nil
}

type OAuthServerConfiguration struct {
	Issuer                      string
	AuthorizationEndpoint       string
	DeviceAuthorizationEndpoint string
	DeviceVerificationEndpoint  string
	TokenEndpoint               string
	RegistrationEndpoint        string
	ProtectedResourceMetadata   string
	MCPResource                 string
	CLIResource                 string
}

func NewOAuthServerConfiguration(publicBaseURL string) (OAuthServerConfiguration, error) {
	base, ok := canonicalOAuthBaseURL(publicBaseURL)
	if !ok {
		return OAuthServerConfiguration{}, ErrOAuthConfigurationInvalid
	}
	return OAuthServerConfiguration{
		Issuer:                      base,
		AuthorizationEndpoint:       base + "/workspace/mcp-cli/authorize",
		DeviceAuthorizationEndpoint: base + "/api/v1/tool-auth/device",
		DeviceVerificationEndpoint:  base + "/workspace/mcp-cli/device",
		TokenEndpoint:               base + "/api/v1/tool-auth/token",
		RegistrationEndpoint:        base + "/api/v1/tool-auth/register",
		ProtectedResourceMetadata:   base + "/.well-known/oauth-protected-resource",
		MCPResource:                 base + "/api/v1/mcp",
		CLIResource:                 base + "/api/v1/tool-runtime",
	}, nil
}

func (configuration OAuthServerConfiguration) Validate() error {
	canonical, ok := canonicalOAuthBaseURL(configuration.Issuer)
	if !ok || canonical != configuration.Issuer {
		return ErrOAuthConfigurationInvalid
	}
	expected, err := NewOAuthServerConfiguration(canonical)
	if err != nil || expected != configuration {
		return ErrOAuthConfigurationInvalid
	}
	return nil
}

func canonicalOAuthBaseURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return "", false
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return parsed.String(), true
}

func normalizeOAuthResource(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return "", false
	}
	parsed.RawPath = ""
	return parsed.String(), true
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func validOAuthState(value string) bool {
	if value == "" || len(value) > 1024 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func normalizeOAuthScope(raw string) (string, bool) {
	values := strings.Fields(raw)
	if len(values) == 0 || len(values) > 2 {
		return "", false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != OAuthScopeTools && value != OAuthScopeOfflineAccess {
			return "", false
		}
		if _, duplicate := seen[value]; duplicate {
			return "", false
		}
		seen[value] = struct{}{}
	}
	if _, ok := seen[OAuthScopeTools]; !ok {
		return "", false
	}
	if _, ok := seen[OAuthScopeOfflineAccess]; ok {
		return OAuthScopeTools + " " + OAuthScopeOfflineAccess, true
	}
	return OAuthScopeTools, true
}

func redirectURIMatches(registered, actual string) bool {
	registeredURI, registeredOK := normalizeRedirectURI(registered)
	actualURI, actualOK := normalizeRedirectURI(actual)
	if !registeredOK || !actualOK {
		return false
	}
	if registeredURI == actualURI {
		return true
	}
	registeredURL, _ := url.Parse(registeredURI)
	actualURL, _ := url.Parse(actualURI)
	return registeredURL.Scheme == "http" && actualURL.Scheme == "http" &&
		isLoopbackHost(registeredURL.Hostname()) &&
		strings.EqualFold(registeredURL.Hostname(), actualURL.Hostname()) &&
		registeredURL.Port() == "" && actualURL.Port() != "" &&
		registeredURL.EscapedPath() == actualURL.EscapedPath() &&
		registeredURL.RawQuery == actualURL.RawQuery
}

func oauthClientDisplayName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Codex MCP"
	}
	if len(value) > 255 {
		return value[:255]
	}
	return value
}

func validateOAuthResource(configuration OAuthServerConfiguration, channel DeliveryChannel, resource string) error {
	expected := configuration.MCPResource
	if channel == DeliveryCLI {
		expected = configuration.CLIResource
	}
	if resource != expected {
		return fmt.Errorf("%w: resource does not match channel", ErrOAuthAuthorizationInvalid)
	}
	return nil
}
