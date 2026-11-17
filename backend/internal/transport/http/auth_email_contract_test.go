package httpapi

import (
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

func TestEmailPasswordLoginKeepsOriginalIdentityAndCredentialChecks(t *testing.T) {
	app := newTestApp(t, nil)
	user := app.createUser(t, 1, "email-owner", "user", "StrongPassword123!")
	if err := app.db.Model(&user).Update("email", " Person@Example.com ").Error; err != nil {
		t.Fatal(err)
	}
	for _, identifier := range []string{"email-owner", " person@EXAMPLE.COM "} {
		result := app.request(t, http.MethodPost, "/api/v1/core/auth/login", "", 0, map[string]any{"username": identifier, "password": "StrongPassword123!"})
		requireStatus(t, result, http.StatusOK)
		data := decodeData(t, result).(map[string]any)
		me := app.request(t, http.MethodGet, "/api/v1/core/auth/me", data["token"].(string), 0, nil)
		requireStatus(t, me, http.StatusOK)
		actor := decodeData(t, me).(map[string]any)
		if actor["username"] != user.Username {
			t.Fatalf("wrong login identity: %v", actor["username"])
		}
	}
	for _, input := range []map[string]any{
		{"username": "person@example.com", "password": "wrong-password"},
		{"username": "unknown@example.com", "password": "StrongPassword123!"},
		{"username": "", "password": "StrongPassword123!"},
	} {
		requireStatus(t, app.request(t, http.MethodPost, "/api/v1/core/auth/login", "", 0, input), http.StatusUnauthorized)
	}
	other := app.createUser(t, 2, "another-owner", "user", "StrongPassword123!")
	if err := app.db.Model(&other).Update("email", "person@example.com").Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/core/auth/login", "", 0, map[string]any{"username": "person@example.com", "password": "StrongPassword123!"}), http.StatusUnauthorized)
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/core/auth/login", "", 0, map[string]any{"username": user.Username, "password": "StrongPassword123!"}), http.StatusOK)
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", other.ID).Updates(map[string]any{"email": "", "username": "person@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/core/auth/login", "", 0, map[string]any{"username": "person@example.com", "password": "StrongPassword123!"}), http.StatusUnauthorized)
}
