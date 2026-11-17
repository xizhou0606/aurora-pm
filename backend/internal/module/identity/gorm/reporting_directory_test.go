package identitygorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func newReportingDirectoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &Workspace{}, &WorkspaceMember{},
		&AccessUserGroup{}, &AccessUserGroupBinding{}, &AccessRole{}, &AccessUserRole{},
		&FeishuDepartment{}); err != nil {
		t.Fatal(err)
	}
	// users.feishu_job_title belongs to the Feishu-contact sync migration; the
	// GORM User struct does not own the column, so the test schema mirrors it.
	if err := db.Exec("ALTER TABLE users ADD COLUMN feishu_job_title TEXT NOT NULL DEFAULT ''").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]Workspace{{ID: 7, Name: "A", Slug: "a", Enabled: true}, {ID: 8, Name: "B", Slug: "b", Enabled: true}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]User{
		{ID: 1, ExternalSubject: "local:a", Username: "alpha", DisplayName: "Alpha", Active: true, Status: string(identity.AccountActive)},
		{ID: 2, ExternalSubject: "local:b", Username: "beta", DisplayName: "Beta", Active: true, Status: string(identity.AccountActive)},
		{ID: 3, ExternalSubject: "local:c", Username: "banned", DisplayName: "Banned", Active: false, Status: string(identity.AccountBanned)},
		{ID: 4, ExternalSubject: "local:d", Username: "gamma", DisplayName: "Gamma", Active: true, Status: string(identity.AccountActive)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]WorkspaceMember{
		{WorkspaceID: 7, UserID: 1, Role: "member", Enabled: true},
		{WorkspaceID: 7, UserID: 2, Role: "member", Enabled: true},
		{WorkspaceID: 7, UserID: 3, Role: "member", Enabled: true},
		{WorkspaceID: 7, UserID: 4, Role: "member", Enabled: true},
		{WorkspaceID: 8, UserID: 1, Role: "member", Enabled: true},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]AccessUserGroup{
		{ID: 21, Code: "dev", Name: "研发组", Enabled: true},
		{ID: 22, Code: "qa", Name: "测试组", Enabled: true},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]AccessUserGroupBinding{
		{UserID: 1, GroupID: 21},
		{UserID: 2, GroupID: 22},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]AccessRole{{ID: 31, Code: "backend", Name: "后端", Enabled: true}, {ID: 32, Code: "retired", Name: "已停用", Enabled: true}}).Error; err != nil {
		t.Fatal(err)
	}
	// A false bool is a zero value, so GORM omits it on batch insert and the
	// column default applies; disable the second role explicitly.
	if err := db.Model(&AccessRole{ID: 32}).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]AccessUserRole{{UserID: 1, RoleID: 31}, {UserID: 1, RoleID: 32}}).Error; err != nil {
		t.Fatal(err)
	}
	// Job titles cover the role priority states: alpha holds a granted role
	// (permission wins, title still passes through), beta only a title
	// (fallback), gamma neither (empty).
	for _, seed := range []struct {
		userID uint
		title  string
	}{
		{userID: 1, title: "后端负责人"},
		{userID: 2, title: "测试工程师"},
	} {
		if err := db.Exec("UPDATE users SET feishu_job_title = ? WHERE id = ?", seed.title, seed.userID).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestReportingDirectoryGroupsActiveMembersByUserGroup(t *testing.T) {
	service, err := identity.NewDirectoryService(NewReportingDirectoryRepository(newReportingDirectoryDB(t)))
	if err != nil {
		t.Fatal(err)
	}
	members, err := service.ListWorkspaceMembers(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	// Workspace 7 has four memberships but the banned account is excluded;
	// workspace 8 must not leak into the projection.
	if len(members) != 3 {
		t.Fatalf("members=%+v", members)
	}
	var alpha, beta *identity.DirectoryMember
	for index := range members {
		switch members[index].Username {
		case "alpha":
			alpha = &members[index]
		case "beta":
			beta = &members[index]
		}
	}
	if alpha == nil || beta == nil {
		t.Fatalf("members=%+v", members)
	}
	if alpha.GroupID != 21 || alpha.GroupName != "研发组" {
		t.Fatalf("alpha=%+v", alpha)
	}
	if alpha.FactID != 1 {
		t.Fatalf("alpha fact id = %d, want 1", alpha.FactID)
	}
	if len(alpha.RoleNames) != 1 || alpha.RoleNames[0] != "后端" {
		t.Fatalf("alpha roles=%+v", alpha.RoleNames)
	}
	// Compatibility FactID always mirrors the canonical internal users.id.
	if beta.FactID != beta.ID || beta.GroupCode != "qa" {
		t.Fatalf("beta=%+v", beta)
	}
}

func TestReportingDirectoryFindMemberByFactID(t *testing.T) {
	service, err := identity.NewDirectoryService(NewReportingDirectoryRepository(newReportingDirectoryDB(t)))
	if err != nil {
		t.Fatal(err)
	}
	member, err := service.FindWorkspaceMember(context.Background(), 7, 1)
	if err != nil || member.Username != "alpha" {
		t.Fatalf("member=%+v err=%v", member, err)
	}
	if _, err := service.FindWorkspaceMember(context.Background(), 7, 999); err != identity.ErrReportingDirectoryMemberNotFound {
		t.Fatalf("err=%v, want member not found", err)
	}
}

// TestReportingDirectoryRolePriorityFallback locks the role priority rule:
// granted permission roles win, the synced Feishu job title is the fallback
// for members without any granted role, and members with neither keep an
// empty list (the frontend renders 暂无角色). The job title always passes
// through so surfaces can label the fallback source.
func TestReportingDirectoryRolePriorityFallback(t *testing.T) {
	service, err := identity.NewDirectoryService(NewReportingDirectoryRepository(newReportingDirectoryDB(t)))
	if err != nil {
		t.Fatal(err)
	}
	members, err := service.ListWorkspaceMembers(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	byUsername := map[string]*identity.DirectoryMember{}
	for index := range members {
		byUsername[members[index].Username] = &members[index]
	}
	// Permission role wins even though a job title exists; the title still
	// passes through as the fallback-source fact.
	if alpha := byUsername["alpha"]; alpha == nil ||
		len(alpha.RoleNames) != 1 || alpha.RoleNames[0] != "后端" || alpha.JobTitle != "后端负责人" {
		t.Fatalf("alpha=%+v", byUsername["alpha"])
	}
	// No granted role: the job title becomes the single role name.
	if beta := byUsername["beta"]; beta == nil ||
		len(beta.RoleNames) != 1 || beta.RoleNames[0] != "测试工程师" || beta.JobTitle != "测试工程师" {
		t.Fatalf("beta=%+v", byUsername["beta"])
	}
	// Neither granted roles nor a job title: the list stays empty (not nil)
	// so the HTTP contract keeps its empty-array shape.
	if gamma := byUsername["gamma"]; gamma == nil ||
		gamma.RoleNames == nil || len(gamma.RoleNames) != 0 || gamma.JobTitle != "" {
		t.Fatalf("gamma=%+v", byUsername["gamma"])
	}
}
