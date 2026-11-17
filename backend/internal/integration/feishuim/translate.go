package feishuim

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

const maxTranslateResponseBytes = 64 << 10

// OpenIDTranslator rewrites an open_id issued by a previous Feishu application
// into the current login/IM application's open_id. The intermediate Feishu
// identifier is not persisted.
type OpenIDTranslator struct {
	previousAppID     string
	previousAppSecret string
	currentAppID      string
	currentAppSecret  string
	baseURL           string
	http              *http.Client
}

func NewOpenIDTranslator(cfg config.FeishuLogin, timeout time.Duration) (*OpenIDTranslator, error) {
	previousID := strings.TrimSpace(cfg.PreviousAppID)
	previousSecret := strings.TrimSpace(cfg.PreviousAppSecret)
	if previousID == "" && previousSecret == "" {
		return nil, nil
	}
	if previousID == "" || previousSecret == "" {
		return nil, errors.New("previous Feishu login application requires both app id and secret")
	}
	currentID := strings.TrimSpace(cfg.AppID)
	currentSecret := strings.TrimSpace(cfg.AppSecret)
	if currentID == "" || currentSecret == "" {
		return nil, errors.New("current Feishu login application is required to translate open_id")
	}
	if previousID == currentID {
		return nil, nil
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = "https://open.feishu.cn"
	}
	return &OpenIDTranslator{
		previousAppID: previousID, previousAppSecret: previousSecret,
		currentAppID: currentID, currentAppSecret: currentSecret,
		baseURL: base, http: &http.Client{Timeout: timeout},
	}, nil
}

func (t *OpenIDTranslator) TranslateToCurrent(ctx context.Context, sourceOpenID string) (string, error) {
	if t == nil {
		return "", errors.New("Feishu open_id translator is not configured")
	}
	sourceOpenID = strings.TrimSpace(sourceOpenID)
	if sourceOpenID == "" {
		return "", errors.New("source Feishu open_id is required")
	}
	bridgeID, err := t.lookupBridgeID(ctx, t.previousAppID, t.previousAppSecret, sourceOpenID, "open_id")
	if err != nil {
		return "", err
	}
	openID, err := t.lookupOpenID(ctx, t.currentAppID, t.currentAppSecret, bridgeID, "union_id")
	if err != nil {
		return "", err
	}
	if openID == "" || openID == sourceOpenID {
		return "", errors.New("previous Feishu open_id could not be rewritten for the current application")
	}
	return openID, nil
}

func (t *OpenIDTranslator) lookupBridgeID(ctx context.Context, appID, appSecret, userID, idType string) (string, error) {
	user, err := t.contactUser(ctx, appID, appSecret, userID, idType)
	if err != nil {
		return "", err
	}
	bridgeID := strings.TrimSpace(user.UnionID)
	if bridgeID == "" {
		return "", errors.New("previous Feishu application did not return a translatable identity")
	}
	return bridgeID, nil
}

func (t *OpenIDTranslator) lookupOpenID(ctx context.Context, appID, appSecret, userID, idType string) (string, error) {
	user, err := t.contactUser(ctx, appID, appSecret, userID, idType)
	if err != nil {
		return "", err
	}
	openID := strings.TrimSpace(user.OpenID)
	if openID == "" {
		return "", errors.New("current Feishu application did not return an open_id")
	}
	return openID, nil
}

type contactUser struct {
	OpenID  string `json:"open_id"`
	UnionID string `json:"union_id"`
}

func (t *OpenIDTranslator) contactUser(ctx context.Context, appID, appSecret, userID, idType string) (contactUser, error) {
	token, err := t.tenantToken(ctx, appID, appSecret)
	if err != nil {
		return contactUser{}, err
	}
	endpoint, err := url.Parse(t.baseURL + "/open-apis/contact/v3/users/" + url.PathEscape(userID))
	if err != nil {
		return contactUser{}, err
	}
	query := endpoint.Query()
	query.Set("user_id_type", idType)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return contactUser{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	body, err := t.doJSON(request)
	if err != nil {
		return contactUser{}, err
	}
	var payload struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			User contactUser `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return contactUser{}, err
	}
	if payload.Code != 0 {
		return contactUser{}, fmt.Errorf("feishu contact code %d: %s", payload.Code, payload.Msg)
	}
	return payload.Data.User, nil
}

func (t *OpenIDTranslator) tenantToken(ctx context.Context, appID, appSecret string) (string, error) {
	raw, err := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.baseURL+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	body, err := t.doJSON(request)
	if err != nil {
		return "", err
	}
	var payload struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Code != 0 || strings.TrimSpace(payload.TenantAccessToken) == "" {
		return "", fmt.Errorf("feishu tenant token code %d", payload.Code)
	}
	return strings.TrimSpace(payload.TenantAccessToken), nil
}

func (t *OpenIDTranslator) doJSON(request *http.Request) ([]byte, error) {
	response, err := t.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTranslateResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxTranslateResponseBytes {
		return nil, errors.New("feishu contact response is too large")
	}
	return body, nil
}
