package toolruntimehttp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
)

type deviceHTTPFixture struct {
	engine        *gin.Engine
	db            *gorm.DB
	service       *toolruntime.DeviceAuthorizationService
	oauth         *toolruntime.OAuthService
	credentials   *toolruntime.CredentialService
	principal     toolruntime.ManagementPrincipal
	configuration toolruntime.OAuthServerConfiguration
}

func newDeviceHTTPFixture(t *testing.T) deviceHTTPFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := []any{
		&toolruntimegorm.ExternalAuthorizationGrant{}, &toolruntimegorm.ExternalAuthorizationRequest{},
		&toolruntimegorm.ExternalToolingWorkspaceConfiguration{},
	}
	records = append(records, toolruntimegorm.ExternalCredentialMigrationRecords()...)
	records = append(records, toolruntimegorm.OAuthMigrationRecords()...)
	records = append(records, toolruntimegorm.DeviceAuthorizationMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{managementDefinition("project.list", "project:read", toolruntime.LifecycleActive)})
	if err != nil {
		t.Fatal(err)
	}
	requests := toolruntimegorm.NewAuthorizationRequestRepository(db)
	grants := toolruntimegorm.NewGrantRepository(db)
	configurations := toolruntimegorm.NewWorkspaceConfigurationRepository(db)
	principal := toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "研发"}
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := configurations.Save(context.Background(), toolruntime.WorkspaceConfiguration{
		WorkspaceID: principal.WorkspaceID, Mode: toolruntime.WorkspaceConfigurationWindow,
		AllowedTools: []string{"project.list"}, AllowedScopes: []string{"project:read"},
		EffectiveFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour), UpdatedBy: principal.PrincipalID,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	authorizations, err := toolruntime.NewAuthorizationService(catalog, requests, grants, configurations)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := toolruntime.NewCredentialService(
		grants, requests, toolruntimegorm.NewCredentialRepository(db),
		toolruntime.WorkspaceMembershipVerifierFunc(func(context.Context, uint, uint) error { return nil }),
		toolruntime.DefaultCredentialPolicy(),
	)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := toolruntime.NewOAuthServerConfiguration("http://localhost:8081")
	if err != nil {
		t.Fatal(err)
	}
	oauth, err := toolruntime.NewOAuthService(authorizations, credentials, toolruntimegorm.NewOAuthClientRepository(db), configuration)
	if err != nil {
		t.Fatal(err)
	}
	device, err := toolruntime.NewDeviceAuthorizationService(
		authorizations, credentials, toolruntimegorm.NewDeviceAuthorizationRepository(db), configuration,
		toolruntime.DefaultDeviceAuthorizationPolicy(),
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewCredentialHandler(credentials, ManagementPrincipalResolverFunc(func(*gin.Context) (toolruntime.ManagementPrincipal, error) {
		return principal, nil
	}), oauth).WithDeviceAuthorization(device)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler.RegisterManagementRoutes(engine.Group("/api/v1/platform"))
	handler.RegisterPublicRoutes(engine.Group("/api/v1/tool-auth"))
	return deviceHTTPFixture{
		engine: engine, db: db, service: device, oauth: oauth, credentials: credentials,
		principal: principal, configuration: configuration,
	}
}

func (fixture deviceHTTPFixture) startDevice(t *testing.T) (string, string) {
	t.Helper()
	response := fixture.form("/api/v1/tool-auth/device", url.Values{
		"client_id": {toolruntime.OAuthClientCLI}, "workspace_id": {"11"},
		"resource": {fixture.configuration.CLIResource}, "scope": {toolruntime.OAuthScopeTools},
	})
	var payload struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil || payload.DeviceCode == "" || payload.UserCode == "" {
		t.Fatalf("start status=%d body=%s", response.Code, response.Body.String())
	}
	return payload.DeviceCode, payload.UserCode
}

func (fixture deviceHTTPFixture) form(target string, form url.Values) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	fixture.engine.ServeHTTP(response, request)
	return response
}

func (fixture deviceHTTPFixture) json(method, target, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	fixture.engine.ServeHTTP(response, request)
	return response
}

