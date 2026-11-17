package httpapi

import (
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestLocalAuthAndHealthContract(t *testing.T) {
	app := newTestApp(t, nil)
	user := app.createUser(t, 1, "admin", "super_admin", "StrongPassword123!")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	if err := app.db.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: 1, UserID: user.ID, Role: "owner", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	ready := app.request(t, http.MethodGet, "/health/ready", "", 0, nil)
	requireStatus(t, ready, http.StatusOK)

	login := app.request(t, http.MethodPost, "/api/v1/core/auth/login", "", 0, map[string]any{
		"username": "admin", "password": "StrongPassword123!",
	})
	requireStatus(t, login, http.StatusOK)
	data := decodeData(t, login).(map[string]any)
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatalf("login did not return token: %v", data)
	}
	me := app.request(t, http.MethodGet, "/core/auth/me", token, 0, nil)
	requireStatus(t, me, http.StatusOK)
	if got := decodeData(t, me).(map[string]any)["username"]; got != "admin" {
		t.Fatalf("username=%v", got)
	}
	for _, path := range []string{"/api/v1/core/auth/caps", "/api/v1/core/auth/rbac-effective"} {
		retired := app.request(t, http.MethodGet, path, token, workspace.ID, nil)
		requireStatus(t, retired, http.StatusNotFound)
	}
	logout := app.request(t, http.MethodPost, "/api/v1/core/auth/logout", token, 0, nil)
	requireStatus(t, logout, http.StatusNoContent)
}

func TestProductionLeavesDatabaseSessionSecretUnresolvedAtConfigLoad(t *testing.T) {
	t.Setenv("PM_ENV", "production")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_FEISHU_LOGIN_APP_ID", "")
	t.Setenv("PM_FEISHU_LOGIN_APP_SECRET", "")
	t.Setenv("PM_LOCAL_JWT_SECRET", "environment-secret-that-is-at-least-thirty-two-characters")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocalJWTSecret != "" {
		t.Fatal("production config read the session secret from environment")
	}
}
