package identityhttp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

type handlerTelegramProvider struct {
	state     string
	nonce     string
	challenge string
	verifier  string
}

func (provider *handlerTelegramProvider) AuthorizationURL(state, nonce, challenge string) (string, error) {
	provider.state, provider.nonce, provider.challenge = state, nonce, challenge
	values := url.Values{"state": {state}, "nonce": {nonce}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	return "https://oauth.telegram.org/auth?" + values.Encode(), nil
}

func (provider *handlerTelegramProvider) Exchange(_ context.Context, _ string, verifier string) (identity.TelegramIdentity, error) {
	provider.verifier = verifier
	return identity.TelegramIdentity{
		Issuer: "https://oauth.telegram.org", Subject: "telegram-alice", TelegramUserID: 987654321,
		Name: "Alice", PreferredUsername: "alice_tg", Nonce: provider.nonce,
	}, nil
}

func TestTelegramLoginHandlerCompletesPKCEForPreboundUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&identitygorm.User{}, &identitygorm.LoginTransaction{}, &identitygorm.LoginCode{}, &identitygorm.UserLoginIdentity{}); err != nil {
		t.Fatal(err)
	}
	user := identitygorm.User{ExternalSubject: "legacy-telegram-alice", Username: "alice", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.UserLoginIdentity{
		UserID: user.ID, Provider: identity.FlowTelegram, Issuer: "https://oauth.telegram.org", Subject: "telegram-alice",
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := identitygorm.NewLoginRepository(db)
	login := identity.NewLoginService(repository, identity.LoginOptions{})
	provider := &handlerTelegramProvider{}
	engine := gin.New()
	NewTelegramLoginHandler(identity.NewTelegramLoginService(login, provider)).RegisterRoutes(engine.Group("/api/v1/core"))

	start := httptest.NewRecorder()
	engine.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/api/v1/core/auth/telegram/start?return_to=%2Flogin", nil))
	if start.Code != http.StatusFound || provider.state == "" || provider.nonce == "" || provider.challenge == "" {
		t.Fatalf("start status=%d location=%q", start.Code, start.Header().Get("Location"))
	}
	callback := httptest.NewRecorder()
	engine.ServeHTTP(callback, httptest.NewRequest(http.MethodGet,
		"/api/v1/core/auth/telegram/callback?state="+url.QueryEscape(provider.state)+"&code=authorization-code", nil))
	if callback.Code != http.StatusFound || provider.verifier == "" {
		t.Fatalf("callback status=%d body=%s verifier=%q", callback.Code, callback.Body.String(), provider.verifier)
	}
	location, err := url.Parse(callback.Header().Get("Location"))
	if err != nil || location.Path != "/login" || location.Query().Get("login_code") == "" {
		t.Fatalf("location=%q err=%v", callback.Header().Get("Location"), err)
	}
	resolved, flow, err := login.ExchangeCode(context.Background(), location.Query().Get("login_code"))
	if err != nil || resolved.ID != user.ID || flow != identity.FlowTelegram {
		t.Fatalf("resolved=%+v flow=%q err=%v", resolved, flow, err)
	}
	var binding identitygorm.UserLoginIdentity
	if err := db.Where("provider = ? AND subject = ?", identity.FlowTelegram, "telegram-alice").First(&binding).Error; err != nil {
		t.Fatal(err)
	}
	if binding.TelegramUserID == nil || *binding.TelegramUserID != 987654321 {
		t.Fatalf("binding=%+v", binding)
	}
}
