// Package feishulogin implements PM's standalone Feishu OAuth client.
package feishulogin

import (
	"bytes"
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
	ErrUnavailable = errors.New("feishu login provider unavailable")
	ErrRejected    = errors.New("feishu login provider rejected request")
	ErrMalformed   = errors.New("feishu login provider returned malformed response")
)

type Identity struct {
	OpenID      string
	TenantKey   string
	DisplayName string
	AvatarURL   string
}

type Client struct {
	appID     string
	appSecret string
	baseURL   string
	redirect  string
	http      *http.Client
}

func New(cfg config.FeishuLogin) (*Client, error) {
	return NewWithHTTPClient(cfg, nil)
}

// NewForTest creates a client with an injected transport. It exists
// to keep HTTP adapter tests network-free; production code uses New.
func NewForTest(baseURL, redirectURL, appID, appSecret string, client *http.Client) (*Client, error) {
	return NewWithHTTPClient(config.FeishuLogin{
		BaseURL: baseURL, RedirectURL: redirectURL,
		AppID: appID, AppSecret: appSecret,
	}, client)
}

// NewWithHTTPClient is kept small and explicit so transport behavior can be
// tested without changing production configuration or global HTTP state.
func NewWithHTTPClient(cfg config.FeishuLogin, client *http.Client) (*Client, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" || strings.TrimSpace(cfg.RedirectURL) == "" {
		return nil, errors.New("Feishu login configuration is incomplete")
	}
	base, err := validBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Feishu login base URL: %w", err)
	}
	redirect, err := validRedirectURL(cfg.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Feishu login redirect URL: %w", err)
	}
	if client == nil {
		timeout := cfg.RequestTimeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Client{
		appID: strings.TrimSpace(cfg.AppID), appSecret: strings.TrimSpace(cfg.AppSecret),
		baseURL: strings.TrimRight(base.String(), "/"), redirect: redirect.String(), http: client,
	}, nil
}

func (c *Client) AuthorizationURL(state string) (string, error) {
	if c == nil {
		return "", errors.New("Feishu login is disabled")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return "", errors.New("OAuth state is required")
	}
	parsed, err := url.Parse(c.baseURL + "/open-apis/authen/v1/index")
	if err != nil {
		return "", errors.New("build Feishu authorization URL")
	}
	values := parsed.Query()
	values.Set("app_id", c.appID)
	values.Set("redirect_uri", c.redirect)
	values.Set("state", state)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func (c *Client) Exchange(ctx context.Context, code string) (Identity, error) {
	if c == nil {
		return Identity{}, errors.New("Feishu login is disabled")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return Identity{}, ErrRejected
	}
	payload, err := json.Marshal(map[string]string{
		"grant_type": "authorization_code", "client_id": c.appID,
		"client_secret": c.appSecret, "code": code, "redirect_uri": c.redirect,
	})
	if err != nil {
		return Identity{}, ErrMalformed
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/open-apis/authen/v2/oauth/token", bytes.NewReader(payload))
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	body, readErr := readResponse(response)
	if readErr != nil {
		return Identity{}, readErr
	}
	var token struct {
		Code        int    `json:"code"`
		AccessToken string `json:"access_token"`
		Data        struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return Identity{}, ErrMalformed
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || token.Code != 0 {
		return Identity{}, ErrRejected
	}
	accessToken := strings.TrimSpace(token.AccessToken)
	if accessToken == "" {
		accessToken = strings.TrimSpace(token.Data.AccessToken)
	}
	if accessToken == "" {
		return Identity{}, ErrMalformed
	}
	return c.userInfo(ctx, accessToken)
}

func (c *Client) userInfo(ctx context.Context, accessToken string) (Identity, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/open-apis/authen/v1/user_info", nil)
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := c.http.Do(request)
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	body, readErr := readResponse(response)
	if readErr != nil {
		return Identity{}, readErr
	}
	var profile struct {
		Code int `json:"code"`
		Data struct {
			OpenID    string `json:"open_id"`
			TenantKey string `json:"tenant_key"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		return Identity{}, ErrMalformed
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || profile.Code != 0 {
		return Identity{}, ErrRejected
	}
	openID, tenantKey := strings.TrimSpace(profile.Data.OpenID), strings.TrimSpace(profile.Data.TenantKey)
	if openID == "" || tenantKey == "" {
		return Identity{}, ErrMalformed
	}
	return Identity{
		OpenID: openID, TenantKey: tenantKey,
		DisplayName: strings.TrimSpace(profile.Data.Name), AvatarURL: strings.TrimSpace(profile.Data.AvatarURL),
	}, nil
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

func validBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("must be an absolute http(s) URL without credentials, query, or fragment")
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
