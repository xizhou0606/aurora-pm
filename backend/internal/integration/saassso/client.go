// Package saassso implements PM's relying-party client for the SaaS Control
// authorization-code issuer. It does not implement or modify that issuer.
package saassso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const maxResponseBytes = 128 << 10

var (
	ErrUnavailable = errors.New("sso provider unavailable")
	ErrRejected    = errors.New("sso provider rejected request")
	ErrMalformed   = errors.New("sso provider returned malformed response")
)

type Identity struct {
	Issuer      string
	Subject     string
	Username    string
	DisplayName string
	Email       string
}

type Client struct {
	authorizeURL string
	tokenURL     string
	issuer       string
	clientID     string
	clientSecret string
	redirectURL  string
	http         *http.Client
}

func New(cfg config.SSO) (*Client, error) {
	return NewWithHTTPClient(cfg, nil)
}

// NewForTest creates an enabled client with an injected transport. Production
// construction remains config-bound through New.
func NewForTest(cfg config.SSO, client *http.Client) (*Client, error) {
	cfg.Enabled = true
	return NewWithHTTPClient(cfg, client)
}

func NewWithHTTPClient(cfg config.SSO, client *http.Client) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	values := []struct{ name, value string }{
		{"authorize URL", cfg.AuthorizeURL}, {"token URL", cfg.TokenURL},
		{"issuer", cfg.Issuer}, {"client ID", cfg.ClientID},
		{"client secret", cfg.ClientSecret}, {"redirect URL", cfg.RedirectURL},
	}
	for _, value := range values {
		if strings.TrimSpace(value.value) == "" {
			return nil, fmt.Errorf("SSO %s is required", value.name)
		}
	}
	authorizeURL, err := validEndpointURL(cfg.AuthorizeURL)
	if err != nil {
		return nil, fmt.Errorf("invalid SSO authorize URL: %w", err)
	}
	tokenURL, err := validEndpointURL(cfg.TokenURL)
	if err != nil {
		return nil, fmt.Errorf("invalid SSO token URL: %w", err)
	}
	redirectURL, err := validRedirectURL(cfg.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("invalid SSO redirect URL: %w", err)
	}
	if client == nil {
		timeout := cfg.RequestTimeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Client{
		authorizeURL: authorizeURL.String(), tokenURL: tokenURL.String(),
		issuer: strings.TrimSpace(cfg.Issuer), clientID: strings.TrimSpace(cfg.ClientID),
		clientSecret: strings.TrimSpace(cfg.ClientSecret), redirectURL: redirectURL.String(), http: client,
	}, nil
}

func (c *Client) AuthorizationURL(state string) (string, error) {
	if c == nil {
		return "", errors.New("SSO is disabled")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return "", errors.New("OAuth state is required")
	}
	parsed, err := url.Parse(c.authorizeURL)
	if err != nil {
		return "", ErrMalformed
	}
	query := parsed.Query()
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", c.redirectURL)
	query.Set("response_type", "code")
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) Exchange(ctx context.Context, code string) (Identity, error) {
	if c == nil {
		return Identity{}, errors.New("SSO is disabled")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return Identity{}, ErrRejected
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("redirect_uri", c.redirectURL)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	body, readErr := readResponse(response)
	if readErr != nil {
		return Identity{}, readErr
	}
	var envelope struct {
		Issuer      string `json:"issuer"`
		Subject     string `json:"subject"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Data        *struct {
			Issuer      string `json:"issuer"`
			Subject     string `json:"subject"`
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Identity{}, ErrMalformed
	}
	identity := Identity{Issuer: envelope.Issuer, Subject: envelope.Subject, Username: envelope.Username, DisplayName: envelope.DisplayName, Email: envelope.Email}
	if envelope.Data != nil {
		if identity.Issuer == "" {
			identity.Issuer = envelope.Data.Issuer
		}
		if identity.Subject == "" {
			identity.Subject = envelope.Data.Subject
		}
		if identity.Username == "" {
			identity.Username = envelope.Data.Username
		}
		if identity.DisplayName == "" {
			identity.DisplayName = envelope.Data.DisplayName
		}
		if identity.Email == "" {
			identity.Email = envelope.Data.Email
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Identity{}, ErrRejected
	}
	if strings.TrimSpace(identity.Issuer) != c.issuer || strings.TrimSpace(identity.Subject) == "" {
		return Identity{}, ErrMalformed
	}
	identity.Issuer = strings.TrimSpace(identity.Issuer)
	identity.Subject = strings.TrimSpace(identity.Subject)
	identity.Username = strings.TrimSpace(identity.Username)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	identity.Email = strings.TrimSpace(identity.Email)
	return identity, nil
}

func readResponse(response *http.Response) ([]byte, error) {
	if response == nil || response.Body == nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, ErrUnavailable
	}
	if len(body) > maxResponseBytes {
		return nil, ErrMalformed
	}
	return body, nil
}

func validEndpointURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("must be an absolute http(s) URL without credentials or fragment")
	}
	return parsed, nil
}

func validRedirectURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("must be an absolute http(s) URL without credentials or fragment")
	}
	return parsed, nil
}
