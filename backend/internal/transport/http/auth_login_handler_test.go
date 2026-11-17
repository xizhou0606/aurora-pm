package httpapi

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishulogin"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/saassso"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type loginRoundTripper func(*http.Request) (*http.Response, error)

func (f loginRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func loginResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func newLoginTestServer(t *testing.T) (*testApp, *Server) {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.FeishuLogin = config.FeishuLogin{AppID: "feishu-app", AppSecret: "feishu-secret", BaseURL: "https://feishu.test", RedirectURL: "https://pm.test/api/v1/core/auth/feishu/callback"}
		cfg.SSO = config.SSO{Enabled: true, AuthorizeURL: "https://sso.test/authorize", TokenURL: "https://sso.test/token", Issuer: "one2all", ClientID: "pm", ClientSecret: "sso-secret", RedirectURL: "https://pm.test/api/v1/core/auth/sso/callback"}
	})
	if err := app.db.Create(&identitygorm.Workspace{
		Name: "Default", Slug: "default", Enabled: true, IsDefault: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	feishuClient, err := feishulogin.NewForTest("https://feishu.test", "https://pm.test/api/v1/core/auth/feishu/callback", "feishu-app", "feishu-secret", &http.Client{Transport: loginRoundTripper(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/open-apis/authen/v2/oauth/token":
			return loginResponse(http.StatusOK, `{"code":0,"data":{"access_token":"provider-token"}}`), nil
		case "/open-apis/authen/v1/user_info":
			if request.Header.Get("Authorization") != "Bearer provider-token" {
				t.Fatalf("Feishu authorization=%q", request.Header.Get("Authorization"))
			}
			return loginResponse(http.StatusOK, `{"code":0,"data":{"open_id":"ou_alice","tenant_key":"tenant_pm","name":"Alice","avatar_url":"https://avatar.test/alice.png"}}`), nil
		default:
			return loginResponse(http.StatusNotFound, "{}"), nil
		}
	})})
	if err != nil {
		t.Fatal(err)
	}
	ssoClient, err := saassso.NewForTest(app.cfg.SSO, &http.Client{Transport: loginRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/token" {
			return loginResponse(http.StatusNotFound, "{}"), nil
		}
		return loginResponse(http.StatusOK, `{"issuer":"one2all","subject":"one2all|7","username":"alice"}`), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{
		cfg: app.cfg, db: app.db, tokens: app.tokens, authCache: map[string]cachedIdentity{},
		loginService: identity.NewLoginService(identitygorm.NewLoginRepository(app.db), identity.LoginOptions{}),
		feishuLogin:  feishuClient, ssoLogin: ssoClient,
	}
	engine := gin.New()
	group := engine.Group("/api/v1/core")
	group.GET("/auth/feishu/start", server.feishuStart)
	group.GET("/auth/feishu/callback", server.feishuCallback)
	group.GET("/auth/sso/start", server.ssoStart)
	group.GET("/auth/sso/callback", server.ssoCallback)
	group.POST("/auth/session/exchange", server.exchangeSession)
	authenticated := group.Group("")
	authenticated.Use(server.authenticate)
	authenticated.GET("/auth/me", server.me)
	app.engine = engine
	return app, server
}

func TestDualLoginEntriesIssueTheSamePMSessionUser(t *testing.T) {
	app, _ := newLoginTestServer(t)
	registerLoginTenant(t, app, "tenant_pm")
	user := identitygorm.User{ID: 7, ExternalIssuer: "one2all", ExternalSubject: "one2all|7", Username: "alice", DisplayName: "Alice", Active: true, Role: "user"}
	if err := app.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	feishuCode := completeLoginCallback(t, app, "/api/v1/core/auth/feishu/start?return_to=/projects", "/api/v1/core/auth/feishu/callback?code=provider-code&state=")
	ssoCode := completeLoginCallback(t, app, "/api/v1/core/auth/sso/start?return_to=/projects", "/api/v1/core/auth/sso/callback?code=sso-code&state=")
	feishuToken := exchangeLoginCode(t, app, feishuCode)
	ssoToken := exchangeLoginCode(t, app, ssoCode)
	for _, token := range []string{feishuToken, ssoToken} {
		me := app.request(t, http.MethodGet, "/api/v1/core/auth/me", token, 0, nil)
		requireStatus(t, me, http.StatusOK)
		if got := decodeData(t, me).(map[string]any)["id"]; got != float64(user.ID) {
			t.Fatalf("session user id=%v want=%d", got, user.ID)
		}
	}
}

func TestLoginStateCodeAndReturnTargetAreOneTimeAndBounded(t *testing.T) {
	app, _ := newLoginTestServer(t)
	registerLoginTenant(t, app, "tenant_already_registered")
	start := app.request(t, http.MethodGet, "/api/v1/core/auth/feishu/start?return_to=https://evil.example/steal", "", 0, nil)
	if start.Code != http.StatusFound {
		t.Fatalf("start status=%d body=%s", start.Code, start.Body.String())
	}
	state := redirectQuery(t, start.Header().Get("Location"), "state")
	callbackPath := "/api/v1/core/auth/feishu/callback?code=provider-code&state=" + url.QueryEscape(state)
	callback := app.request(t, http.MethodGet, callbackPath, "", 0, nil)
	requireStatus(t, callback, http.StatusForbidden)
	if !strings.Contains(callback.Body.String(), apiErrorAuthLoginTenantUnavailable) {
		t.Fatalf("unregistered tenant response=%s", callback.Body.String())
	}
	replay := app.request(t, http.MethodGet, callbackPath, "", 0, nil)
	requireStatus(t, replay, http.StatusUnauthorized)
	if !strings.Contains(replay.Body.String(), apiErrorAuthLoginCodeInvalid) {
		t.Fatalf("replay response=%s", replay.Body.String())
	}
}

