package identityhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type passwordResetHTTPRepository struct {
	tokenHash    string
	passwordHash string
}

func (*passwordResetHTTPRepository) CreatePasswordReset(context.Context, identity.PasswordResetRequest) error {
	return nil
}

func (repository *passwordResetHTTPRepository) CompletePasswordReset(_ context.Context, tokenHash, passwordHash string, _ time.Time) error {
	repository.tokenHash, repository.passwordHash = tokenHash, passwordHash
	return nil
}

type accountHTTPRepository struct{ account identity.Account }

func (*accountHTTPRepository) CreateExternalRegistration(context.Context, *identity.ExternalRegistration) error {
	return nil
}
func (*accountHTTPRepository) CompleteExternalRegistration(context.Context, identity.ExternalRegistrationComplete) (identity.User, error) {
	return identity.User{}, nil
}
func (r *accountHTTPRepository) Account(context.Context, uint) (identity.Account, error) {
	return r.account, nil
}
func (*accountHTTPRepository) UpdateProfile(context.Context, uint, string) (identity.User, error) {
	return identity.User{}, nil
}
func (*accountHTTPRepository) UpdatePassword(context.Context, uint, string, string) error { return nil }
func (*accountHTTPRepository) UnbindIdentity(context.Context, uint, string) error         { return nil }
func (*accountHTTPRepository) CreateIdentityOperation(context.Context, *identity.IdentityOperation) error {
	return nil
}
func (*accountHTTPRepository) ConsumeIdentityOperation(context.Context, string, string, time.Time) (identity.IdentityOperation, error) {
	return identity.IdentityOperation{}, nil
}
func (*accountHTTPRepository) BindIdentity(context.Context, uint, string, identity.GoogleIdentity, identity.TelegramIdentity) error {
	return nil
}

func TestAccountHandlerReturnsFullUserDTO(t *testing.T) {
	repo := &accountHTTPRepository{account: identity.Account{User: identity.User{ID: 7, Username: "alice", DisplayName: "Alice", Email: "a@example.com", Active: true}, DisplayNameEditable: true, PasswordConfigured: true, Identities: []identity.LoginIdentity{{Provider: identity.FlowGoogle, Email: "a@example.com"}}}}
	service := identity.NewAccountService(repo, nil, false)
	handler := NewAccountHandler(service)
	engine := gin.New()
	group := engine.Group("")
	group.Use(func(c *gin.Context) {
		c.Set(pmauth.PrincipalContextKey, pmauth.PMPrincipal(7, "alice", "Alice", "a@example.com"))
		c.Next()
	})
	handler.RegisterAuthenticatedRoutes(group)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth/me/account", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			User               identity.User         `json:"user"`
			EmailBinding       identity.EmailBinding `json:"email_binding"`
			PasswordConfigured bool                  `json:"password_configured"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.EmailBinding.Email != "a@example.com" || !payload.Data.EmailBinding.Verified || payload.Data.EmailBinding.Available {
		t.Fatalf("binding=%+v", payload.Data.EmailBinding)
	}
	if payload.Data.User.ID != 7 || payload.Data.User.Username != "alice" || !payload.Data.PasswordConfigured {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestProviderLoginMapsDisabledRegistrationToStableForbidden(t *testing.T) {
	for name, failure := range map[string]func(*gin.Context, error){"google": googleLoginFailure, "telegram": telegramLoginFailure} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			failure(c, identity.ErrExternalRegistrationDisabled)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var body map[string]string
			_ = json.Unmarshal(response.Body.Bytes(), &body)
			if body["code"] != "AUTH_LOGIN_USER_UNAVAILABLE" {
				t.Fatalf("body=%v", body)
			}
		})
	}
}

func TestPasswordResetCompletionIsPublicAndDoesNotRequireCurrentPassword(t *testing.T) {
	repository := &passwordResetHTTPRepository{}
	service := identity.NewPasswordResetService(repository, identity.PasswordResetOptions{
		PublicBaseURL: "https://pm.test",
		Random:        bytes.NewReader(bytes.Repeat([]byte{1}, 32)),
	})
	handler := NewAccountHandler(nil)
	handler.SetPasswordResetService(service)
	engine := gin.New()
	handler.RegisterPublicRoutes(engine.Group("/api/v1/core"))

	body := bytes.NewBufferString(`{"token":"one-time-token","new_password":"new-password-value","new_password_confirmation":"new-password-value"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/core/auth/password-reset/complete", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repository.tokenHash == "" || bcrypt.CompareHashAndPassword([]byte(repository.passwordHash), []byte("new-password-value")) != nil {
		t.Fatal("password reset contract did not persist the new password")
	}
}
