// Package googlelogin implements PM's Google OAuth 2.0 and OpenID Connect
// client. It verifies the ID token before returning provider claims to the
// identity domain; it never treats access-token userinfo as login proof.
package googlelogin

import (
	"context"
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

const googleIssuer = "https://accounts.google.com"

var (
	ErrRejected  = errors.New("Google login provider rejected request")
	ErrMalformed = errors.New("Google login provider returned malformed identity")
)

type tokenVerifier interface {
	Verify(context.Context, string) (*oidc.IDToken, error)
}

type Client struct {
	oauth      oauth2.Config
	verifier   tokenVerifier
	httpClient *http.Client
}

func New(ctx context.Context, cfg config.GoogleLogin) (*Client, error) {
	client, redirect, err := validatedClientConfig(cfg)
	if err != nil {
		return nil, err
	}
	discoveryContext, cancel := context.WithTimeout(oidc.ClientContext(ctx, client), client.Timeout)
	defer cancel()
	provider, err := oidc.NewProvider(discoveryContext, googleIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover Google OIDC provider: %w", identity.ErrLoginProviderUnavailable)
	}
	return &Client{
		oauth: oauth2.Config{
			ClientID: strings.TrimSpace(cfg.ClientID), ClientSecret: strings.TrimSpace(cfg.ClientSecret),
			RedirectURL: redirect, Endpoint: provider.Endpoint(),
			Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier:   provider.Verifier(&oidc.Config{ClientID: strings.TrimSpace(cfg.ClientID)}),
		httpClient: client,
	}, nil
}

func (c *Client) AuthorizationURL(state, nonce string) (string, error) {
	if c == nil || c.verifier == nil {
		return "", identity.ErrLoginProviderUnavailable
	}
	state, nonce = strings.TrimSpace(state), strings.TrimSpace(nonce)
	if state == "" || nonce == "" {
		return "", ErrRejected
	}
	return c.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	), nil
}

func (c *Client) Exchange(ctx context.Context, code string) (identity.GoogleIdentity, error) {
	if c == nil || c.verifier == nil || c.httpClient == nil {
		return identity.GoogleIdentity{}, identity.ErrLoginProviderUnavailable
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return identity.GoogleIdentity{}, ErrRejected
	}
	providerContext := oidc.ClientContext(ctx, c.httpClient)
	token, err := c.oauth.Exchange(providerContext, code)
	if err != nil {
		var rejected *oauth2.RetrieveError
		if errors.As(err, &rejected) && rejected.Response != nil && rejected.Response.StatusCode >= 400 && rejected.Response.StatusCode < 500 {
			return identity.GoogleIdentity{}, ErrRejected
		}
		return identity.GoogleIdentity{}, identity.ErrLoginProviderUnavailable
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return identity.GoogleIdentity{}, ErrMalformed
	}
	verified, err := c.verifier.Verify(providerContext, rawIDToken)
	if err != nil {
		return identity.GoogleIdentity{}, ErrRejected
	}
	var claims struct {
		Issuer        string `json:"iss"`
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Nonce         string `json:"nonce"`
	}
	if err := verified.Claims(&claims); err != nil {
		return identity.GoogleIdentity{}, ErrMalformed
	}
	claimIssuer := strings.TrimSpace(claims.Issuer)
	verifiedIssuer := strings.TrimSpace(verified.Issuer)
	if claimIssuer == "" {
		claimIssuer = verifiedIssuer
	}
	if claimIssuer != verifiedIssuer || claims.Subject == "" || claims.Email == "" || claims.Nonce == "" {
		return identity.GoogleIdentity{}, ErrMalformed
	}
	// Google is allowed by its OIDC metadata (and by go-oidc) to return the
	// historical issuer without a scheme. Persist one canonical issuer.
	if claimIssuer == "accounts.google.com" {
		claimIssuer = googleIssuer
	}
	return identity.GoogleIdentity{
		Issuer: claimIssuer, Subject: strings.TrimSpace(claims.Subject),
		Email: strings.TrimSpace(claims.Email), EmailVerified: claims.EmailVerified,
		Nonce: strings.TrimSpace(claims.Nonce),
	}, nil
}

func validatedClientConfig(cfg config.GoogleLogin) (*http.Client, string, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, "", errors.New("Google login client credentials are incomplete")
	}
	redirect, err := url.Parse(strings.TrimSpace(cfg.RedirectURL))
	if err != nil || redirect.Host == "" || (redirect.Scheme != "http" && redirect.Scheme != "https") || redirect.User != nil || redirect.Fragment != "" {
		return nil, "", errors.New("Google login redirect URL must be an absolute http(s) URL without credentials or fragment")
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{Timeout: timeout}, redirect.String(), nil
}

var _ identity.GoogleLoginProvider = (*Client)(nil)