func TestSuccessfulCallbackSanitizesMaliciousReturnTarget(t *testing.T) {
	app, _ := newLoginTestServer(t)
	registerLoginTenant(t, app, "tenant_pm")
	start := app.request(t, http.MethodGet, "/api/v1/core/auth/feishu/start?return_to=https://evil.example/steal", "", 0, nil)
	requireStatus(t, start, http.StatusFound)
	state := redirectQuery(t, start.Header().Get("Location"), "state")
	callback := app.request(t, http.MethodGet,
		"/api/v1/core/auth/feishu/callback?code=provider-code&state="+url.QueryEscape(state), "", 0, nil)
	requireStatus(t, callback, http.StatusFound)
	location, err := url.Parse(callback.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Path != "/" || len(location.Query()) != 1 || location.Query().Get("login_code") == "" ||
		location.Query().Get("access_token") != "" || location.Query().Get("token") != "" {
		t.Fatalf("unsafe callback redirect=%q", location.String())
	}
	var user identitygorm.User
	if err := app.db.Where("display_name = ?", "Alice").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	var tenant identitygorm.FeishuTenant
	if err := app.db.Where("tenant_key = ?", "tenant_pm").First(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	if !user.Active || user.Username != identity.FeishuUsername("tenant_pm", "Alice") || user.DisplayName != "Alice" ||
		user.FeishuTenantID == nil || *user.FeishuTenantID != tenant.ID {
		t.Fatalf("JIT user=%+v", user)
	}
	var membership identitygorm.WorkspaceMember
	if err := app.db.Where("user_id = ?", user.ID).First(&membership).Error; err != nil {
		t.Fatalf("JIT user did not join the default workspace: %v", err)
	}
}

func TestFeishuCallbackReusesOpenIDWhenDirectoryNameChanges(t *testing.T) {
	app, _ := newLoginTestServer(t)
	registerLoginTenant(t, app, "tenant_pm")
	var tenant identitygorm.FeishuTenant
	if err := app.db.Where("tenant_key = ?", "tenant_pm").First(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	openID := "ou_alice"
	existing := identitygorm.User{
		ExternalIssuer: "one2all", ExternalSubject: "one2all|renamed", Username: "alice.previous",
		DisplayName: "Alice Previous", FeishuOpenID: &openID, FeishuTenantID: &tenant.ID,
		Active: true, Role: "user", Status: string(identity.AccountActive), SessionVersion: 1,
	}
	if err := app.db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}

	code := completeLoginCallback(t, app,
		"/api/v1/core/auth/feishu/start?return_to=/projects",
		"/api/v1/core/auth/feishu/callback?code=provider-code&state=",
	)
	token := exchangeLoginCode(t, app, code)
	me := app.request(t, http.MethodGet, "/api/v1/core/auth/me", token, 0, nil)
	requireStatus(t, me, http.StatusOK)
	if got := decodeData(t, me).(map[string]any)["id"]; got != float64(existing.ID) {
		t.Fatalf("session user id=%v want=%d", got, existing.ID)
	}
	var users int64
	if err := app.db.Model(&identitygorm.User{}).Where("feishu_open_id = ?", openID).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("users with open_id=%d want=1", users)
	}
}

func TestSessionLoginCodeCannotBeReplayed(t *testing.T) {
	app, _ := newLoginTestServer(t)
	registerLoginTenant(t, app, "tenant_pm")
	code := completeLoginCallback(t, app, "/api/v1/core/auth/feishu/start", "/api/v1/core/auth/feishu/callback?code=provider-code&state=")
	_ = exchangeLoginCode(t, app, code)
	replay := app.request(t, http.MethodPost, "/api/v1/core/auth/session/exchange", "", 0, map[string]string{"login_code": code})
	requireStatus(t, replay, http.StatusUnauthorized)
	if !strings.Contains(replay.Body.String(), apiErrorAuthLoginCodeInvalid) {
		t.Fatalf("replay response=%s", replay.Body.String())
	}
}

func registerLoginTenant(t *testing.T, app *testApp, tenantKey string) {
	t.Helper()
	if err := app.db.Create(&identitygorm.FeishuTenant{TenantKey: tenantKey, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
}

func completeLoginCallback(t *testing.T, app *testApp, startPath, callbackPrefix string) string {
	t.Helper()
	start := app.request(t, http.MethodGet, startPath, "", 0, nil)
	if start.Code != http.StatusFound {
		t.Fatalf("start status=%d body=%s", start.Code, start.Body.String())
	}
	state := redirectQuery(t, start.Header().Get("Location"), "state")
	callback := app.request(t, http.MethodGet, callbackPrefix+url.QueryEscape(state), "", 0, nil)
	if callback.Code != http.StatusFound {
		t.Fatalf("callback status=%d body=%s", callback.Code, callback.Body.String())
	}
	return redirectQuery(t, callback.Header().Get("Location"), "login_code")
}

func exchangeLoginCode(t *testing.T, app *testApp, code string) string {
	t.Helper()
	response := app.request(t, http.MethodPost, "/api/v1/core/auth/session/exchange", "", 0, map[string]string{"login_code": code})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	token, _ := data["access_token"].(string)
	if token == "" {
		t.Fatalf("session response=%s", response.Body.String())
	}
	return token
}

func redirectQuery(t *testing.T, location, key string) string {
	t.Helper()
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	value := parsed.Query().Get(key)
	if value == "" {
		t.Fatalf("redirect %q lacks %q", location, key)
	}
	return value
}
