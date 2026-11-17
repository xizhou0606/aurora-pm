package identityhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

type handlerGoogleProvider struct {
	state string
	nonce string
}

func (provider *handlerGoogleProvider) AuthorizationURL(state, nonce string) (string, error) {
	provider.state, provider.nonce = state, nonce
	values := url.Values{"state": {state}, "nonce": {nonce}}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + values.Encode(), nil
}

func (provider *handlerGoogleProvider) Exchange(context.Context, string) (identity.GoogleIdentity, error) {
	return identity.GoogleIdentity{
		Issuer: "https://accounts.google.com", Subject: "google-alice",
		Email: "alice@example.com", EmailVerified: true, Nonce: provider.nonce,
	}, nil
}

func TestGoogleLoginHandlerRedirectsThroughProviderAndReturnsOneTimeLoginCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&identitygorm.User{}, &identitygorm.LoginTransaction{}, &identitygorm.LoginCode{}, &identitygorm.UserLoginIdentity{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.User{ExternalSubject: "legacy-alice", Username: "alice", Email: "alice@example.com", Active: true}).Error; err != nil {
		t.Fatal(err)
	}
	var alice identitygorm.User
	if err := db.Where("username = ?", "alice").First(&alice).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.UserLoginIdentity{UserID: alice.ID, Provider: identity.FlowGoogle, Issuer: "https://accounts.google.com", Subject: "google-alice", EmailSnapshot: "alice@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := identitygorm.NewLoginRepository(db)
	login := identity.NewLoginService(repository, identity.LoginOptions{})
	provider := &handlerGoogleProvider{}
	engine := gin.New()
	NewGoogleLoginHandler(identity.NewGoogleLoginService(login, provider)).RegisterRoutes(engine.Group("/api/v1/core"))

	start := httptest.NewRecorder()
	engine.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/api/v1/core/auth/google/start?return_to=%2Flogin%3Fredirect%3D%252Fworkspace%252Fprojects", nil))
	if start.Code != http.StatusFound || provider.state == "" || provider.nonce == "" {
		t.Fatalf("start status=%d location=%q", start.Code, start.Header().Get("Location"))
	}

	callback := httptest.NewRecorder()
	callbackPath := "/api/v1/core/auth/google/callback?state=" + url.QueryEscape(provider.state) + "&code=provider-code"
	engine.ServeHTTP(callback, httptest.NewRequest(http.MethodGet, callbackPath, nil))
	if callback.Code != http.StatusFound {
		t.Fatalf("callback status=%d body=%s", callback.Code, callback.Body.String())
	}
	location, err := url.Parse(callback.Header().Get("Location"))
	if err != nil || location.Path != "/login" || location.Query().Get("redirect") != "/workspace/projects" {
		t.Fatalf("callback location=%q err=%v", callback.Header().Get("Location"), err)
	}
	loginCode := location.Query().Get("login_code")
	user, flow, err := login.ExchangeCode(context.Background(), loginCode)
	if err != nil || user.Username != "alice" || flow != identity.FlowGoogle {
		t.Fatalf("exchange user=%+v flow=%q err=%v", user, flow, err)
	}
}

func TestGoogleLoginHandlerReportsUnavailableServiceWithStableCode(t *testing.T) {
	engine := gin.New()
	NewGoogleLoginHandler(nil).RegisterRoutes(engine.Group("/api/v1/core"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/core/auth/google/callback?state=invalid&code=value", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || !strings.EqualFold(body["code"], authUpstreamUnavailable) {
		t.Fatalf("body=%v err=%v", body, err)
	}
}
