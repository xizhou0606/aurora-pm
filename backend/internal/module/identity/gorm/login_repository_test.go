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

func TestLoginRepositoryResolvesByCompanyAndName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &FeishuTenant{}, &AccessUserGroup{}, &AccessUserGroupBinding{}, &Workspace{}, &WorkspaceMember{}); err != nil {
		t.Fatal(err)
	}
	defaultWorkspace := Workspace{Name: "Default", Slug: "default", Enabled: true, IsDefault: true}
	if err := db.Create(&defaultWorkspace).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewLoginRepository(db)
	legacy := User{ID: 7, ExternalIssuer: "one2all", ExternalSubject: "one2all|7", Username: "alice", DisplayName: "Alice", Active: true, Role: "user"}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&User{ID: 8, ExternalIssuer: "one2all", ExternalSubject: "one2all|8", Username: "bob", DisplayName: "Bob", Active: true, Role: "user"}).Error; err != nil {
		t.Fatal(err)
	}
	profile := identity.FeishuIdentity{TenantKey: "tenant_pm", DisplayName: "Alice", AvatarURL: "https://avatar.test/alice.png", OpenID: "ou_alice"}

	first, err := repository.ProvisionFeishuUser(context.Background(), profile)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != legacy.ID || first.Username != "alice" || first.DisplayName != "Alice" {
		t.Fatalf("first user=%+v", first)
	}
	var tenant FeishuTenant
	if err := db.Where("tenant_key = ?", profile.TenantKey).First(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	var linkedCount int64
	if err := db.Model(&User{}).Where("feishu_tenant_id = ?", tenant.ID).Count(&linkedCount).Error; err != nil {
		t.Fatal(err)
	}
	if linkedCount != 2 {
		t.Fatalf("linked users=%d want=2", linkedCount)
	}
	var storedAlice User
	if err := db.Where("id = ?", first.ID).First(&storedAlice).Error; err != nil {
		t.Fatal(err)
	}
	if storedAlice.FeishuOpenID == nil || *storedAlice.FeishuOpenID != "ou_alice" {
		t.Fatalf("persisted open_id=%v", storedAlice.FeishuOpenID)
	}
	openIDs, err := repository.FeishuOpenIDs(context.Background(), []uint{first.ID, 8, 0})
	if err != nil || len(openIDs) != 1 || openIDs[first.ID] != "ou_alice" {
		t.Fatalf("lookup=%v err=%v", openIDs, err)
	}

	second, err := repository.ProvisionFeishuUser(context.Background(), identity.FeishuIdentity{
		TenantKey: profile.TenantKey, DisplayName: "Alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.DisplayName != "Alice" {
		t.Fatalf("updated user=%+v", second)
	}
	if err := db.Where("id = ?", first.ID).First(&storedAlice).Error; err != nil {
		t.Fatal(err)
	}
	if storedAlice.FeishuOpenID == nil || *storedAlice.FeishuOpenID != "ou_alice" {
		t.Fatalf("open_id was cleared on later login=%v", storedAlice.FeishuOpenID)
	}
	newUser, err := repository.ProvisionFeishuUser(context.Background(), identity.FeishuIdentity{
		TenantKey: profile.TenantKey, DisplayName: "New User",
	})
	if err != nil {
		t.Fatal(err)
	}
	reusedNewUser, err := repository.ProvisionFeishuUser(context.Background(), identity.FeishuIdentity{
		TenantKey: profile.TenantKey, DisplayName: "New User",
	})
	if err != nil || reusedNewUser.ID != newUser.ID {
		t.Fatalf("new user was not reused: first=%+v second=%+v err=%v", newUser, reusedNewUser, err)
	}
	if newUser.Username != identity.FeishuUsername(profile.TenantKey, "New User") {
		t.Fatalf("new user username=%q", newUser.Username)
	}
	if newUser.Status != identity.AccountPendingApproval || !newUser.Active {
		t.Fatalf("new Feishu user admission=%+v", newUser)
	}
	var newUserBindings int64
	if err := db.Model(&AccessUserGroupBinding{}).Where("user_id = ?", newUser.ID).Count(&newUserBindings).Error; err != nil {
		t.Fatal(err)
	}
	if newUserBindings != 1 {
		t.Fatalf("new Feishu user group bindings=%d want 1", newUserBindings)
	}
	var workspaceMembership WorkspaceMember
	if err := db.Where("workspace_id = ? AND user_id = ?", defaultWorkspace.ID, newUser.ID).First(&workspaceMembership).Error; err != nil {
		t.Fatalf("new Feishu user must join default workspace: %v", err)
	}
	if workspaceMembership.Role != "member" || !workspaceMembership.Enabled {
		t.Fatalf("new Feishu workspace membership=%+v", workspaceMembership)
	}
	if !db.Migrator().HasIndex(&User{}, "ux_users_feishu_tenant_display_name") {
		t.Fatal("missing company/name uniqueness index")
	}
	var duplicate User
	if err := db.Where("id = ?", newUser.ID).First(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	duplicate.ID = 0
	duplicate.Username = "another-username"
	duplicate.ExternalSubject = "another-subject"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("company/name duplicate was accepted")
	}
	if _, err := repository.ProvisionFeishuUser(context.Background(), identity.FeishuIdentity{TenantKey: "tenant_unknown", DisplayName: "Other"}); !errors.Is(err, identity.ErrLoginTenantNotRegistered) {
		t.Fatalf("unregistered tenant error=%v", err)
	}

	if err := db.Model(&FeishuTenant{}).Where("tenant_key = ?", profile.TenantKey).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ProvisionFeishuUser(context.Background(), profile); !errors.Is(err, identity.ErrLoginTenantDisabled) {
		t.Fatalf("disabled tenant error=%v", err)
	}
}
