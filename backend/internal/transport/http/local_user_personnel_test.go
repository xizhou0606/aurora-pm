package httpapi

import (
	"net/http"
	"testing"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

func TestCreateLocalUserStartsPendingInDefaultGroup(t *testing.T) {
	app := newTestApp(t, nil)
	creator := app.createUser(t, 101, "user-creator", "admin", "")
	defaultWorkspace := identitygorm.Workspace{Name: "Default", Slug: "default", Enabled: true, IsDefault: true}
	if err := app.db.Create(&defaultWorkspace).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodPost, "/api/v1/core/users", app.token(t, creator), 0, map[string]any{
		"username":      "new-pm-user",
		"password":      "strong-password-123",
		"display_name":  "New PM User",
		"platform_role": "user",
	})
	requireStatus(t, response, http.StatusCreated)

	var user identitygorm.User
	if err := app.db.Where("username = ?", "new-pm-user").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if got := identity.EffectiveAccountStatus(user.Status, user.Active); got != identity.AccountPendingApproval {
		t.Fatalf("status=%q want=%q", got, identity.AccountPendingApproval)
	}
	var group identitygorm.AccessUserGroup
	if err := app.db.Joins(
		"JOIN access_user_group_bindings binding ON binding.group_id = access_user_groups.id",
	).Where("binding.user_id = ?", user.ID).First(&group).Error; err != nil {
		t.Fatal(err)
	}
	if group.Code != identity.DefaultUserGroupCode || !group.Enabled || !group.SystemManaged {
		t.Fatalf("default group=%+v", group)
	}
	var membership identitygorm.WorkspaceMember
	if err := app.db.Where("workspace_id = ? AND user_id = ?", defaultWorkspace.ID, user.ID).First(&membership).Error; err != nil {
		t.Fatalf("new local user must join default workspace: %v", err)
	}
	if membership.Role != "member" || !membership.Enabled {
		t.Fatalf("default workspace membership=%+v", membership)
	}
}
