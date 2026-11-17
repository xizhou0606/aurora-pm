package identitygorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func TestWorkspaceMembershipServiceEnforcesWorkspaceVisibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &Workspace{}, &WorkspaceMember{}, &AccessRole{}, &AccessUserRole{}); err != nil {
		t.Fatal(err)
	}
	user := User{Username: "member", DisplayName: "Member", Active: true, Status: string(identity.AccountActive)}
	workspace := Workspace{Name: "PM", Slug: "pm", Enabled: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	service, err := identity.NewWorkspaceMembershipService(NewWorkspaceMembershipRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyWorkspaceMembership(context.Background(), user.ID, workspace.ID); !errors.Is(err, identity.ErrWorkspaceMembershipInactive) {
		t.Fatalf("user without workspace membership error=%v", err)
	}
	membership := WorkspaceMember{UserID: user.ID, WorkspaceID: workspace.ID, Enabled: true}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyWorkspaceMembership(context.Background(), user.ID, workspace.ID); err != nil {
		t.Fatalf("enabled workspace member rejected: %v", err)
	}
	if err := db.Model(&membership).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyWorkspaceMembership(context.Background(), user.ID, workspace.ID); !errors.Is(err, identity.ErrWorkspaceMembershipInactive) {
		t.Fatalf("disabled workspace membership error=%v", err)
	}

	checks := []struct {
		name   string
		update func() error
	}{
		{name: "workspace disabled", update: func() error {
			return db.Model(&workspace).Update("enabled", false).Error
		}},
		{name: "user banned", update: func() error {
			if err := db.Model(&workspace).Update("enabled", true).Error; err != nil {
				return err
			}
			return db.Model(&user).Updates(map[string]any{"active": true, "status": string(identity.AccountBanned)}).Error
		}},
		{name: "user inactive", update: func() error {
			return db.Model(&user).Updates(map[string]any{"active": false, "status": string(identity.AccountActive)}).Error
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.update(); err != nil {
				t.Fatal(err)
			}
			if err := service.VerifyWorkspaceMembership(context.Background(), user.ID, workspace.ID); !errors.Is(err, identity.ErrWorkspaceMembershipInactive) {
				t.Fatalf("membership error=%v", err)
			}
		})
	}
}

func TestWorkspaceMembershipServiceAllowsAccessAdministratorWithoutMembership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &Workspace{}, &WorkspaceMember{}, &AccessRole{}, &AccessUserRole{}); err != nil {
		t.Fatal(err)
	}
	administrator := User{Username: "administrator", DisplayName: "Administrator", Active: true, Status: string(identity.AccountActive)}
	workspace := Workspace{Name: "PM", Slug: "pm", Enabled: true}
	role := AccessRole{Code: identity.AccessAdministratorRoleCode, Name: "Access administrator", Enabled: true, SystemManaged: true}
	for _, record := range []any{&administrator, &workspace, &role} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&AccessUserRole{UserID: administrator.ID, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}
	service, err := identity.NewWorkspaceMembershipService(NewWorkspaceMembershipRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyWorkspaceMembership(context.Background(), administrator.ID, workspace.ID); err != nil {
		t.Fatalf("access administrator rejected: %v", err)
	}
}

func TestWorkspaceMembershipServiceRejectsMissingIdentifiers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.NewWorkspaceMembershipService(NewWorkspaceMembershipRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyWorkspaceMembership(context.Background(), 0, 1); !errors.Is(err, identity.ErrWorkspaceMembershipInactive) {
		t.Fatalf("membership error=%v", err)
	}
}
