package httpapi

import (
	"net/http"
	"testing"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

func TestAccountAdmissionPrecedesBusinessAuthorization(t *testing.T) {
	app := newTestApp(t, nil)
	user := app.createUser(t, 91, "pending-user", "user", "")
	if err := app.db.Model(&identitygorm.User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"status": identity.AccountPendingApproval,
			"active": true,
		}).Error; err != nil {
		t.Fatal(err)
	}
	token := app.token(t, user)

	me := app.request(t, http.MethodGet, "/api/v1/core/auth/me", token, 0, nil)
	requireStatus(t, me, http.StatusOK)

	blocked := app.request(t, http.MethodGet, "/api/v1/core/workspaces", token, 0, nil)
	requireStatus(t, blocked, http.StatusForbidden)
	assertAccessResponseCode(t, blocked, apiErrorIdentityApprovalPending)

	logout := app.request(t, http.MethodPost, "/api/v1/core/auth/logout", token, 0, nil)
	requireStatus(t, logout, http.StatusNoContent)
}

func TestAccountAdmissionReturnsStableCodesForRejectedAndBannedUsers(t *testing.T) {
	app := newTestApp(t, nil)
	user := app.createUser(t, 92, "restricted-user", "user", "")
	token := app.token(t, user)

	for _, test := range []struct {
		status identity.AccountStatus
		code   string
	}{
		{status: identity.AccountRejected, code: apiErrorIdentityRejected},
		{status: identity.AccountBanned, code: apiErrorIdentityBanned},
	} {
		if err := app.db.Model(&identitygorm.User{}).
			Where("id = ?", user.ID).
			Update("status", test.status).Error; err != nil {
			t.Fatal(err)
		}
		response := app.request(t, http.MethodGet, "/api/v1/core/workspaces", token, 0, nil)
		requireStatus(t, response, http.StatusForbidden)
		assertAccessResponseCode(t, response, test.code)
	}
}

func TestAccountAdmissionBlocksBusinessRoutesWhilePasswordResetIsRequired(t *testing.T) {
	app := newTestApp(t, nil)
	user := app.createUser(t, 93, "reset-required", "user", "")
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", user.ID).
		Update("password_reset_required", true).Error; err != nil {
		t.Fatal(err)
	}
	token := app.token(t, user)

	me := app.request(t, http.MethodGet, "/api/v1/core/auth/me", token, 0, nil)
	requireStatus(t, me, http.StatusOK)

	blocked := app.request(t, http.MethodGet, "/api/v1/core/workspaces", token, 0, nil)
	requireStatus(t, blocked, http.StatusForbidden)
	assertAccessResponseCode(t, blocked, apiErrorPasswordResetRequired)
}

func TestRequiredPasswordChangeRouteIsNarrowlyScoped(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		want   bool
	}{
		{method: http.MethodPut, path: "/api/v1/core/auth/me/password", want: true},
		{method: http.MethodPut, path: "/core/auth/me/password", want: true},
		{method: http.MethodGet, path: "/api/v1/core/auth/me/account", want: false},
		{method: http.MethodPatch, path: "/api/v1/core/auth/me/profile", want: false},
	} {
		if got := isRequiredPasswordChangeRoute(test.method, test.path); got != test.want {
			t.Fatalf("%s %s allowed=%v want=%v", test.method, test.path, got, test.want)
		}
	}
}
