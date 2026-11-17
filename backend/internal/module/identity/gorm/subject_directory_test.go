package identitygorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func TestSubjectRepositoryUsesCanonicalPMUserIDsAndDynamicMemberships(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&User{}, &Workspace{}, &WorkspaceMember{}, &AccessRole{}, &AccessUserRole{},
		&AccessUserGroup{}, &AccessUserGroupBinding{},
	); err != nil {
		t.Fatal(err)
	}
	workspaces := []Workspace{{ID: 7, Name: "A", Slug: "a", Enabled: true}, {ID: 8, Name: "B", Slug: "b", Enabled: true}}
	users := []User{
		{ID: 11, ExternalSubject: "u11", Username: "same-a", DisplayName: "Same", Active: true, Status: "active"},
		{ID: 99, ExternalSubject: "u99", Username: "same-b", DisplayName: "Same", Active: true, Status: "active"},
		{ID: 12, ExternalSubject: "u12", Username: "disabled-member", DisplayName: "Disabled", Active: true, Status: "active"},
		{ID: 13, ExternalSubject: "u13", Username: "other-workspace", DisplayName: "Other", Active: true, Status: "active"},
	}
	if err := db.Create(&workspaces).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	// Legacy fixture only; the runtime User model intentionally has no external field.
	if err := db.Exec("ALTER TABLE users ADD COLUMN external_user_id bigint").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE users SET external_user_id = ? WHERE id = ?", 99, 11).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE users SET external_user_id = ? WHERE id = ?", 11, 99).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]WorkspaceMember{
		{WorkspaceID: 7, UserID: 11, Enabled: true}, {WorkspaceID: 7, UserID: 99, Enabled: true},
		{WorkspaceID: 7, UserID: 12, Enabled: false}, {WorkspaceID: 8, UserID: 13, Enabled: true},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&WorkspaceMember{}).Where("workspace_id = ? AND user_id = ?", 7, 12).
		Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	role := AccessRole{ID: 21, Code: "reviewer", Name: "Reviewer", Enabled: true}
	disabledRole := AccessRole{ID: 22, Code: "old", Name: "Old", Enabled: false}
	group := AccessUserGroup{ID: 31, Code: "design", Name: "Design", Enabled: true}
	if err := db.Create(&[]AccessRole{role, disabledRole}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AccessUserRole{UserID: 11, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AccessUserGroupBinding{UserID: 11, GroupID: group.ID}).Error; err != nil {
		t.Fatal(err)
	}

	service, err := identity.NewSubjectDirectoryService(NewSubjectRepository(db), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.SearchSubjects(context.Background(), 7, identity.SubjectSearch{
		Kind: identity.SubjectUser, Query: "same", Page: 1, PageSize: 1,
	})
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != "11" || !page.HasMore {
		t.Fatalf("canonical page=%+v err=%v", page, err)
	}
	page, err = service.SearchSubjects(context.Background(), 7, identity.SubjectSearch{
		Kind: identity.SubjectUser, Query: "same", Page: 2, PageSize: 1,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != "99" || page.HasMore {
		t.Fatalf("second page=%+v err=%v", page, err)
	}
	resolved, err := service.ResolveSubjects(context.Background(), 7, []identity.SubjectRef{{Kind: identity.SubjectUser, ID: "11"}})
	if err != nil || len(resolved) != 1 || resolved[0].ID != "11" {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	if _, err := service.ResolveSubjects(context.Background(), 7, []identity.SubjectRef{{Kind: identity.SubjectUser, ID: "12"}}); err == nil {
		t.Fatal("disabled workspace member resolved")
	}

	refs := []identity.SubjectRef{{Kind: identity.SubjectRole, ID: "21"}, {Kind: identity.SubjectUserGroup, ID: "31"}}
	matched, err := service.MatchSubjects(context.Background(), identity.SubjectMatchRequest{WorkspaceID: 7, UserID: 11, Subjects: refs})
	if err != nil || !matched.Matches[0].Matched || !matched.Matches[1].Matched {
		t.Fatalf("matches=%+v err=%v", matched, err)
	}
	if err := db.Where("user_id = ? AND role_id = ?", 11, 21).Delete(&AccessUserRole{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&AccessUserGroup{}).Where("id = ?", 31).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	matched, err = service.MatchSubjects(context.Background(), identity.SubjectMatchRequest{WorkspaceID: 7, UserID: 11, Subjects: refs})
	if err != nil || matched.Matches[0].Matched || matched.Matches[1].Matched {
		t.Fatalf("stale grants survived: %+v err=%v", matched, err)
	}
	other, err := service.MatchSubjects(context.Background(), identity.SubjectMatchRequest{
		WorkspaceID: 7, UserID: 13, Subjects: []identity.SubjectRef{{Kind: identity.SubjectUser, ID: "13"}},
	})
	if err != nil || other.Matches[0].Matched {
		t.Fatalf("cross-workspace user matched: %+v err=%v", other, err)
	}
}
