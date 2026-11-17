package identitygorm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func TestDefaultUserGroupBackfillIsIdempotentAndSingleValued(t *testing.T) {
	db := newPersonnelFixture(t)
	users := []User{
		{ID: 1, Username: "alice", DisplayName: "Alice", Active: true, Status: string(identity.AccountActive)},
		{ID: 2, Username: "bob", DisplayName: "Bob", Active: false, Status: string(identity.AccountBanned)},
	}
	createPersonnelUsers(t, db, users...)
	for attempt := 0; attempt < 2; attempt++ {
		if err := db.Transaction(BackfillDefaultUserGroup); err != nil {
			t.Fatalf("backfill attempt %d: %v", attempt, err)
		}
	}
	var groups []AccessUserGroup
	if err := db.Find(&groups).Error; err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Code != identity.DefaultUserGroupCode || !groups[0].Enabled || !groups[0].SystemManaged {
		t.Fatalf("default groups=%+v", groups)
	}
	var bindings []AccessUserGroupBinding
	if err := db.Order("user_id ASC").Find(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 2 || bindings[0].UserID != 1 || bindings[1].UserID != 2 {
		t.Fatalf("bindings=%+v", bindings)
	}
	if err := db.Create(&AccessUserGroupBinding{UserID: 1, GroupID: groups[0].ID}).Error; err == nil {
		t.Fatal("unique user group binding accepted a second group")
	}
}

func TestPersonnelListFallsBackToDefaultGroupForLegacyUnboundUser(t *testing.T) {
	db := newPersonnelFixture(t)
	defaultGroup, err := EnsureDefaultUserGroup(db)
	if err != nil {
		t.Fatal(err)
	}
	createPersonnelUsers(t, db, User{
		ID: 7, Username: "legacy-unbound", DisplayName: "Legacy Unbound",
		Active: true, Status: string(identity.AccountActive),
	})

	page, err := NewAccessRepository(db).ListPersonnelUsers(context.Background(), identity.PersonnelUserFilter{
		Query: "legacy-unbound", Page: 1, PageSize: 20,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].GroupID != defaultGroup.ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestPersonnelRepositoryGroupLifecycleAndAudits(t *testing.T) {
	db := newPersonnelFixture(t)
	createPersonnelUsers(t, db,
		User{ID: 1, Username: "admin", DisplayName: "Admin", Active: true, Status: string(identity.AccountActive)},
		User{ID: 2, Username: "member", DisplayName: "Member", Active: true, Status: string(identity.AccountActive)},
	)
	if err := db.Transaction(BackfillDefaultUserGroup); err != nil {
		t.Fatal(err)
	}
	repository := NewAccessRepository(db)
	service := identity.NewPersonnelService(repository)
	group, err := service.CreateGroup(context.Background(), 1, identity.UserGroupInput{
		Code: "qa-team", Name: "QA team", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ReplaceUserGroup(context.Background(), 1, 2, group.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteGroup(context.Background(), 1, group.ID); !errors.Is(err, identity.ErrUserGroupInUse) {
		t.Fatalf("delete assigned group err=%v", err)
	}
	defaultGroup, err := findDefaultGroup(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ReplaceUserGroup(context.Background(), 1, 2, defaultGroup.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteGroup(context.Background(), 1, group.ID); err != nil {
		t.Fatal(err)
	}
	disabled := false
	if _, err := service.UpdateGroup(context.Background(), 1, defaultGroup.ID, identity.UserGroupUpdate{Enabled: &disabled}); !errors.Is(err, identity.ErrDefaultUserGroupProtected) {
		t.Fatalf("disable default group err=%v", err)
	}
	if err := service.DeleteGroup(context.Background(), 1, defaultGroup.ID); !errors.Is(err, identity.ErrDefaultUserGroupProtected) {
		t.Fatalf("delete default group err=%v", err)
	}
	events, err := service.AuditEvents(context.Background(), identity.PersonnelAuditFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("audit events=%d want 4: %+v", len(events), events)
	}
}

func TestPersonnelRepositoryStatusRolesAndLastAdministratorProtection(t *testing.T) {
	db := newPersonnelFixture(t)
	createPersonnelUsers(t, db,
		User{ID: 1, Username: "admin-one", DisplayName: "Admin One", Active: true, Status: string(identity.AccountActive), SessionVersion: 1},
		User{ID: 2, Username: "admin-two", DisplayName: "Admin Two", Active: true, Status: string(identity.AccountActive), SessionVersion: 1},
		User{ID: 3, Username: "applicant", DisplayName: "Applicant", Active: true, Status: string(identity.AccountPendingApproval), SessionVersion: 1},
	)
	if err := db.Transaction(BackfillDefaultUserGroup); err != nil {
		t.Fatal(err)
	}
	adminRole := AccessRole{Code: identity.AccessAdministratorRoleCode, Name: "Administrator", Enabled: true, SystemManaged: true}
	viewerRole := AccessRole{Code: "viewer", Name: "Viewer", Enabled: true}
	if err := db.Create(&adminRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&viewerRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AccessUserRole{UserID: 1, RoleID: adminRole.ID}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewAccessRepository(db)
	service := identity.NewPersonnelService(repository)
	if err := service.ReplaceUserRoles(context.Background(), 1, 1, []uint{viewerRole.ID}); !errors.Is(err, identity.ErrLastAdministrator) {
		t.Fatalf("strip last administrator err=%v", err)
	}
	if err := service.ReplaceUserRoles(context.Background(), 1, 2, []uint{adminRole.ID, viewerRole.ID}); err != nil {
		t.Fatal(err)
	}
	if err := service.ReplaceUserRoles(context.Background(), 1, 1, []uint{viewerRole.ID}); err != nil {
		t.Fatalf("strip administrator after recovery assignment: %v", err)
	}
	approved, err := service.TransitionUserStatus(context.Background(), 1, 3, identity.AccountActive, "")
	if err != nil || approved.Status != identity.AccountActive {
		t.Fatalf("approve=%+v err=%v", approved, err)
	}
	if _, err := service.TransitionUserStatus(context.Background(), 1, 3, identity.AccountBanned, ""); !errors.Is(err, identity.ErrAccountReasonRequired) {
		t.Fatalf("ban without reason err=%v", err)
	}
	banned, err := service.TransitionUserStatus(context.Background(), 1, 3, identity.AccountBanned, "policy violation")
	if err != nil {
		t.Fatal(err)
	}
	if banned.Status != identity.AccountBanned || banned.SessionVersion != 2 {
		t.Fatalf("banned user=%+v", banned)
	}
	if _, err := service.TransitionUserStatus(context.Background(), 1, 3, identity.AccountActive, "restored"); err != nil {
		t.Fatal(err)
	}
	events, err := service.AuditEvents(context.Background(), identity.PersonnelAuditFilter{SubjectUserID: 3, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("applicant events=%d want 3", len(events))
	}
}

func TestPersonnelRepositoryReplacesGroupAndRolesAtomically(t *testing.T) {
	db := newPersonnelFixture(t)
	createPersonnelUsers(t, db,
		User{ID: 1, Username: "admin", DisplayName: "Admin", Active: true, Status: string(identity.AccountActive)},
		User{ID: 2, Username: "member", DisplayName: "Member", Active: true, Status: string(identity.AccountActive)},
	)
	if err := db.Transaction(BackfillDefaultUserGroup); err != nil {
		t.Fatal(err)
	}
	role := AccessRole{Code: "editor", Name: "Editor", Enabled: true}
	group := AccessUserGroup{Code: "delivery", Name: "Delivery", Enabled: true}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	service := identity.NewPersonnelService(NewAccessRepository(db))
	if err := service.ReplaceUserAccess(context.Background(), 1, 2, group.ID, []uint{role.ID, 999}); !errors.Is(err, identity.ErrAccessRoleNotFound) {
		t.Fatalf("invalid access replacement err=%v", err)
	}
	defaultGroup, err := findDefaultGroup(db)
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.User(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if user.GroupID != defaultGroup.ID || len(user.RoleIDs) != 0 {
		t.Fatalf("failed replacement changed access=%+v", user)
	}
	if err := service.ReplaceUserAccess(context.Background(), 1, 2, group.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	user, err = service.User(context.Background(), 2)
	if err != nil || user.GroupID != group.ID || len(user.RoleIDs) != 1 || user.RoleIDs[0] != role.ID {
		t.Fatalf("replaced access=%+v err=%v", user, err)
	}
	events, err := service.AuditEvents(context.Background(), identity.PersonnelAuditFilter{SubjectUserID: 2, Limit: 100})
	if err != nil || len(events) != 1 || events[0].EventType != "user.access_replaced" {
		t.Fatalf("access audit=%+v err=%v", events, err)
	}
}

func TestPersonnelRepositoryListsUsersWithSearchMembershipAndPagination(t *testing.T) {
	db := newPersonnelFixture(t)
	createPersonnelUsers(t, db,
		User{ID: 1, Username: "admin", DisplayName: "Admin", Email: "admin@example.com", Active: true, Status: string(identity.AccountActive)},
		User{ID: 2, Username: "alice", DisplayName: "研发小艾", Email: "alice@example.com", AvatarURL: "/avatars/alice.png", Active: true, Status: string(identity.AccountActive)},
		User{ID: 3, Username: "bob", DisplayName: "Bob", Email: "bob@example.com", Active: true, Status: string(identity.AccountActive)},
	)
	if err := db.Transaction(BackfillDefaultUserGroup); err != nil {
		t.Fatal(err)
	}
	role := AccessRole{Code: "reviewer", Name: "Reviewer", Enabled: true}
	group := AccessUserGroup{Code: "delivery", Name: "Delivery", Enabled: true}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AccessUserRole{UserID: 2, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id = ?", 2).Delete(&AccessUserGroupBinding{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AccessUserGroupBinding{UserID: 2, GroupID: group.ID}).Error; err != nil {
		t.Fatal(err)
	}
	service := identity.NewPersonnelService(NewAccessRepository(db))

	searchPage, err := service.ListUsers(context.Background(), identity.PersonnelUserFilter{
		Query: "ALICE@", Page: 1, PageSize: 10,
	})
	if err != nil || searchPage.Total != 1 || len(searchPage.Items) != 1 || searchPage.Items[0].Username != "alice" ||
		searchPage.Items[0].AvatarURL != "/avatars/alice.png" {
		t.Fatalf("search page=%+v err=%v", searchPage, err)
	}
	nicknamePage, err := service.ListUsers(context.Background(), identity.PersonnelUserFilter{
		Query: "小艾", Page: 1, PageSize: 10,
	})
	if err != nil || nicknamePage.Total != 1 || len(nicknamePage.Items) != 1 || nicknamePage.Items[0].ID != 2 {
		t.Fatalf("nickname search page=%+v err=%v", nicknamePage, err)
	}
	rolePage, err := service.ListUsers(context.Background(), identity.PersonnelUserFilter{
		RoleID: role.ID, Page: 1, PageSize: 10,
	})
	if err != nil || rolePage.Total != 1 || len(rolePage.Items) != 1 || rolePage.Items[0].ID != 2 {
		t.Fatalf("role page=%+v err=%v", rolePage, err)
	}
	groupPage, err := service.ListUsers(context.Background(), identity.PersonnelUserFilter{
		GroupID: group.ID, Page: 1, PageSize: 10,
	})
	if err != nil || groupPage.Total != 1 || len(groupPage.Items) != 1 || groupPage.Items[0].ID != 2 {
		t.Fatalf("group page=%+v err=%v", groupPage, err)
	}
	paged, err := service.ListUsers(context.Background(), identity.PersonnelUserFilter{Page: 2, PageSize: 2})
	if err != nil || paged.Total != 3 || len(paged.Items) != 1 || paged.HasMore {
		t.Fatalf("paged=%+v err=%v", paged, err)
	}
}

func newPersonnelFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&User{}, &AccessRole{}, &AccessUserRole{}, &AccessUserGroup{}, &AccessUserGroupBinding{}, &PersonnelAuditEvent{},
		&AccessGroupServiceInterface{}, &AccessGroupMenu{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func createPersonnelUsers(t *testing.T, db *gorm.DB, users ...User) {
	t.Helper()
	for index := range users {
		if users[index].ExternalSubject == "" {
			users[index].ExternalSubject = fmt.Sprintf("local:%d", users[index].ID)
		}
		if err := db.Create(&users[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func findDefaultGroup(db *gorm.DB) (AccessUserGroup, error) {
	var group AccessUserGroup
	err := db.Where("code = ?", identity.DefaultUserGroupCode).First(&group).Error
	return group, err
}