func TestDeviceAuthorizationHTTPResumesWithoutLoopback(t *testing.T) {
	fixture := newDeviceHTTPFixture(t)
	started := fixture.form("/api/v1/tool-auth/device", url.Values{
		"client_id": {toolruntime.OAuthClientCLI}, "workspace_id": {"11"},
		"resource": {fixture.configuration.CLIResource}, "scope": {toolruntime.OAuthScopeTools + " " + toolruntime.OAuthScopeOfflineAccess},
	})
	if started.Code != http.StatusOK || started.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("start status=%d headers=%v body=%s", started.Code, started.Header(), started.Body.String())
	}
	var startPayload struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int64  `json:"expires_in"`
		Interval                int64  `json:"interval"`
	}
	if err := json.Unmarshal(started.Body.Bytes(), &startPayload); err != nil || startPayload.DeviceCode == "" || startPayload.UserCode == "" ||
		startPayload.VerificationURI != "http://localhost:8081/workspace/mcp-cli/device" ||
		!strings.Contains(startPayload.VerificationURIComplete, "user_code=") || startPayload.ExpiresIn != 600 || startPayload.Interval != 5 {
		t.Fatalf("start payload=%+v err=%v", startPayload, err)
	}
	tokenForm := url.Values{
		"grant_type": {toolruntime.DeviceAuthorizationGrantType}, "device_code": {startPayload.DeviceCode},
		"client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
	}
	pending := fixture.form("/api/v1/tool-auth/token", tokenForm)
	if pending.Code != http.StatusBadRequest || !strings.Contains(pending.Body.String(), `"error":"authorization_pending"`) {
		t.Fatalf("pending status=%d body=%s", pending.Code, pending.Body.String())
	}
	slow := fixture.form("/api/v1/tool-auth/token", tokenForm)
	if slow.Code != http.StatusBadRequest || !strings.Contains(slow.Body.String(), `"error":"slow_down"`) {
		t.Fatalf("slow status=%d body=%s", slow.Code, slow.Body.String())
	}
	for attempt := 0; attempt < 12; attempt++ {
		capped := fixture.form("/api/v1/tool-auth/token", tokenForm)
		if capped.Code != http.StatusBadRequest || !strings.Contains(capped.Body.String(), `"error":"slow_down"`) {
			t.Fatalf("capped slow_down attempt=%d status=%d body=%s", attempt, capped.Code, capped.Body.String())
		}
	}
	review := fixture.json(http.MethodGet, "/api/v1/platform/external-tooling/device-authorizations?user_code="+url.QueryEscape(startPayload.UserCode), "")
	if review.Code != http.StatusOK || !strings.Contains(review.Body.String(), `"status":"pending"`) ||
		!strings.Contains(review.Body.String(), `"id":11`) || !strings.Contains(review.Body.String(), `"name":"project.list"`) {
		t.Fatalf("review status=%d body=%s", review.Code, review.Body.String())
	}
	approved := fixture.json(http.MethodPost, "/api/v1/platform/external-tooling/device-authorizations",
		`{"user_code":"`+startPayload.UserCode+`","decision":"authorize"}`)
	if approved.Code != http.StatusOK || !strings.Contains(approved.Body.String(), `"status":"approved"`) ||
		strings.Contains(approved.Body.String(), "authorization_redirect") {
		t.Fatalf("approve status=%d body=%s", approved.Code, approved.Body.String())
	}
	exchanged := fixture.form("/api/v1/tool-auth/token", tokenForm)
	if exchanged.Code != http.StatusOK || !strings.Contains(exchanged.Body.String(), `"token_type":"Bearer"`) ||
		!strings.Contains(exchanged.Body.String(), `"refresh_token"`) {
		t.Fatalf("exchange status=%d body=%s", exchanged.Code, exchanged.Body.String())
	}
	replayed := fixture.form("/api/v1/tool-auth/token", tokenForm)
	if replayed.Code != http.StatusBadRequest || !strings.Contains(replayed.Body.String(), `"error":"invalid_grant"`) {
		t.Fatalf("replay status=%d body=%s", replayed.Code, replayed.Body.String())
	}
}

