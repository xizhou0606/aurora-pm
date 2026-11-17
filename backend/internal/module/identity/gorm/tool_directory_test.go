package identitygorm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func TestToolDirectoryScopesMinimalUsersAndTeamMembersToWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:identity_tool_directory?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &Workspace{}, &WorkspaceMember{}, &Team{}, &TeamMember{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]Workspace{{ID: 7, Name: "A", Slug: "a", Enabled: true}, {ID: 8, Name: "B", Slug: "b", Enabled: true}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]User{
		{ID: 1, ExternalSubject: "local:a", Username: "alpha", DisplayName: "Alpha", Email: "private@example.test", Active: true, Status: string(identity.AccountActive)},
		{ID: 2, ExternalSubject: "local:b", Username: "beta", DisplayName: "Beta", Active: true, Status: string(identity.AccountActive)},
		{ID: 3, ExternalSubject: "local:c", Username: "disabled", DisplayName: "Disabled", Active: false, Status: string(identity.AccountBanned)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]WorkspaceMember{
		{WorkspaceID: 7, UserID: 1, Role: "member", Enabled: true},
		{WorkspaceID: 8, UserID: 2, Role: "member", Enabled: true},
		{WorkspaceID: 7, UserID: 3, Role: "member", Enabled: true},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]Team{{ID: 11, WorkspaceID: 7, Name: "A"}, {ID: 12, WorkspaceID: 8, Name: "B"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]TeamMember{
		{WorkspaceID: 7, TeamID: 11, UserID: 1, Role: "admin"},
		{WorkspaceID: 8, TeamID: 12, UserID: 2, Role: "member"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	service, err := identity.NewToolDirectoryService(NewToolDirectoryRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.CurrentWorkspaceUser(context.Background(), 7, 2)
	if err != nil || current.ID != 2 || current.Username != "beta" {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	users, err := service.SearchWorkspaceUsers(context.Background(), 7, identity.ToolUserFilter{Query: "Al", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if users.Total != 1 || len(users.Items) != 1 || users.Items[0].ID != 1 {
		t.Fatalf("users=%+v", users)
	}
	workspaceMembers, err := service.ListWorkspaceMembers(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if workspaceMembers.Total != 1 || len(workspaceMembers.Items) != 1 || workspaceMembers.Items[0].ID != 1 {
		t.Fatalf("workspace members=%+v", workspaceMembers)
	}
	payload, err := json.Marshal(workspaceMembers)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private@example.test", "email", "token", "role", "permission", "group"} {
		if strings.Contains(strings.ToLower(string(payload)), forbidden) {
			t.Fatalf("workspace member payload leaked %q: %s", forbidden, payload)
		}
	}
	otherWorkspaceMembers, err := service.ListWorkspaceMembers(context.Background(), 8)
	if err != nil {
		t.Fatal(err)
	}
	if otherWorkspaceMembers.Total != 1 || len(otherWorkspaceMembers.Items) != 1 || otherWorkspaceMembers.Items[0].ID != 2 {
		t.Fatalf("other workspace members=%+v", otherWorkspaceMembers)
	}
	members, err := service.ListTeamMembers(context.Background(), 7, 11)
	if err != nil {
		t.Fatal(err)
	}
	if members.Total != 1 || len(members.Items) != 1 || members.Items[0].ID != 1 || members.Items[0].Role != "admin" {
		t.Fatalf("members=%+v", members)
	}
	other, err := service.ListTeamMembers(context.Background(), 7, 12)
	if err != nil {
		t.Fatal(err)
	}
	if other.Total != 0 || len(other.Items) != 0 {
		t.Fatalf("cross-workspace members=%+v", other)
	}
}
