package identitygorm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func TestBaselineDirectoryReturnsOnlyMinimalActiveWorkspaceFacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&User{}, &Workspace{}, &WorkspaceMember{}, &Team{}, &TeamMember{}, &AccessRole{}, &AccessUserRole{},
	); err != nil {
		t.Fatal(err)
	}
	workspaces := []Workspace{
		{ID: 1, Name: "默认工作区", Slug: "default", Enabled: true, IsDefault: true},
		{ID: 2, Name: "已停用", Slug: "disabled", Enabled: false},
		{ID: 3, Name: "其他工作区", Slug: "other", Enabled: true},
	}
	if err := db.Create(&workspaces).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Workspace{}).Where("id = ?", 2).UpdateColumn("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	users := []User{
		{ID: 10, ExternalSubject: "local:10", Username: "alice", DisplayName: "Alice", Email: "private@example.com", Role: "admin", Active: true, Status: string(identity.AccountActive)},
		{ID: 11, ExternalSubject: "local:11", Username: "disabled-user", DisplayName: "Disabled", Active: true, Status: string(identity.AccountActive)},
		{ID: 12, ExternalSubject: "local:12", Username: "root", DisplayName: "Root", Active: true, Status: string(identity.AccountActive)},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&User{}).Where("id = ?", 11).UpdateColumn("active", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]WorkspaceMember{
		{WorkspaceID: 1, UserID: 10, Role: "member", Enabled: true},
		{WorkspaceID: 3, UserID: 10, Role: "member", Enabled: false},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", 3, 10).
		UpdateColumn("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	administratorRole := AccessRole{Code: identity.AccessAdministratorRoleCode, Name: "Access administrator", Enabled: true, SystemManaged: true}
	if err := db.Create(&administratorRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AccessUserRole{UserID: 12, RoleID: administratorRole.ID}).Error; err != nil {
		t.Fatal(err)
	}
	teams := []Team{
		{ID: 20, WorkspaceID: 1, Name: "研发"},
		{ID: 21, WorkspaceID: 2, Name: "其他空间团队"},
	}
	if err := db.Create(&teams).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&TeamMember{WorkspaceID: 1, TeamID: 20, UserID: 10, Role: "lead"}).Error; err != nil {
		t.Fatal(err)
	}

	service, err := identity.NewBaselineDirectoryService(NewBaselineDirectoryRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	available, err := service.ListAvailableWorkspaces(context.Background(), 10)
	if err != nil || len(available) != 1 || available[0].ID != 1 {
		t.Fatalf("available workspaces=%+v err=%v", available, err)
	}
	administratorWorkspaces, err := service.ListAvailableWorkspaces(context.Background(), 12)
	if err != nil || len(administratorWorkspaces) != 2 || administratorWorkspaces[0].ID != 1 || administratorWorkspaces[1].ID != 3 {
		t.Fatalf("administrator workspaces=%+v err=%v", administratorWorkspaces, err)
	}
	page, err := service.SearchUsers(context.Background(), 1, identity.DirectoryUserFilter{})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != 10 {
		t.Fatalf("directory users=%+v err=%v", page, err)
	}
	otherWorkspace, err := service.SearchUsers(context.Background(), 3, identity.DirectoryUserFilter{})
	if err != nil || otherWorkspace.Total != 0 || len(otherWorkspace.Items) != 0 {
		t.Fatalf("other workspace users=%+v err=%v", otherWorkspace, err)
	}
	raw, err := json.Marshal(page.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "email") || strings.Contains(string(raw), "role") || strings.Contains(string(raw), "active") {
		t.Fatalf("minimal user projection leaked management fields: %s", raw)
	}
	hydrated, err := service.SearchUsers(context.Background(), 1, identity.DirectoryUserFilter{IDs: []uint{10}})
	if err != nil || hydrated.Total != 1 || len(hydrated.Items) != 1 || hydrated.Items[0].ID != 10 {
		t.Fatalf("selected user hydration=%+v err=%v", hydrated, err)
	}
	if len(hydrated.Items) != 1 || hydrated.Items[0].Email != "private@example.com" {
		t.Fatalf("precise directory lookup missing email: %+v", hydrated)
	}
	externalIDMustNotMatch, err := service.SearchUsers(context.Background(), 1, identity.DirectoryUserFilter{IDs: []uint{110}})
	if err != nil || externalIDMustNotMatch.Total != 0 || len(externalIDMustNotMatch.Items) != 0 {
		t.Fatalf("external user id unexpectedly matched internal id filter: %+v err=%v", externalIDMustNotMatch, err)
	}
	directoryTeams, err := service.ListTeams(context.Background(), 1)
	if err != nil || len(directoryTeams) != 1 || directoryTeams[0].ID != 20 {
		t.Fatalf("directory teams=%+v err=%v", directoryTeams, err)
	}
	members, err := service.ListTeamMembers(context.Background(), 1, 20)
	if err != nil || len(members) != 1 || members[0].ID != 10 || members[0].Role != "lead" {
		t.Fatalf("team members=%+v err=%v", members, err)
	}
}
