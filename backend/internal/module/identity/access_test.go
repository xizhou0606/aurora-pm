package identity_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
)

func TestAccessServiceAuthorizesEnabledRoleUnion(t *testing.T) {
	access, _, db := newAccessFixture(t)
	user := activeUser(t, db, 1, "access-user")
	reader := role(t, access, "reader")
	writer := role(t, access, "writer")
	list := serviceInterface(t, access, "project.list", "GET", "/api/v1/platform/project-sets")
	create := serviceInterface(t, access, "project.create", "POST", "/api/v1/platform/project-sets")

	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{reader.ID, writer.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), reader.ID, []uint{list.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), writer.ID, []uint{create.ID}); err != nil {
		t.Fatal(err)
	}

	assertAuthorization(t, access, user.ID, "GET", "/api/v1/platform/project-sets", identity.AuthorizationAllowed)
	assertAuthorization(t, access, user.ID, "POST", "/api/v1/platform/project-sets", identity.AuthorizationAllowed)
	assertAuthorization(t, access, user.ID, "DELETE", "/api/v1/platform/project-sets", identity.AuthorizationInterfaceUnregistered)

	disabled := false
	if _, err := access.UpdateRole(context.Background(), writer.ID, identity.RoleUpdate{Enabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	assertAuthorization(t, access, user.ID, "POST", "/api/v1/platform/project-sets", identity.AuthorizationForbidden)

	if _, err := access.UpdateServiceInterface(context.Background(), list.ID, identity.ServiceInterfaceUpdate{Enabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	assertAuthorization(t, access, user.ID, "GET", "/api/v1/platform/project-sets", identity.AuthorizationInterfaceUnregistered)
}

func TestAccessServicePreservesBroadActionGrantForDedicatedTransitionRoute(t *testing.T) {
	access, _, db := newAccessFixture(t)
	user := activeUser(t, db, 1, "quality-editor")
	editor := role(t, access, "quality-editor")
	for _, input := range []identity.ServiceInterfaceInput{
		{Code: "quality.bug.update", ActionCode: string(accesscatalog.ActionQualityBugsUpdate), HTTPMethod: "PATCH", RouteTemplate: "/api/v1/platform/quality/bugs/:id", Name: "Edit bug", Enabled: true},
		{Code: "quality.bug.transition", ActionCode: string(accesscatalog.ActionQualityBugsTransitionStatus), HTTPMethod: "PATCH", RouteTemplate: "/api/v1/platform/quality/bugs/:id/status", Name: "Transition bug", Enabled: true},
	} {
		if _, err := access.CreateServiceInterface(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	if err := access.ReplaceRoleActionCodes(context.Background(), user.ID, editor.ID, []string{string(accesscatalog.ActionQualityBugsUpdate)}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{editor.ID}); err != nil {
		t.Fatal(err)
	}

	assertAuthorization(t, access, user.ID, "PATCH", "/api/v1/platform/quality/bugs/:id", identity.AuthorizationAllowed)
	assertAuthorization(t, access, user.ID, "PATCH", "/api/v1/platform/quality/bugs/:id/status", identity.AuthorizationAllowed)
}

func TestAccessServiceRejectsInvalidGrantReplacementWithoutChangingExistingGrants(t *testing.T) {
	access, _, db := newAccessFixture(t)
	user := activeUser(t, db, 1, "access-user")
	role := role(t, access, "operator")
	first := serviceInterface(t, access, "task.list", "GET", "/api/v1/platform/tasks")
	second := serviceInterface(t, access, "task.create", "POST", "/api/v1/platform/tasks")

	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{first.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleServiceInterfaces(context.Background(), role.ID, []uint{second.ID, 999}); !errors.Is(err, identity.ErrAccessServiceInterfaceNotFound) {
		t.Fatalf("invalid replacement err=%v", err)
	}
	ids, err := access.RoleServiceInterfaceIDs(context.Background(), role.ID)
	if err != nil || len(ids) != 1 || ids[0] != first.ID {
		t.Fatalf("interface grants after failed replace: %v err=%v", ids, err)
	}
	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{role.ID, role.ID}); !errors.Is(err, identity.ErrAccessDuplicateID) {
		t.Fatalf("duplicate roles err=%v", err)
	}
	roleIDs, err := access.UserRoleIDs(context.Background(), user.ID)
	if err != nil || len(roleIDs) != 1 || roleIDs[0] != role.ID {
		t.Fatalf("role grants after duplicate replace: %v err=%v", roleIDs, err)
	}
}

func TestAccessServiceAuthorizesEnabledDefaultGroupUnion(t *testing.T) {
	access, _, db := newAccessFixture(t)
	user := activeUser(t, db, 1, "group-user")
	if err := db.Transaction(identitygorm.BackfillDefaultUserGroup); err != nil {
		t.Fatal(err)
	}
	var group identitygorm.AccessUserGroup
	if err := db.Where("code = ?", identity.DefaultUserGroupCode).First(&group).Error; err != nil {
		t.Fatal(err)
	}
	view := serviceInterface(t, access, "projects.view", "GET", "/api/v1/core/projects")
	if err := access.ReplaceGroupServiceInterfaces(context.Background(), user.ID, group.ID, []uint{view.ID}); err != nil {
		t.Fatal(err)
	}
	if got, err := access.Authorize(context.Background(), user.ID, "GET", "/api/v1/core/projects"); err != nil || got != identity.AuthorizationAllowed {
		t.Fatalf("default group authorize=%v err=%v", got, err)
	}
	if err := db.Model(&identitygorm.AccessUserGroup{}).Where("id = ?", group.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if got, err := access.Authorize(context.Background(), user.ID, "GET", "/api/v1/core/projects"); err != nil || got != identity.AuthorizationForbidden {
		t.Fatalf("disabled group authorize=%v err=%v", got, err)
	}
}

func TestAccessServiceExpandsActionsAcrossRoutesAndReturnsRoleGroupUnion(t *testing.T) {
	access, _, db := newAccessFixture(t)
	user := activeUser(t, db, 1, "action-user")
	if err := db.Transaction(identitygorm.BackfillDefaultUserGroup); err != nil {
		t.Fatal(err)
	}
	var group identitygorm.AccessUserGroup
	if err := db.Where("code = ?", identity.DefaultUserGroupCode).First(&group).Error; err != nil {
		t.Fatal(err)
	}
	const groupAction = "pm.projects.detail.view"
	for index, route := range []string{"/api/v1/core/projects/:id", "/api/v1/core/projects/:id/activity"} {
		_, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
			Code: fmt.Sprintf("project.route-%d", index), ActionCode: groupAction,
			HTTPMethod: "GET", RouteTemplate: route, Name: route, Enabled: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := access.ReplaceGroupActionCodes(context.Background(), user.ID, group.ID, []string{groupAction}); err != nil {
		t.Fatal(err)
	}
	groupCodes, err := access.GroupActionCodes(context.Background(), group.ID)
	if err != nil || len(groupCodes) != 1 || groupCodes[0] != groupAction {
		t.Fatalf("group action codes=%v err=%v", groupCodes, err)
	}
	for _, route := range []string{"/api/v1/core/projects/:id", "/api/v1/core/projects/:id/activity"} {
		if got, err := access.Authorize(context.Background(), user.ID, "GET", route); err != nil || got != identity.AuthorizationAllowed {
			t.Fatalf("authorize %s=%v err=%v", route, got, err)
		}
	}
	role := role(t, access, "report-reader")
	const roleAction = "pm.testing.reports.view"
	if _, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
		Code: "testing.reports.get", ActionCode: roleAction, HTTPMethod: "GET",
		RouteTemplate: "/api/v1/core/testing/reports/:id", Name: "Read report", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleActionCodes(context.Background(), user.ID, role.ID, []string{roleAction}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	codes, err := access.EffectiveActionCodes(context.Background(), user.ID)
	if err != nil || len(codes) != 2 || codes[0] != groupAction || codes[1] != roleAction {
		t.Fatalf("effective action codes=%v err=%v", codes, err)
	}
	events, err := identity.NewPersonnelService(identitygorm.NewAccessRepository(db)).AuditEvents(
		context.Background(), identity.PersonnelAuditFilter{SubjectRoleID: role.ID, Limit: 100},
	)
	if err != nil || len(events) != 1 || events[0].ActorUserID != user.ID || events[0].EventType != "role.action_grants_replaced" {
		t.Fatalf("role grant audit=%+v err=%v", events, err)
	}
}

func TestAccessServiceAuditsCanonicalRoleLifecycle(t *testing.T) {
	access, _, db := newAccessFixture(t)
	actor := activeUser(t, db, 1, "role-admin")
	created, err := access.CreateRoleAudited(context.Background(), actor.ID, identity.RoleInput{
		Code: "release-manager", Name: "Release manager", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	name := "Release owner"
	if _, err := access.UpdateRoleAudited(context.Background(), actor.ID, created.ID, identity.RoleUpdate{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if err := access.DeleteRoleAudited(context.Background(), actor.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	events, err := identity.NewPersonnelService(identitygorm.NewAccessRepository(db)).AuditEvents(
		context.Background(), identity.PersonnelAuditFilter{SubjectRoleID: created.ID, Limit: 100},
	)
	if err != nil || len(events) != 3 {
		t.Fatalf("role lifecycle audit=%+v err=%v", events, err)
	}
	for _, event := range events {
		if event.ActorUserID != actor.ID || event.SubjectRoleID == nil || *event.SubjectRoleID != created.ID {
			t.Fatalf("invalid role audit=%+v", event)
		}
	}
}

func TestAccessServiceOwnsRoleCodeAndRequiresUniqueName(t *testing.T) {
	access, _, db := newAccessFixture(t)
	actor := activeUser(t, db, 1, "role-owner")
	created, err := access.CreateRoleAudited(context.Background(), actor.ID, identity.RoleInput{
		Code: "client-supplied-code", Name: "前端开发", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Code == "client-supplied-code" || created.Code == created.Name || created.Name != "前端开发" {
		t.Fatalf("created role=%+v", created)
	}
	if decoded, err := hex.DecodeString(created.Code); err != nil || len(decoded) != 32 {
		t.Fatalf("generated role code=%q err=%v", created.Code, err)
	}
	stableCode := created.Code
	if _, err := access.CreateRoleAudited(context.Background(), actor.ID, identity.RoleInput{
		Code: "another-code", Name: "前端开发", Enabled: true,
	}); !errors.Is(err, identity.ErrAccessRoleNameConflict) {
		t.Fatalf("duplicate name err=%v", err)
	}
	name := "Web 开发"
	updated, err := access.UpdateRoleAudited(context.Background(), actor.ID, created.ID, identity.RoleUpdate{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Code != stableCode || updated.Name != name {
		t.Fatalf("updated role=%+v", updated)
	}
}

func TestAccessServiceBuildsMenuAncestorClosureAndProtectsGrantedResources(t *testing.T) {
	access, menus, db := newAccessFixture(t)
	user := activeUser(t, db, 1, "access-user")
	role := role(t, access, "viewer")
	root := menu(t, menus, workbench.MenuInput{Code: "root", Name: "Root", MenuType: workbench.MenuTypeDirectory, HostKey: "pm", Enabled: true})
	child := menu(t, menus, workbench.MenuInput{Code: "child", ParentID: &root.ID, Name: "Child", MenuType: workbench.MenuTypeDirectory, HostKey: "pm", Enabled: true})
	leaf := menu(t, menus, workbench.MenuInput{Code: "leaf", ParentID: &child.ID, Name: "Leaf", MenuType: workbench.MenuTypePage, Path: "/projects", HostKey: "pm", Enabled: true})
	if err := access.ReplaceUserRoles(context.Background(), user.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleMenus(context.Background(), role.ID, []uint{leaf.ID}); err != nil {
		t.Fatal(err)
	}

	tree, err := menus.MenusForUser(context.Background(), user.ID, "pm")
	if err != nil || len(tree) != 1 || tree[0].ID != root.ID || tree[0].DirectGrant || !tree[0].Structural ||
		len(tree[0].Children) != 1 || tree[0].Children[0].ID != child.ID || tree[0].Children[0].DirectGrant ||
		len(tree[0].Children[0].Children) != 1 || tree[0].Children[0].Children[0].ID != leaf.ID || !tree[0].Children[0].Children[0].DirectGrant {
		t.Fatalf("menu closure=%+v err=%v", tree, err)
	}
	if err := menus.Delete(context.Background(), leaf.ID); !errors.Is(err, workbench.ErrMenuInUse) {
		t.Fatalf("delete granted menu err=%v", err)
	}
	if err := access.DeleteRole(context.Background(), role.ID); !errors.Is(err, identity.ErrAccessResourceInUse) {
		t.Fatalf("delete granted role err=%v", err)
	}
}

func TestAccessServiceReturnsEnabledGroupMenuGrants(t *testing.T) {
	access, menus, db := newAccessFixture(t)
	user := activeUser(t, db, 1, "group-menu-user")
	if err := db.Transaction(identitygorm.BackfillDefaultUserGroup); err != nil {
		t.Fatal(err)
	}
	var group identitygorm.AccessUserGroup
	if err := db.Where("code = ?", identity.DefaultUserGroupCode).First(&group).Error; err != nil {
		t.Fatal(err)
	}
	page := menu(t, menus, workbench.MenuInput{Code: "group-page", Name: "Group page", MenuType: workbench.MenuTypePage, Path: "/group-page", HostKey: "pm", Enabled: true})
	if err := access.ReplaceGroupMenus(context.Background(), user.ID, group.ID, []uint{page.ID}); err != nil {
		t.Fatal(err)
	}
	ids, err := access.DirectMenuIDsForUser(context.Background(), user.ID)
	if err != nil || len(ids) != 1 || ids[0] != page.ID {
		t.Fatalf("group menu ids=%v err=%v", ids, err)
	}
	if err := db.Model(&identitygorm.AccessUserGroup{}).Where("id = ?", group.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	ids, err = access.DirectMenuIDsForUser(context.Background(), user.ID)
	if err != nil || len(ids) != 0 {
		t.Fatalf("disabled group menu ids=%v err=%v", ids, err)
	}
}

func TestAccessServiceBulkRegistrationIsAtomicAndPMUserRegistryExcludesInactiveUsers(t *testing.T) {
	access, _, db := newAccessFixture(t)
	activeUser(t, db, 1, "active-user")
	inactive := identitygorm.User{ID: 2, ExternalSubject: "user:2", Username: "inactive-user", DisplayName: "Inactive", Active: false}
	if err := db.Create(&inactive).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&inactive).Update("active", false).Error; err != nil {
		t.Fatal(err)
	}
	existing := serviceInterface(t, access, "task.list", "GET", "/api/v1/platform/tasks")
	_, err := access.CreateServiceInterfaces(context.Background(), []identity.ServiceInterfaceInput{
		{Code: "task.create", HTTPMethod: "POST", RouteTemplate: "/api/v1/platform/tasks", Name: "Create task", Enabled: true},
		{Code: existing.Code, HTTPMethod: "DELETE", RouteTemplate: "/api/v1/platform/tasks", Name: "Conflicting task", Enabled: true},
	})
	if !errors.Is(err, identity.ErrAccessCodeConflict) {
		t.Fatalf("bulk registration err=%v", err)
	}
	interfaces, err := access.ListServiceInterfaces(context.Background(), true)
	if err != nil || len(interfaces) != 1 || interfaces[0].ID != existing.ID {
		t.Fatalf("interfaces after rejected bulk registration=%+v err=%v", interfaces, err)
	}
	users, err := access.ListActiveUsers(context.Background())
	if err != nil || len(users) != 1 || users[0].ID != 1 || !users[0].Active {
		t.Fatalf("active users=%+v err=%v", users, err)
	}
}

func newAccessFixture(t *testing.T) (*identity.AccessService, *workbench.MenuService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&identitygorm.User{}, &identitygorm.AccessRole{}, &identitygorm.AccessUserRole{},
		&identitygorm.AccessServiceInterface{}, &identitygorm.AccessRoleServiceInterface{}, &identitygorm.AccessRoleMenu{},
		&identitygorm.AccessUserGroup{}, &identitygorm.AccessUserGroupBinding{}, &identitygorm.AccessGroupServiceInterface{},
		&identitygorm.AccessGroupMenu{},
		&identitygorm.PersonnelAuditEvent{},
		&workbenchgorm.AccessMenu{},
	); err != nil {
		t.Fatal(err)
	}
	access := identity.NewAccessService(identitygorm.NewAccessRepository(db), nil)
	menus := workbench.NewMenuService(workbenchgorm.NewAccessMenuRepository(db), access)
	access.SetMenuGrantValidator(menus)
	access.SetServiceInterfaceValidator(allowAllServiceInterfaces{})
	return access, menus, db
}

type allowAllServiceInterfaces struct{}

func (allowAllServiceInterfaces) ValidateServiceInterface(context.Context, string, string) error {
	return nil
}

func activeUser(t *testing.T, db *gorm.DB, id uint, username string) identitygorm.User {
	t.Helper()
	user := identitygorm.User{ID: id, ExternalSubject: fmt.Sprintf("user:%d", id), Username: username, DisplayName: username, Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func role(t *testing.T, service *identity.AccessService, code string) identity.Role {
	t.Helper()
	value, err := service.CreateRole(context.Background(), identity.RoleInput{Code: code, Name: code, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func serviceInterface(t *testing.T, service *identity.AccessService, code, method, route string) identity.ServiceInterface {
	t.Helper()
	value, err := service.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{Code: code, HTTPMethod: method, RouteTemplate: route, Name: code, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func menu(t *testing.T, service *workbench.MenuService, input workbench.MenuInput) workbench.Menu {
	t.Helper()
	value, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func assertAuthorization(t *testing.T, service *identity.AccessService, userID uint, method, route string, want identity.AuthorizationResult) {
	t.Helper()
	got, err := service.Authorize(context.Background(), userID, method, route)
	if err != nil || got != want {
		t.Fatalf("Authorize(%q, %q)=(%v, %v), want (%v, nil)", method, route, got, err, want)
	}
}
