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

func TestGoogleLoginRepositoryBindsOnlyOneActiveExactEmailUser(t *testing.T) {
	db := googleLoginDB(t)
	repository := NewLoginRepository(db)
	alice := User{ID: 7, Username: "alice", DisplayName: "Alice", Email: "Alice@Example.com", Active: true}
	if err := db.Create(&alice).Error; err != nil {
		t.Fatal(err)
	}
	provider := identity.GoogleIdentity{
		Issuer: "https://accounts.google.com", Subject: "google-7",
		Email: " alice@example.com ", EmailVerified: true,
	}
	if err := db.Create(&UserLoginIdentity{UserID: alice.ID, Provider: identity.FlowGoogle, Issuer: provider.Issuer, Subject: provider.Subject, EmailSnapshot: "alice@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	resolved, err := repository.ResolveOrBindGoogleUser(context.Background(), provider)
	if err != nil || resolved.ID != alice.ID {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	var binding UserLoginIdentity
	if err := db.First(&binding).Error; err != nil {
		t.Fatal(err)
	}
	if binding.UserID != alice.ID || binding.Provider != identity.FlowGoogle || binding.Subject != "google-7" || binding.EmailSnapshot != "alice@example.com" {
		t.Fatalf("binding=%+v", binding)
	}

	// Existing subject binding remains authoritative if the provider email or
	// local email later changes.
	if err := db.Model(&alice).Update("email", "renamed@example.com").Error; err != nil {
		t.Fatal(err)
	}
	provider.Email = "new-google-email@example.com"
	reused, err := repository.ResolveOrBindGoogleUser(context.Background(), provider)
	if err != nil || reused.ID != alice.ID {
		t.Fatalf("reused=%+v err=%v", reused, err)
	}

	var userCount int64
	if err := db.Model(&User{}).Count(&userCount).Error; err != nil || userCount != 1 {
		t.Fatalf("users=%d err=%v", userCount, err)
	}
}

func TestGoogleLoginRepositoryDoesNotProvisionAndRejectsAmbiguousOrDisabledUsers(t *testing.T) {
	t.Run("unknown email", func(t *testing.T) {
		repository := NewLoginRepository(googleLoginDB(t))
		_, err := repository.ResolveOrBindGoogleUser(context.Background(), identity.GoogleIdentity{
			Issuer: "https://accounts.google.com", Subject: "unknown", Email: "unknown@example.com", EmailVerified: true,
		})
		if !errors.Is(err, identity.ErrLoginUserNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ambiguous active email", func(t *testing.T) {
		db := googleLoginDB(t)
		if err := db.Create(&[]User{
			{ExternalSubject: "first", Username: "first", Email: "same@example.com", Active: true},
			{ExternalSubject: "second", Username: "second", Email: "SAME@example.com", Active: true},
		}).Error; err != nil {
			t.Fatal(err)
		}
		_, err := NewLoginRepository(db).ResolveOrBindGoogleUser(context.Background(), identity.GoogleIdentity{
			Issuer: "https://accounts.google.com", Subject: "ambiguous", Email: "same@example.com", EmailVerified: true,
		})
		if !errors.Is(err, identity.ErrLoginUserNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("disabled existing binding", func(t *testing.T) {
		db := googleLoginDB(t)
		user := User{ExternalSubject: "disabled", Username: "disabled", Email: "disabled@example.com", Active: true}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&user).Update("active", false).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&UserLoginIdentity{UserID: user.ID, Provider: identity.FlowGoogle, Issuer: "https://accounts.google.com", Subject: "disabled", EmailSnapshot: user.Email}).Error; err != nil {
			t.Fatal(err)
		}
		_, err := NewLoginRepository(db).ResolveOrBindGoogleUser(context.Background(), identity.GoogleIdentity{
			Issuer: "https://accounts.google.com", Subject: "disabled", Email: user.Email, EmailVerified: true,
		})
		if !errors.Is(err, identity.ErrLoginUserDisabled) {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestGoogleLoginRepositoryDoesNotBindASecondIdentityByEmail(t *testing.T) {
	db := googleLoginDB(t)
	user := User{Username: "alice", Email: "alice@example.com", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewLoginRepository(db)
	first := identity.GoogleIdentity{Issuer: "https://accounts.google.com", Subject: "first", Email: user.Email, EmailVerified: true}
	if err := db.Create(&UserLoginIdentity{UserID: user.ID, Provider: identity.FlowGoogle, Issuer: first.Issuer, Subject: first.Subject, EmailSnapshot: first.Email}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ResolveOrBindGoogleUser(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Subject = "second"
	if _, err := repository.ResolveOrBindGoogleUser(context.Background(), second); !errors.Is(err, identity.ErrLoginUserNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func googleLoginDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}, &UserLoginIdentity{}, &LoginTransaction{}, &LoginCode{}, &Workspace{}, &WorkspaceMember{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Workspace{Name: "Default", Slug: "default", Enabled: true, IsDefault: true}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
