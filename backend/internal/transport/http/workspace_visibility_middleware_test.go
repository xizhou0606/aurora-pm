package httpapi

import (
	"net/http"
	"testing"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

func TestWorkspaceMiddlewareRejectsNonMemberAndAllowsAccessAdministrator(t *testing.T) {
	app := newTestApp(t, nil)
	member := app.createUser(t, 91, "workspace-member", "user", "")
	administrator := app.createUser(t, 92, "workspace-administrator", identity.LegacySuperAdminRole, "")
	owned := identitygorm.Workspace{ID: 91, Name: "Owned", Slug: "owned", Enabled: true}
	foreign := identitygorm.Workspace{ID: 92, Name: "Foreign", Slug: "foreign", Enabled: true}
	for _, record := range []any{
		&owned,
		&foreign,
		&identitygorm.WorkspaceMember{WorkspaceID: owned.ID, UserID: member.ID, Role: "member", Enabled: true},
	} {
		if err := app.db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}

	memberToken := app.token(t, member)
	requireStatus(t, app.request(t, http.MethodGet, "/api/v1/core/projects", memberToken, owned.ID, nil), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodGet, "/api/v1/core/projects", memberToken, foreign.ID, nil), http.StatusNotFound)
	requireStatus(t, app.request(t, http.MethodGet, "/api/v1/core/projects", app.token(t, administrator), foreign.ID, nil), http.StatusOK)
}
