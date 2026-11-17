package identitygorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func TestSubjectAdmissionMatchesCanonicalAdministratorWorkspaceAdmission(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &Workspace{}, &WorkspaceMember{}, &AccessRole{}, &AccessUserRole{}); err != nil {
		t.Fatal(err)
	}
	user := User{ID: 110, Username: "sysadmin", Active: true, Status: "active"}
	workspace := Workspace{ID: 2, Name: "jiguang", Slug: "jiguang", Enabled: true}
	role := AccessRole{ID: 1, Code: identity.AccessAdministratorRoleCode, Name: "Administrator", Enabled: true, SystemManaged: true}
	for _, row := range []any{&user, &workspace, &role, &AccessUserRole{UserID: 110, RoleID: 1}} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	service, err := identity.NewSubjectDirectoryService(NewSubjectRepository(db), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := identity.SubjectMatchRequest{WorkspaceID: 2, UserID: 110, Subjects: []identity.SubjectRef{
		{Kind: identity.SubjectUser, ID: "110"}, {Kind: identity.SubjectUser, ID: "111"},
	}}
	assertSelf := func(want bool) {
		t.Helper()
		result, err := service.MatchSubjects(context.Background(), request)
		if err != nil || len(result.Matches) != 2 || result.Matches[0].Matched != want || result.Matches[1].Matched {
			t.Fatalf("matches=%+v err=%v want self=%v", result, err, want)
		}
	}
	assertSelf(true)
	for _, status := range []string{"banned", "pending_approval", "rejected"} {
		if err := db.Model(&User{}).Where("id = ?", user.ID).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		assertSelf(false)
	}
	if err := db.Model(&User{}).Where("id = ?", user.ID).Update("status", "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Workspace{}).Where("id = ?", workspace.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	assertSelf(false)
	if err := db.Model(&Workspace{}).Where("id = ?", workspace.ID).Update("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&AccessUserRole{}, "user_id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertSelf(false)
}
