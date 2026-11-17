package projectgorm

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"gorm.io/gorm"
)

func openAdminCheckerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&identitygorm.User{},
		&identitygorm.WorkspaceMember{},
		&identitygorm.AccessRole{},
		&identitygorm.AccessUserRole{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedUser(t *testing.T, db *gorm.DB, id uint, username string, active bool, status string) {
	t.Helper()
	seedUserWithRole(t, db, id, username, "user", active, status)
}

func seedUserWithRole(t *testing.T, db *gorm.DB, id uint, username, role string, active bool, status string) {
	t.Helper()
	if err := db.Create(&identitygorm.User{
		ID:              id,
		ExternalSubject: fmt.Sprintf("user-%d", id),
		Username:        username,
		Role:            role,
		Active:          active,
		Status:          status,
	}).Error; err != nil {
		t.Fatalf("seed user %d: %v", id, err)
	}
}

func TestAdminRoleCheckerIsAccessAdministrator(t *testing.T) {
	db := openAdminCheckerDB(t)
	checker := NewAdminRoleChecker(db)
	ctx := context.Background()

	// User 1: active, workspace member, has access-administrator role.
	seedUser(t, db, 1, "admin", true, "active")
	// User 2: active, workspace member, no admin role.
	seedUser(t, db, 2, "regular", true, "active")
	// User 644: inactive, workspace member, has admin role.
	seedUser(t, db, 644, "inactive", false, "banned")

	db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 10, UserID: 1, Enabled: true})
	db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 10, UserID: 2, Enabled: true})
	db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 10, UserID: 644, Enabled: true})

	var role identitygorm.AccessRole
	db.Where("code = ?", "access-administrator").FirstOrCreate(&role, identitygorm.AccessRole{Code: "access-administrator", Name: "Access Admin"})

	db.Create(&identitygorm.AccessUserRole{UserID: 1, RoleID: role.ID})
	db.Create(&identitygorm.AccessUserRole{UserID: 644, RoleID: role.ID})

	t.Run("active admin in workspace", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 10, 1)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !ok {
			t.Fatal("active admin should be recognized")
		}
	})

	t.Run("non-admin user", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 10, 2)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("non-admin user should not be admin")
		}
	})

	t.Run("inactive user even with role", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 10, 644)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("inactive user should not be admin")
		}
	})

	t.Run("admin in wrong workspace", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 20, 1)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("admin not in workspace 20")
		}
	})

	t.Run("zero userID", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 10, 0)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("zero userID should not be admin")
		}
	})

	t.Run("zero workspaceID", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 0, 1)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("zero workspaceID should not be admin")
		}
	})
}

func TestAdminRoleCheckerSuperAdminBypassesWorkspaceMembership(t *testing.T) {
	db := openAdminCheckerDB(t)
	checker := NewAdminRoleChecker(db)
	ctx := context.Background()

	// A super_admin that is active but has NO workspace_members row.
	seedUserWithRole(t, db, 109, "admin", "super_admin", true, "active")
	// A super_admin that is inactive/banned.
	seedUserWithRole(t, db, 644, "banned-admin", "super_admin", false, "banned")

	t.Run("active super_admin without workspace membership is admin", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 2, 109)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !ok {
			t.Fatal("active super_admin must be recognized as admin without a workspace_members row")
		}
	})

	t.Run("inactive super_admin is not admin", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 2, 644)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("inactive/banned super_admin must not be recognized as admin")
		}
	})

	t.Run("zero workspace still rejects", func(t *testing.T) {
		ok, err := checker.IsAccessAdministrator(ctx, 0, 109)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("zero workspace must not be admin")
		}
	})
}

func TestAdminRoleCheckerIsActive(t *testing.T) {
	db := openAdminCheckerDB(t)
	checker := NewAdminRoleChecker(db)
	ctx := context.Background()

	seedUser(t, db, 927, "u927", true, "active")
	seedUser(t, db, 644, "banned", false, "banned")
	seedUser(t, db, 355, "u355", true, "active")

	db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 10, UserID: 927, Enabled: true})
	db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 10, UserID: 644, Enabled: true})

	t.Run("active workspace member", func(t *testing.T) {
		ok, err := checker.IsActive(ctx, 10, 927)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !ok {
			t.Fatal("user should be active")
		}
	})

	t.Run("inactive user even if workspace member", func(t *testing.T) {
		ok, err := checker.IsActive(ctx, 10, 644)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("banned user should not be active")
		}
	})

	t.Run("active user not in workspace", func(t *testing.T) {
		ok, err := checker.IsActive(ctx, 10, 355)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("user not in workspace should not be active")
		}
	})

	t.Run("zero workspace", func(t *testing.T) {
		_, err := checker.IsActive(ctx, 0, 242)
		if err == nil {
			t.Fatal("expected error for zero workspace")
		}
	})
}
