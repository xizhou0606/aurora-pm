package identitygorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func workspaceAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&User{}, &Workspace{}, &WorkspaceMember{}, &Team{}, &AccessRole{}, &AccessUserRole{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestWorkspaceAdminRepositoryScopesWorkspaceListToMembershipExceptAdministrator(t *testing.T) {
	db := workspaceAdminTestDB(t)
	repository := NewAccessRepository(db)
	users := []User{
		{ExternalSubject: "workspace-list-member", Username: "member", Active: true, Status: "active"},
		{ExternalSubject: "workspace-list-root", Username: "root", Active: true, Status: "active"},
	}
	workspaces := []Workspace{
		{Name: "A", Slug: "a", Enabled: true},
		{Name: "B", Slug: "b", Enabled: true},
		{Name: "Disabled", Slug: "disabled", Enabled: false},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&workspaces).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Workspace{}).Where("id = ?", workspaces[2].ID).UpdateColumn("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WorkspaceMember{
		WorkspaceID: workspaces[0].ID, UserID: users[0].ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	administratorRole := AccessRole{
		Code: identity.AccessAdministratorRoleCode, Name: "Access administrator", Enabled: true, SystemManaged: true,
	}
	if err := db.Create(&administratorRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AccessUserRole{UserID: users[1].ID, RoleID: administratorRole.ID}).Error; err != nil {
		t.Fatal(err)
	}

	memberWorkspaces, err := repository.ListManagedWorkspaces(context.Background(), users[0].ID, true)
	if err != nil || len(memberWorkspaces) != 1 || memberWorkspaces[0].ID != workspaces[0].ID {
		t.Fatalf("member workspaces=%+v err=%v", memberWorkspaces, err)
	}
	if _, err := repository.FindVisibleManagedWorkspace(context.Background(), users[0].ID, workspaces[1].ID); !errors.Is(err, identity.ErrWorkspaceAdminNotFound) {
		t.Fatalf("foreign workspace visibility err=%v", err)
	}
	administratorWorkspaces, err := repository.ListManagedWorkspaces(context.Background(), users[1].ID, true)
	if err != nil || len(administratorWorkspaces) != 3 {
		t.Fatalf("administrator workspaces=%+v err=%v", administratorWorkspaces, err)
	}
	if workspace, err := repository.FindVisibleManagedWorkspace(context.Background(), users[1].ID, workspaces[1].ID); err != nil || workspace.ID != workspaces[1].ID {
		t.Fatalf("administrator workspace=%+v err=%v", workspace, err)
	}
}

func TestWorkspaceAdminRepositoryScopesMembersByWorkspace(t *testing.T) {
	db := workspaceAdminTestDB(t)
	repository := NewAccessRepository(db)
	user := User{Username: "member", DisplayName: "成员", Active: true, Status: "active"}
	workspaces := []Workspace{{Name: "A", Slug: "a", Enabled: true}, {Name: "B", Slug: "b", Enabled: true}}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&workspaces).Error; err != nil {
		t.Fatal(err)
	}
	created, err := repository.CreateManagedWorkspaceMember(context.Background(), workspaces[0].ID, identity.WorkspaceMemberInput{
		UserID: user.ID, Role: "member", Enabled: true,
	})
	if err != nil || created.UserID != user.ID {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	role := "owner"
	if _, err := repository.UpdateManagedWorkspaceMember(context.Background(), workspaces[1].ID, user.ID, identity.WorkspaceMemberUpdate{Role: &role}); !errors.Is(err, identity.ErrWorkspaceMemberNotFound) {
		t.Fatalf("cross-workspace update err=%v", err)
	}
	if err := repository.DeleteManagedWorkspaceMember(context.Background(), workspaces[1].ID, user.ID); !errors.Is(err, identity.ErrWorkspaceMemberNotFound) {
		t.Fatalf("cross-workspace delete err=%v", err)
	}
	members, err := repository.ListManagedWorkspaceMembers(context.Background(), workspaces[0].ID, identity.WorkspaceMemberFilter{
		Query: "成员", IncludeDisabled: true, Page: 1, PageSize: 20,
	})
	if err != nil || members.Total != 1 || len(members.Items) != 1 || members.Items[0].Role != "member" {
		t.Fatalf("members=%+v err=%v", members, err)
	}
}

func TestWorkspaceAdminRepositorySearchesAndPaginatesMembers(t *testing.T) {
	db := workspaceAdminTestDB(t)
	repository := NewAccessRepository(db)
	workspace := Workspace{Name: "Search", Slug: "search", Enabled: true}
	users := []User{
		{ExternalSubject: "workspace-search-1", Username: "alice-one", DisplayName: "Alice One", Email: "one@example.com", Active: true, Status: "active"},
		{ExternalSubject: "workspace-search-2", Username: "alice-two", DisplayName: "Alice Two", Email: "two@example.com", Active: true, Status: "active"},
		{ExternalSubject: "workspace-search-3", Username: "bob", DisplayName: "Bob", Email: "alice-contact@example.com", Active: true, Status: "active"},
	}
	if err := db.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if err := db.Create(&WorkspaceMember{WorkspaceID: workspace.ID, UserID: user.ID, Role: "member", Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
	}

	page, err := repository.ListManagedWorkspaceMembers(context.Background(), workspace.ID, identity.WorkspaceMemberFilter{
		Query: "ALICE", IncludeDisabled: true, Page: 2, PageSize: 1,
	})
	if err != nil || page.Total != 3 || page.Page != 2 || page.PageSize != 1 || !page.HasMore || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestWorkspaceAdminRepositoryProtectsDefaultAndUsedWorkspaces(t *testing.T) {
	db := workspaceAdminTestDB(t)
	repository := NewAccessRepository(db)
	workspaces := []Workspace{
		{Name: "Default", Slug: "default", Enabled: true, IsDefault: true},
		{Name: "Used", Slug: "used", Enabled: true},
		{Name: "Empty", Slug: "empty", Enabled: true},
	}
	if err := db.Create(&workspaces).Error; err != nil {
		t.Fatal(err)
	}
	defaultWorkspace, usedWorkspace, emptyWorkspace := workspaces[0], workspaces[1], workspaces[2]
	if err := db.Create(&Team{WorkspaceID: usedWorkspace.ID, Name: "Team"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteManagedWorkspace(context.Background(), defaultWorkspace.ID); !errors.Is(err, identity.ErrWorkspaceDefaultProtected) {
		t.Fatalf("default delete err=%v", err)
	}
	disabled := false
	if _, err := repository.UpdateManagedWorkspace(context.Background(), defaultWorkspace.ID, identity.WorkspaceUpdate{Enabled: &disabled}); !errors.Is(err, identity.ErrWorkspaceDefaultProtected) {
		t.Fatalf("default disable err=%v", err)
	}
	if _, err := repository.UpdateManagedWorkspace(context.Background(), defaultWorkspace.ID, identity.WorkspaceUpdate{IsDefault: &disabled}); !errors.Is(err, identity.ErrWorkspaceDefaultProtected) {
		t.Fatalf("default clear err=%v", err)
	}
	if err := db.Create(&Workspace{Name: "Second Default", Slug: "second-default", Enabled: true, IsDefault: true}).Error; err == nil {
		t.Fatal("second default workspace was accepted")
	}
	if err := repository.DeleteManagedWorkspace(context.Background(), usedWorkspace.ID); !errors.Is(err, identity.ErrWorkspaceInUse) {
		t.Fatalf("used delete err=%v", err)
	}
	if err := repository.DeleteManagedWorkspace(context.Background(), emptyWorkspace.ID); err != nil {
		t.Fatalf("empty delete err=%v", err)
	}
}

func TestWorkspaceAdminRepositorySwitchesDefaultAtomically(t *testing.T) {
	db := workspaceAdminTestDB(t)
	repository := NewAccessRepository(db)
	workspaces := []Workspace{
		{Name: "Default", Slug: "default", Enabled: true, IsDefault: true},
		{Name: "Disabled", Slug: "disabled", Enabled: false},
	}
	if err := db.Create(&workspaces).Error; err != nil {
		t.Fatal(err)
	}
	makeDefault := true
	updated, err := repository.UpdateManagedWorkspace(context.Background(), workspaces[1].ID, identity.WorkspaceUpdate{IsDefault: &makeDefault})
	if err != nil || !updated.IsDefault || !updated.Enabled {
		t.Fatalf("set default workspace=%+v err=%v", updated, err)
	}
	var previousDefault Workspace
	if err := db.First(&previousDefault, workspaces[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if previousDefault.IsDefault {
		t.Fatalf("previous default was not cleared: %+v", previousDefault)
	}
}