func TestDeviceAuthorizationPermanentModeReturnsFiniteCLIWindowAfterPoll(t *testing.T) {
	fixture := newDeviceHTTPFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := toolruntimegorm.NewWorkspaceConfigurationRepository(fixture.db).Save(context.Background(), toolruntime.WorkspaceConfiguration{
		WorkspaceID: fixture.principal.WorkspaceID, Mode: toolruntime.WorkspaceConfigurationPermanent,
		AllowedTools: []string{"project.list"}, AllowedScopes: []string{"project:read"}, CLIGrantTTLHours: 3,
		UpdatedBy: fixture.principal.PrincipalID, CreatedAt: now.Add(-time.Hour), UpdatedAt: now, Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	deviceCode, userCode := fixture.startDevice(t)
	tokenForm := url.Values{
		"grant_type": {toolruntime.DeviceAuthorizationGrantType}, "device_code": {deviceCode},
		"client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
	}
	pending := fixture.form("/api/v1/tool-auth/token", tokenForm)
	if pending.Code != http.StatusBadRequest || !strings.Contains(pending.Body.String(), `"error":"authorization_pending"`) {
		t.Fatalf("pending status=%d body=%s", pending.Code, pending.Body.String())
	}
	type reviewWindow struct {
		Data struct {
			Mode          string    `json:"mode"`
			Status        string    `json:"status"`
			EffectiveFrom time.Time `json:"effective_from"`
			ExpiresAt     time.Time `json:"expires_at"`
		} `json:"data"`
	}
	decodeWindow := func(response *httptest.ResponseRecorder) reviewWindow {
		t.Helper()
		var payload reviewWindow
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil ||
			payload.Data.Mode != string(toolruntime.WorkspaceConfigurationPermanent) || payload.Data.EffectiveFrom.IsZero() ||
			payload.Data.ExpiresAt.Sub(payload.Data.EffectiveFrom) != 3*time.Hour {
			t.Fatalf("review status=%d body=%s", response.Code, response.Body.String())
		}
		return payload
	}
	review := decodeWindow(fixture.json(http.MethodGet,
		"/api/v1/platform/external-tooling/device-authorizations?user_code="+url.QueryEscape(userCode), ""))
	if review.Data.Status != string(toolruntime.DeviceAuthorizationPending) {
		t.Fatalf("review status=%q", review.Data.Status)
	}
	approved := decodeWindow(fixture.json(http.MethodPost, "/api/v1/platform/external-tooling/device-authorizations",
		`{"user_code":"`+userCode+`","decision":"authorize"}`))
	if approved.Data.Status != string(toolruntime.DeviceAuthorizationApproved) {
		t.Fatalf("approved status=%q", approved.Data.Status)
	}
	var grant toolruntimegorm.ExternalAuthorizationGrant
	if err := fixture.db.Order("id DESC").First(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if grant.ExpiresAt.Sub(grant.EffectiveFrom) != 3*time.Hour ||
		absDuration(grant.EffectiveFrom.Sub(approved.Data.EffectiveFrom)) >= time.Second {
		t.Fatalf("approved window=%s..%s grant=%s..%s", approved.Data.EffectiveFrom, approved.Data.ExpiresAt,
			grant.EffectiveFrom, grant.ExpiresAt)
	}
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func TestPMCLILoopbackAuthorizationCodeAndRefreshRemainCompatible(t *testing.T) {
	fixture := newDeviceHTTPFixture(t)
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	digest := sha256.Sum256([]byte(verifier))
	redirectURI := "http://127.0.0.1:43177/callback"
	request, err := fixture.oauth.StartAuthorization(context.Background(), fixture.principal, toolruntime.OAuthAuthorizationStartInput{
		ResponseType: "code", ClientID: toolruntime.OAuthClientCLI, RedirectURI: redirectURI,
		PKCEChallenge: base64.RawURLEncoding.EncodeToString(digest[:]), PKCEChallengeMethod: toolruntime.PKCEMethodS256,
		State: "cli-loopback-state", Resource: fixture.configuration.CLIResource,
		Scope: toolruntime.OAuthScopeTools + " " + toolruntime.OAuthScopeOfflineAccess, Channel: toolruntime.DeliveryCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := fixture.oauth.CompleteAuthorization(context.Background(), fixture.principal, request.ID, toolruntime.AuthorizationAuthorize)
	if err != nil {
		t.Fatal(err)
	}
	callback, err := url.Parse(completion.RedirectURL)
	if err != nil || callback.Query().Get("state") != "cli-loopback-state" || callback.Query().Get("code") == "" {
		t.Fatalf("callback=%q err=%v", completion.RedirectURL, err)
	}
	exchanged := fixture.form("/api/v1/tool-auth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {callback.Query().Get("code")}, "code_verifier": {verifier},
		"redirect_uri": {redirectURI}, "client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
	})
	if exchanged.Code != http.StatusOK {
		t.Fatalf("authorization_code status=%d body=%s", exchanged.Code, exchanged.Body.String())
	}
	var tokens struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(exchanged.Body.Bytes(), &tokens); err != nil || tokens.RefreshToken == "" {
		t.Fatalf("tokens=%+v err=%v", tokens, err)
	}
	refreshed := fixture.form("/api/v1/tool-auth/token", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {tokens.RefreshToken},
		"client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
	})
	if refreshed.Code != http.StatusOK || !strings.Contains(refreshed.Body.String(), `"access_token"`) {
		t.Fatalf("refresh status=%d body=%s", refreshed.Code, refreshed.Body.String())
	}
}

func TestDeviceAuthorizationTokenErrorsAndMCPIsolation(t *testing.T) {
	t.Run("denied", func(t *testing.T) {
		fixture := newDeviceHTTPFixture(t)
		deviceCode, userCode := fixture.startDevice(t)
		denied := fixture.json(http.MethodPost, "/api/v1/platform/external-tooling/device-authorizations",
			`{"user_code":"`+userCode+`","decision":"deny"}`)
		if denied.Code != http.StatusOK || !strings.Contains(denied.Body.String(), `"status":"denied"`) {
			t.Fatalf("deny status=%d body=%s", denied.Code, denied.Body.String())
		}
		response := fixture.form("/api/v1/tool-auth/token", url.Values{
			"grant_type": {toolruntime.DeviceAuthorizationGrantType}, "device_code": {deviceCode},
			"client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
		})
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error":"access_denied"`) {
			t.Fatalf("denied token status=%d body=%s", response.Code, response.Body.String())
		}
	})
	t.Run("expired", func(t *testing.T) {
		fixture := newDeviceHTTPFixture(t)
		deviceCode, _ := fixture.startDevice(t)
		if err := fixture.db.Model(&toolruntimegorm.ExternalDeviceAuthorization{}).
			Where("status = ?", toolruntime.DeviceAuthorizationPending).
			Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
		response := fixture.form("/api/v1/tool-auth/token", url.Values{
			"grant_type": {toolruntime.DeviceAuthorizationGrantType}, "device_code": {deviceCode},
			"client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
		})
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error":"expired_token"`) {
			t.Fatalf("expired token status=%d body=%s", response.Code, response.Body.String())
		}
	})
	t.Run("mcp client cannot use device flow", func(t *testing.T) {
		fixture := newDeviceHTTPFixture(t)
		client, err := fixture.oauth.RegisterClient(context.Background(), toolruntime.OAuthClientRegistrationInput{
			Name: "Goose", RedirectURIs: []string{"http://127.0.0.1/oauth_callback"},
		})
		if err != nil {
			t.Fatal(err)
		}
		response := fixture.form("/api/v1/tool-auth/device", url.Values{
			"client_id": {client.ID}, "workspace_id": {"11"},
			"resource": {fixture.configuration.MCPResource}, "scope": {toolruntime.OAuthScopeTools},
		})
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error":"invalid_request"`) {
			t.Fatalf("MCP device start status=%d body=%s", response.Code, response.Body.String())
		}
	})
}

func TestDeviceAuthorizationWorksAfterCLIRevokeAndRelogin(t *testing.T) {
	fixture := newDeviceHTTPFixture(t)
	var consumedBefore int64
	if err := fixture.db.Model(&toolruntimegorm.ExternalDeviceAuthorization{}).
		Where("status = ?", toolruntime.DeviceAuthorizationConsumed).Count(&consumedBefore).Error; err != nil {
		t.Fatal(err)
	}
	login := func() (string, string) {
		deviceCode, userCode := fixture.startDevice(t)
		approved := fixture.json(http.MethodPost, "/api/v1/platform/external-tooling/device-authorizations",
			`{"user_code":"`+userCode+`","decision":"authorize"}`)
		if approved.Code != http.StatusOK || !strings.Contains(approved.Body.String(), `"status":"approved"`) {
			t.Fatalf("approve status=%d body=%s", approved.Code, approved.Body.String())
		}
		exchanged := fixture.form("/api/v1/tool-auth/token", url.Values{
			"grant_type": {toolruntime.DeviceAuthorizationGrantType}, "device_code": {deviceCode},
			"client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
		})
		var tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if exchanged.Code != http.StatusOK || json.Unmarshal(exchanged.Body.Bytes(), &tokens) != nil ||
			tokens.AccessToken == "" || tokens.RefreshToken == "" {
			t.Fatalf("exchange status=%d body=%s", exchanged.Code, exchanged.Body.String())
		}
		return deviceCode, tokens.RefreshToken
	}
	firstCode, firstRefresh := login()
	revoked := fixture.form("/api/v1/tool-auth/revoke", url.Values{
		"token": {firstRefresh}, "token_type_hint": {"refresh_token"},
		"client_id": {toolruntime.OAuthClientCLI}, "resource": {fixture.configuration.CLIResource},
	})
	if revoked.Code != http.StatusOK {
		t.Fatalf("revoke status=%d body=%s", revoked.Code, revoked.Body.String())
	}
	secondCode, _ := login()
	if secondCode == firstCode {
		t.Fatal("re-login reused the prior device code")
	}
	var consumed, pending, approving int64
	for status, target := range map[toolruntime.DeviceAuthorizationStatus]*int64{
		toolruntime.DeviceAuthorizationConsumed:            &consumed,
		toolruntime.DeviceAuthorizationPending:             &pending,
		toolruntime.DeviceAuthorizationStatus("approving"): &approving,
	} {
		if err := fixture.db.Model(&toolruntimegorm.ExternalDeviceAuthorization{}).
			Where("status = ?", status).Count(target).Error; err != nil {
			t.Fatal(err)
		}
	}
	if consumed != consumedBefore+2 || pending != 0 || approving != 0 {
		t.Fatalf("unexpected device states consumed=%d pending=%d approving=%d", consumed, pending, approving)
	}
}
