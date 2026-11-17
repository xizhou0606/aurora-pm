// Package telegramlogin implements Telegram's current OIDC authorization-code
// flow. It uses PKCE S256 and reads identity exclusively from a verified ID
// token because Telegram does not expose a UserInfo endpoint.
package telegramlogin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const telegramIssuer = "https://oauth.telegram.org"

var (
	ErrRejected  = errors.New("Telegram login provider rejected request")
	ErrMalformed = errors.New("Telegram login provider returned malformed identity")
)

type tokenVerifier interface {
	Verify(context.Context, string) (*oidc.IDToken, error)
}

type Client struct {
	oauth      oauth2.Config
	verifier   tokenVerifier
	httpClient *http.Client
}

func New(ctx context.Context, cfg config.TelegramLogin) (*Client, error) {
	httpClient, redirect, err := validatedClientConfig(cfg)
	if err != nil {
		return nil, err
	}
	discoveryContext, cancel := context.WithTimeout(oidc.ClientContext(ctx, httpClient), httpClient.Timeout)
	defer cancel()
	provider, err := oidc.NewProvider(discoveryContext, telegramIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover Telegram OIDC provider: %w", identity.ErrLoginProviderUnavailable)
	}
	endpoint := provider.Endpoint()
	// Telegram's token endpoint requires HTTP Basic client authentication.
	endpoint.AuthStyle = oauth2.AuthStyleInHeader
	clientID := strings.TrimSpace(cfg.ClientID)
	return &Client{
		oauth: oauth2.Config{
			ClientID: clientID, ClientSecret: strings.TrimSpace(cfg.ClientSecret), RedirectURL: redirect,
			Endpoint: endpoint, Scopes: []string{oidc.ScopeOpenID, "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}), httpClient: httpClient,
	}, nil
}

func (c *Client) AuthorizationURL(state, nonce, codeChallenge string) (string, error) {
	if c == nil || c.verifier == nil {
		return "", identity.ErrLoginProviderUnavailable
	}
	state, nonce, codeChallenge = strings.TrimSpace(state), strings.TrimSpace(nonce), strings.TrimSpace(codeChallenge)
	decodedChallenge, err := base64.RawURLEncoding.DecodeString(codeChallenge)
	if state == "" || nonce == "" || err != nil || len(decodedChallenge) != 32 {
		return "", ErrRejected
	}
	return c.oauth.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

func (c *Client) Exchange(ctx context.Context, code, codeVerifier string) (identity.TelegramIdentity, error) {
	if c == nil || c.verifier == nil || c.httpClient == nil {
		return identity.TelegramIdentity{}, identity.ErrLoginProviderUnavailable
	}
	code, codeVerifier = strings.TrimSpace(code), strings.TrimSpace(codeVerifier)
	if code == "" || !validPKCEVerifier(codeVerifier) {
		return identity.TelegramIdentity{}, ErrRejected
	}
	providerContext := oidc.ClientContext(ctx, c.httpClient)
	token, err := c.oauth.Exchange(providerContext, code,
		oauth2.VerifierOption(codeVerifier),
		oauth2.SetAuthURLParam("client_id", c.oauth.ClientID),
	)
	if err != nil {
		var rejected *oauth2.RetrieveError
		if errors.As(err, &rejected) && rejected.Response != nil && rejected.Response.StatusCode >= 400 && rejected.Response.StatusCode < 500 {
			return identity.TelegramIdentity{}, ErrRejected
		}
		return identity.TelegramIdentity{}, identity.ErrLoginProviderUnavailable
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return identity.TelegramIdentity{}, ErrMalformed
	}
	verified, err := c.verifier.Verify(providerContext, rawIDToken)
	if err != nil {
		return identity.TelegramIdentity{}, ErrRejected
	}
	var claims struct {
		Issuer            string `json:"iss"`
		Subject           string `json:"sub"`
		TelegramUserID    int64  `json:"id"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		PictureURL        string `json:"picture"`
		Nonce             string `json:"nonce"`
	}
	if err := verified.Claims(&claims); err != nil {
		return identity.TelegramIdentity{}, ErrMalformed
	}
	if strings.TrimSpace(claims.Issuer) != strings.TrimSpace(verified.Issuer) || claims.TelegramUserID <= 0 ||
		strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.Nonce) == "" {
		return identity.TelegramIdentity{}, ErrMalformed
	}
	return identity.TelegramIdentity{
		Issuer: strings.TrimSpace(claims.Issuer), Subject: strings.TrimSpace(claims.Subject),
		TelegramUserID: claims.TelegramUserID, Name: strings.TrimSpace(claims.Name),
		PreferredUsername: strings.TrimSpace(claims.PreferredUsername), PictureURL: strings.TrimSpace(claims.PictureURL),
		Nonce: strings.TrimSpace(claims.Nonce),
	}, nil
}

func validPKCEVerifier(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("-._~", character) {
			continue
		}
		return false
	}
	return true
}

func validatedClientConfig(cfg config.TelegramLogin) (*http.Client, string, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, "", errors.New("Telegram login client credentials are incomplete")
	}
	redirect, err := url.Parse(strings.TrimSpace(cfg.RedirectURL))
	if err != nil || redirect.Host == "" || (redirect.Scheme != "http" && redirect.Scheme != "https") || redirect.User != nil || redirect.Fragment != "" {
		return nil, "", errors.New("Telegram login redirect URL must be an absolute http(s) URL without credentials or fragment")
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{Timeout: timeout}, redirect.String(), nil
}

var _ identity.TelegramLoginProvider = (*Client)(nil)
