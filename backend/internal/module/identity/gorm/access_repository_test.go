package identitygorm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func TestAccessRepositoryRejectsDuplicateRoleAndRouteRecords(t *testing.T) {
	access := newAccessFixture(t)
	role, err := access.CreateRole(context.Background(), identity.RoleInput{Code: "operator", Name: "Operator", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := access.CreateRole(context.Background(), identity.RoleInput{Code: role.Code + "-copy", Name: role.Name, Enabled: true}); !errors.Is(err, identity.ErrAccessRoleNameConflict) {
		t.Fatalf("duplicate role err=%v", err)
	}
	_, err = access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{Code: "task.list", HTTPMethod: "GET", RouteTemplate: "/tasks", Name: "List tasks", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{Code: "task.list-copy", HTTPMethod: "GET", RouteTemplate: "/tasks", Name: "Duplicate route", Enabled: true}); !errors.Is(err, identity.ErrAccessRouteConflict) {
		t.Fatalf("duplicate route err=%v", err)
	}
}

func TestAccessRepositoryBootstrapIsIdempotentAndBindsOnlyConfiguredUser(t *testing.T) {
	access := newAccessFixture(t)
	if err := access.db.Create(&User{ID: 7, ExternalSubject: "user:7", Username: "admin", Active: true}).Error; err != nil {
		t.Fatal(err)
	}
	spec := identity.BootstrapSpec{
		AdministratorUserID: 7,
		ServiceInterfaces: []identity.ServiceInterfaceInput{
			{Code: "access.roles.list", HTTPMethod: "GET", RouteTemplate: "/api/v1/core/access/admin/roles", Name: "List access roles", Enabled: true},
			{Code: "access.roles.create", HTTPMethod: "POST", RouteTemplate: "/api/v1/core/access/admin/roles", Name: "Create access role", Enabled: true},
		},
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := access.Bootstrap(context.Background(), spec); err != nil {
			t.Fatalf("bootstrap attempt %d: %v", attempt, err)
		}
	}
	var roleCount, interfaceCount, bindingCount, grantCount int64
	access.db.Model(&AccessRole{}).Where("code = ?", identity.AccessAdministratorRoleCode).Count(&roleCount)
	access.db.Model(&AccessServiceInterface{}).Count(&interfaceCount)
	access.db.Model(&AccessUserRole{}).Where("user_id = ?", 7).Count(&bindingCount)
	access.db.Model(&AccessRoleServiceInterface{}).Count(&grantCount)
	if roleCount != 1 || interfaceCount != 2 || bindingCount != 1 || grantCount != 2 {
		t.Fatalf("bootstrap rows roles=%d interfaces=%d bindings=%d grants=%d", roleCount, interfaceCount, bindingCount, grantCount)
	}
	var groupBinding AccessUserGroupBinding
	if err := access.db.Where("user_id = ?", 7).First(&groupBinding).Error; err != nil {
		t.Fatalf("bootstrap must bind existing users to the default group: %v", err)
	}

	if err := access.Bootstrap(context.Background(), identity.BootstrapSpec{AdministratorUserID: 99, ServiceInterfaces: spec.ServiceInterfaces}); !errors.Is(err, identity.ErrAccessUserNotFound) {
		t.Fatalf("unknown bootstrap user err=%v", err)
	}
}

func TestAccessRepositoryBootstrapSeedsLegacyAdminsAndDefaultGroupBaseline(t *testing.T) {
	access := newAccessFixture(t)
	if err := access.db.Create(&User{
		ID: 3, ExternalSubject: "user:3", Username: "jiguang", Role: identity.LegacySuperAdminRole,
		Active: true, Status: string(identity.AccountActive),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := access.db.Create(&User{
		ID: 4, ExternalSubject: "user:4", Username: "member", Role: "user",
		Active: true, Status: string(identity.AccountActive),
	}).Error; err != nil {
		t.Fatal(err)
	}
	spec := identity.BootstrapSpec{
		ServiceInterfaces: []identity.ServiceInterfaceInput{
			{Code: "route.projects.view", ActionCode: string(accesscatalog.ActionProjectsProjectSetsView), HTTPMethod: "GET", RouteTemplate: "/api/v1/core/projects", Name: "View projects", Enabled: true},
			{Code: "route.access.roles.view", ActionCode: string(accesscatalog.ActionAccessManagementRolesView), HTTPMethod: "GET", RouteTemplate: "/api/v1/core/access/admin/roles", Name: "View roles", Enabled: true},
		},
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := access.Bootstrap(context.Background(), spec); err != nil {
			t.Fatalf("bootstrap attempt %d: %v", attempt, err)
		}
	}

	adminCodes, err := access.EffectiveActionCodes(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAction(adminCodes, string(accesscatalog.ActionProjectsProjectSetsView)) || !containsAction(adminCodes, string(accesscatalog.ActionAccessManagementRolesView)) {
		t.Fatalf("legacy super_admin codes=%v", adminCodes)
	}
	memberCodes, err := access.EffectiveActionCodes(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAction(memberCodes, string(accesscatalog.ActionProjectsProjectSetsView)) || containsAction(memberCodes, string(accesscatalog.ActionAccessManagementRolesView)) {
		t.Fatalf("default group codes=%v", memberCodes)
	}
	var groupGrantCount int64
	if err := access.db.Model(&AccessGroupServiceInterface{}).Count(&groupGrantCount).Error; err != nil {
		t.Fatal(err)
	}
	if groupGrantCount != 1 {
		t.Fatalf("default group grants=%d", groupGrantCount)
	}
}

func TestAccessRepositoryBootstrapDoesNotReplaceExistingDefaultGroupGrants(t *testing.T) {
	access := newAccessFixture(t)
	if err := access.db.Create(&User{
		ID: 4, ExternalSubject: "user:4", Username: "member", Role: "user",
		Active: true, Status: string(identity.AccountActive),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := access.Bootstrap(context.Background(), identity.BootstrapSpec{
		ServiceInterfaces: []identity.ServiceInterfaceInput{
			{Code: "route.projects.view", ActionCode: string(accesscatalog.ActionProjectsProjectSetsView), HTTPMethod: "GET", RouteTemplate: "/api/v1/core/projects", Name: "View projects", Enabled: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := access.Bootstrap(context.Background(), identity.BootstrapSpec{
		ServiceInterfaces: []identity.ServiceInterfaceInput{
			{Code: "route.projects.view", ActionCode: string(accesscatalog.ActionProjectsProjectSetsView), HTTPMethod: "GET", RouteTemplate: "/api/v1/core/projects", Name: "View projects", Enabled: true},
			{Code: "route.work-items.view", ActionCode: string(accesscatalog.ActionWorkItemsTaskTableView), HTTPMethod: "GET", RouteTemplate: "/api/v1/core/tasks", Name: "View tasks", Enabled: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	codes, err := access.EffectiveActionCodes(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAction(codes, string(accesscatalog.ActionProjectsProjectSetsView)) || containsAction(codes, string(accesscatalog.ActionWorkItemsTaskTableView)) {
		t.Fatalf("existing default group grants must stay operator-owned, codes=%v", codes)
	}
}

func TestAccessRepositoryBootstrapAddsExplicitDefaultGroupActionsIdempotently(t *testing.T) {
	access := newAccessFixture(t)
	if err := access.db.Create(&User{
		ID: 4, ExternalSubject: "user:4", Username: "member", Role: "user",
		Active: true, Status: string(identity.AccountActive),
	}).Error; err != nil {
		t.Fatal(err)
	}
	interfaces := []identity.ServiceInterfaceInput{
		{Code: "route.projects.view", ActionCode: string(accesscatalog.ActionProjectsProjectSetsView), HTTPMethod: "GET", RouteTemplate: "/api/v1/core/projects", Name: "View projects", Enabled: true},
		{Code: "route.documents.view", ActionCode: string(accesscatalog.ActionDocumentsContentView), HTTPMethod: "GET", RouteTemplate: "/api/v1/platform/documents/:id", Name: "View document", Enabled: true},
		{Code: "route.documents.assets.upload", ActionCode: string(accesscatalog.ActionDocumentsAssetsUpload), HTTPMethod: "POST", RouteTemplate: "/api/v1/platform/documents/:id/assets", Name: "Upload document asset", Enabled: true},
		{Code: "route.documents.scope", ActionCode: string(accesscatalog.ActionDocumentsScopeAll), HTTPMethod: "GET", RouteTemplate: "/api/v1/platform/documents/access-scope", Name: "Manage all documents", Enabled: true},
	}
	if err := access.Bootstrap(context.Background(), identity.BootstrapSpec{ServiceInterfaces: interfaces[:1]}); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := access.Bootstrap(context.Background(), identity.BootstrapSpec{
			ServiceInterfaces:               interfaces,
			DefaultGroupActionCodes:         []string{string(accesscatalog.ActionDocumentsContentView), string(accesscatalog.ActionDocumentsAssetsUpload)},
			DefaultGroupExcludedActionCodes: []string{string(accesscatalog.ActionDocumentsScopeAll)},
		}); err != nil {
			t.Fatalf("bootstrap attempt %d: %v", attempt, err)
		}
	}
	codes, err := access.EffectiveActionCodes(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAction(codes, string(accesscatalog.ActionDocumentsContentView)) ||
		!containsAction(codes, string(accesscatalog.ActionDocumentsAssetsUpload)) ||
		containsAction(codes, string(accesscatalog.ActionDocumentsScopeAll)) {
		t.Fatalf("default group codes=%v", codes)
	}
}

func TestAccessRepositoryBootstrapExcludesPrivilegedActionsFromNewDefaultGroup(t *testing.T) {
	access := newAccessFixture(t)
	if err := access.db.Create(&User{
		ID: 4, ExternalSubject: "user:4", Username: "member", Role: "user",
		Active: true, Status: string(identity.AccountActive),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := access.Bootstrap(context.Background(), identity.BootstrapSpec{
		ServiceInterfaces: []identity.ServiceInterfaceInput{
			{Code: "route.documents.view", ActionCode: string(accesscatalog.ActionDocumentsContentView), HTTPMethod: "GET", RouteTemplate: "/api/v1/platform/documents/:id", Name: "View document", Enabled: true},
			{Code: "route.documents.scope", ActionCode: string(accesscatalog.ActionDocumentsScopeAll), HTTPMethod: "GET", RouteTemplate: "/api/v1/platform/documents/access-scope", Name: "Manage all documents", Enabled: true},
		},
		DefaultGroupExcludedActionCodes: []string{string(accesscatalog.ActionDocumentsScopeAll)},
	}); err != nil {
		t.Fatal(err)
	}
	codes, err := access.EffectiveActionCodes(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAction(codes, string(accesscatalog.ActionDocumentsContentView)) ||
		containsAction(codes, string(accesscatalog.ActionDocumentsScopeAll)) {
		t.Fatalf("default group codes=%v", codes)
	}
}

func TestAccessRepositoryBootstrapExtendsExistingRoleAndGroupActionGrants(t *testing.T) {
	access := newAccessFixture(t)
	ctx := context.Background()
	initial := identity.ServiceInterfaceInput{
		Code: "testing.reports.legacy", ActionCode: string(accesscatalog.ActionTestingVersionReportsView),
		HTTPMethod: "GET", RouteTemplate: "/testing/reports/legacy", Name: "Legacy report list", Enabled: true,
	}
	if err := access.Bootstrap(ctx, identity.BootstrapSpec{ServiceInterfaces: []identity.ServiceInterfaceInput{initial}}); err != nil {
		t.Fatal(err)
	}
	var initialInterface AccessServiceInterface
	if err := access.db.Where("code = ?", initial.Code).First(&initialInterface).Error; err != nil {
		t.Fatal(err)
	}
	role, err := access.CreateRole(ctx, identity.RoleInput{Code: "report-reader", Name: "Report reader", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	group := AccessUserGroup{Code: "report-team", Name: "Report team", Enabled: true}
	if err := access.db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := access.db.Create(&AccessRoleServiceInterface{RoleID: role.ID, ServiceInterfaceID: initialInterface.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := access.db.Create(&AccessGroupServiceInterface{GroupID: group.ID, ServiceInterfaceID: initialInterface.ID}).Error; err != nil {
		t.Fatal(err)
	}

	canonical := identity.ServiceInterfaceInput{
		Code: "testing.reports.canonical", ActionCode: initial.ActionCode,
		HTTPMethod: "GET", RouteTemplate: "/testing/version-reports", Name: "Canonical report list", Enabled: true,
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := access.Bootstrap(ctx, identity.BootstrapSpec{ServiceInterfaces: []identity.ServiceInterfaceInput{initial, canonical}}); err != nil {
			t.Fatalf("bootstrap attempt %d: %v", attempt, err)
		}
	}

	var canonicalInterface AccessServiceInterface
	if err := access.db.Where("code = ?", canonical.Code).First(&canonicalInterface).Error; err != nil {
		t.Fatal(err)
	}
	for _, grant := range []struct {
		name  string
		model any
		query string
		id    uint
	}{
		{name: "role", model: &AccessRoleServiceInterface{}, query: "role_id = ? AND service_interface_id = ?", id: role.ID},
		{name: "group", model: &AccessGroupServiceInterface{}, query: "group_id = ? AND service_interface_id = ?", id: group.ID},
	} {
		var count int64
		if err := access.db.Model(grant.model).Where(grant.query, grant.id, canonicalInterface.ID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s canonical action grant count=%d want=1", grant.name, count)
		}
	}
}

func containsAction(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

func newAccessFixture(t *testing.T) *AccessRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &AccessRole{}, &AccessUserRole{}, &AccessServiceInterface{}, &AccessRoleServiceInterface{}, &AccessUserGroup{}, &AccessUserGroupBinding{}, &AccessGroupServiceInterface{}, &PersonnelAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	return NewAccessRepository(db)
}
