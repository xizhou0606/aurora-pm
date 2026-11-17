package identitygorm

import (
	"context"
	"errors"
	"testing"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"golang.org/x/crypto/bcrypt"
)

func TestCompleteExternalRegistrationCreatesUserBindingAndCodeAtomicallyOnce(t *testing.T) {
	db := googleLoginDB(t)
	if err := db.AutoMigrate(&ExternalRegistrationTransaction{}, &IdentityOperationTransaction{}, &AccessUserGroup{}, &AccessUserGroupBinding{}); err != nil {
		t.Fatal(err)
	}
	repo := NewLoginRepository(db)
	now := time.Now().UTC()
	registration := identity.ExternalRegistration{TicketHash: "ticket", Provider: identity.FlowGoogle, Issuer: "https://accounts.google.com", Subject: "new-sub", Email: "new@example.com", ExpiresAt: now.Add(time.Minute), CreatedAt: now}
	if err := repo.CreateExternalRegistration(context.Background(), &registration); err != nil {
		t.Fatal(err)
	}
	user, err := repo.CompleteExternalRegistration(context.Background(), identity.ExternalRegistrationComplete{TicketHash: "ticket", Username: "new.user", DisplayName: "新用户", PasswordHash: "hash", LoginCodeHash: "code", Now: now, LoginCodeExpiresAt: now.Add(time.Minute)})
	if err != nil || user.ID == 0 || user.Username != "new.user" || user.DisplayName != "新用户" {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	if user.Status != identity.AccountPendingApproval || user.SessionVersion != 1 {
		t.Fatalf("new external registration must be pending approval: user=%+v", user)
	}
	var group AccessUserGroup
	if err := db.Where("code = ?", identity.DefaultUserGroupCode).First(&group).Error; err != nil {
		t.Fatal(err)
	}
	var binding AccessUserGroupBinding
	if err := db.Where("user_id = ? AND group_id = ?", user.ID, group.ID).First(&binding).Error; err != nil {
		t.Fatalf("new external registration must join default group: %v", err)
	}
	var workspace Workspace
	if err := db.Where("is_default = ?", true).First(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	var membership WorkspaceMember
	if err := db.Where("workspace_id = ? AND user_id = ?", workspace.ID, user.ID).First(&membership).Error; err != nil {
		t.Fatalf("new external registration must join default workspace: %v", err)
	}
	if membership.Role != "member" || !membership.Enabled {
		t.Fatalf("default workspace membership=%+v", membership)
	}
	if _, err := repo.CompleteExternalRegistration(context.Background(), identity.ExternalRegistrationComplete{TicketHash: "ticket", Username: "again", DisplayName: "重放", PasswordHash: "hash", LoginCodeHash: "other", Now: now, LoginCodeExpiresAt: now.Add(time.Minute)}); !errors.Is(err, identity.ErrExternalRegistrationInvalid) {
		t.Fatalf("replay err=%v", err)
	}
}

func TestCompleteExternalRegistrationRollsBackWithoutDefaultWorkspace(t *testing.T) {
	db := googleLoginDB(t)
	if err := db.AutoMigrate(&ExternalRegistrationTransaction{}, &IdentityOperationTransaction{}, &AccessUserGroup{}, &AccessUserGroupBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("is_default = ?", true).Delete(&Workspace{}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewLoginRepository(db)
	now := time.Now().UTC()
	registration := identity.ExternalRegistration{
		TicketHash: "missing-default-ticket", Provider: identity.FlowGoogle,
		Issuer: "https://accounts.google.com", Subject: "missing-default-subject",
		Email: "missing-default@example.com", ExpiresAt: now.Add(time.Minute), CreatedAt: now,
	}
	if err := repo.CreateExternalRegistration(context.Background(), &registration); err != nil {
		t.Fatal(err)
	}
	_, err := repo.CompleteExternalRegistration(context.Background(), identity.ExternalRegistrationComplete{
		TicketHash: "missing-default-ticket", Username: "missing.default", DisplayName: "缺少默认工作区",
		PasswordHash: "hash", LoginCodeHash: "missing-default-code", Now: now, LoginCodeExpiresAt: now.Add(time.Minute),
	})
	if !errors.Is(err, identity.ErrDefaultWorkspaceUnavailable) {
		t.Fatalf("registration err=%v", err)
	}
	var userCount int64
	if err := db.Model(&User{}).Where("username = ?", "missing.default").Count(&userCount).Error; err != nil {
		t.Fatal(err)
	}
	if userCount != 0 {
		t.Fatalf("orphaned users=%d", userCount)
	}
	var stored ExternalRegistrationTransaction
	if err := db.Where("ticket_hash = ?", registration.TicketHash).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ConsumedAt != nil {
		t.Fatalf("registration ticket was consumed despite rollback: %+v", stored)
	}
}

func TestAccountRepositoryProtectsFeishuDisplayNameAndLastLoginMethod(t *testing.T) {
	db := googleLoginDB(t)
	repo := NewLoginRepository(db)
	openID := "ou_feishu"
	user := User{ExternalSubject: "account-user", Username: "account-user", DisplayName: "Feishu Name", FeishuOpenID: &openID, Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProfile(context.Background(), user.ID, "Changed"); !errors.Is(err, identity.ErrIdentityDisplayNameReadOnly) {
		t.Fatalf("profile err=%v", err)
	}
	user2 := User{ExternalSubject: "external-only", Username: "external-only", Active: true}
	if err := db.Create(&user2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UserLoginIdentity{UserID: user2.ID, Provider: identity.FlowGoogle, Issuer: "https://accounts.google.com", Subject: "only", EmailSnapshot: "x@y.com"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.UnbindIdentity(context.Background(), user2.ID, identity.FlowGoogle); !errors.Is(err, identity.ErrIdentityLoginMethodRequired) {
		t.Fatalf("unbind err=%v", err)
	}
}

func TestAccountRepositoryCompletesRequiredPasswordChangeWithoutCurrentPassword(t *testing.T) {
	db := googleLoginDB(t)
	if err := db.AutoMigrate(&PasswordResetTransaction{}); err != nil {
		t.Fatal(err)
	}
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-private-value"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := User{
		ExternalSubject: "forced-reset-user", Username: "forced-reset-user",
		PasswordHash: string(oldHash), Active: true, PasswordResetRequired: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	reset := PasswordResetTransaction{
		TokenHash: "unused-reset-token", UserID: user.ID, RequestedByUserID: user.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour), CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&reset).Error; err != nil {
		t.Fatal(err)
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte("new-private-value"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := NewLoginRepository(db)
	if err := repository.UpdatePassword(context.Background(), user.ID, "", string(newHash)); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if user.PasswordResetRequired || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("new-private-value")) != nil {
		t.Fatalf("required password change was not completed: user=%+v", user)
	}
	if err := db.First(&reset, reset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reset.ConsumedAt == nil {
		t.Fatal("outstanding password reset link remained usable")
	}
}

func TestAccountRepositoryStillRequiresCurrentPasswordOutsideResetFlow(t *testing.T) {
	db := googleLoginDB(t)
	if err := db.AutoMigrate(&PasswordResetTransaction{}); err != nil {
		t.Fatal(err)
	}
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-private-value"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := User{ExternalSubject: "regular-user", Username: "regular-user", PasswordHash: string(oldHash), Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte("new-private-value"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	err = NewLoginRepository(db).UpdatePassword(context.Background(), user.ID, "", string(newHash))
	if !errors.Is(err, identity.ErrIdentityCurrentPassword) {
		t.Fatalf("missing current password err=%v", err)
	}
}

func TestCompleteExternalRegistrationNormalizesDuplicateProviderIdentity(t *testing.T) {
	db := googleLoginDB(t)
	if err := db.AutoMigrate(&ExternalRegistrationTransaction{}); err != nil {
		t.Fatal(err)
	}
	repo := NewLoginRepository(db)
	now := time.Now().UTC()
	existing := User{ExternalSubject: "existing", Username: "existing", Active: true}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UserLoginIdentity{UserID: existing.ID, Provider: identity.FlowTelegram, Issuer: "https://oauth.telegram.org", Subject: "same"}).Error; err != nil {
		t.Fatal(err)
	}
	registration := identity.ExternalRegistration{TicketHash: "duplicate-ticket", Provider: identity.FlowTelegram, Issuer: "https://oauth.telegram.org", Subject: "same", ExpiresAt: now.Add(time.Minute), CreatedAt: now}
	if err := repo.CreateExternalRegistration(context.Background(), &registration); err != nil {
		t.Fatal(err)
	}
	_, err := repo.CompleteExternalRegistration(context.Background(), identity.ExternalRegistrationComplete{TicketHash: "duplicate-ticket", Username: "new-user", DisplayName: "新用户", PasswordHash: "hash", LoginCodeHash: "new-code", Now: now, LoginCodeExpiresAt: now.Add(time.Minute)})
	if !errors.Is(err, identity.ErrLoginTelegramIdentityConflict) {
		t.Fatalf("err=%v", err)
	}
}

func TestUnbindExternalRegistrationReleasesLegacySubjectForFutureRegistration(t *testing.T) {
	db := googleLoginDB(t)
	if err := db.AutoMigrate(&ExternalRegistrationTransaction{}, &IdentityOperationTransaction{}, &AccessUserGroup{}, &AccessUserGroupBinding{}); err != nil {
		t.Fatal(err)
	}
	repo := NewLoginRepository(db)
	now := time.Now().UTC()
	provider := identity.ExternalRegistration{
		TicketHash: "first-ticket", Provider: identity.FlowGoogle,
		Issuer: "https://accounts.google.com", Subject: "reusable-google-subject",
		Email: "first@example.com", ExpiresAt: now.Add(time.Minute), CreatedAt: now,
	}
	if err := repo.CreateExternalRegistration(context.Background(), &provider); err != nil {
		t.Fatal(err)
	}
	first, err := repo.CompleteExternalRegistration(context.Background(), identity.ExternalRegistrationComplete{
		TicketHash: "first-ticket", Username: "first.user", DisplayName: "首位用户", PasswordHash: "hash",
		LoginCodeHash: "first-code", Now: now, LoginCodeExpiresAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	originalSubject := externalRegistrationSubject(provider.Provider, provider.Issuer, provider.Subject)
	if err := repo.UnbindIdentity(context.Background(), first.ID, identity.FlowGoogle); err != nil {
		t.Fatal(err)
	}
	var released User
	if err := db.First(&released, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if released.ExternalSubject == originalSubject || released.ExternalSubject == "" {
		t.Fatalf("legacy external subject was not safely released: %q", released.ExternalSubject)
	}
	var bindingCount int64
	if err := db.Model(&UserLoginIdentity{}).Where("user_id = ? AND provider = ?", first.ID, identity.FlowGoogle).Count(&bindingCount).Error; err != nil {
		t.Fatal(err)
	}
	if bindingCount != 0 {
		t.Fatalf("binding count=%d", bindingCount)
	}

	secondRegistration := provider
	secondRegistration.ID = 0
	secondRegistration.TicketHash = "second-ticket"
	secondRegistration.Email = "second@example.com"
	if err := repo.CreateExternalRegistration(context.Background(), &secondRegistration); err != nil {
		t.Fatal(err)
	}
	second, err := repo.CompleteExternalRegistration(context.Background(), identity.ExternalRegistrationComplete{
		TicketHash: "second-ticket", Username: "second.user", DisplayName: "第二位用户", PasswordHash: "hash",
		LoginCodeHash: "second-code", Now: now, LoginCodeExpiresAt: now.Add(time.Minute),
	})
	if err != nil || second.ID == 0 || second.ID == first.ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestCompleteExternalRegistrationRepairsPreviouslyUnboundLegacySubject(t *testing.T) {
	for _, test := range []struct {
		provider string
		issuer   string
	}{
		{provider: identity.FlowGoogle, issuer: "https://accounts.google.com"},
		{provider: identity.FlowTelegram, issuer: "https://oauth.telegram.org"},
	} {
		t.Run(test.provider, func(t *testing.T) {
			db := googleLoginDB(t)
			if err := db.AutoMigrate(&ExternalRegistrationTransaction{}, &IdentityOperationTransaction{}, &AccessUserGroup{}, &AccessUserGroupBinding{}); err != nil {
				t.Fatal(err)
			}
			repo := NewLoginRepository(db)
			now := time.Now().UTC()
			registration := identity.ExternalRegistration{
				TicketHash: "repair-ticket", Provider: test.provider,
				Issuer: test.issuer, Subject: "previously-unbound-subject",
				Email: "new@example.com", ExpiresAt: now.Add(time.Minute), CreatedAt: now,
			}
			originalSubject := externalRegistrationSubject(registration.Provider, registration.Issuer, registration.Subject)
			previous := User{
				ExternalIssuer: externalRegistrationIssuer, ExternalSubject: originalSubject,
				Username: "previous.user", DisplayName: "Previous User", PasswordHash: "hash",
				Role: "user", Active: true, Status: string(identity.AccountPendingApproval), SessionVersion: 1,
			}
			if err := db.Create(&previous).Error; err != nil {
				t.Fatal(err)
			}
			if err := repo.CreateExternalRegistration(context.Background(), &registration); err != nil {
				t.Fatal(err)
			}
			created, err := repo.CompleteExternalRegistration(context.Background(), identity.ExternalRegistrationComplete{
				TicketHash: "repair-ticket", Username: "fresh.user", DisplayName: "新用户", PasswordHash: "hash",
				LoginCodeHash: "repair-code", Now: now, LoginCodeExpiresAt: now.Add(time.Minute),
			})
			if err != nil || created.ID == 0 || created.ID == previous.ID {
				t.Fatalf("created=%+v err=%v", created, err)
			}
			var repaired User
			if err := db.First(&repaired, previous.ID).Error; err != nil {
				t.Fatal(err)
			}
			if repaired.ExternalSubject == originalSubject || repaired.ExternalSubject == "" {
				t.Fatalf("stale external subject was not repaired: %q", repaired.ExternalSubject)
			}
		})
	}
}

func TestUnbindIdentityPreservesNonRegistrationLegacySubject(t *testing.T) {
	db := googleLoginDB(t)
	repo := NewLoginRepository(db)
	user := User{
		ExternalIssuer: "one2all", ExternalSubject: "one2all|42",
		Username: "shared.user", DisplayName: "Shared User", PasswordHash: "hash",
		Role: "user", Active: true, Status: string(identity.AccountActive), SessionVersion: 1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UserLoginIdentity{
		UserID: user.ID, Provider: identity.FlowGoogle,
		Issuer: "https://accounts.google.com", Subject: "shared-google-subject", EmailSnapshot: "shared@example.com",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.UnbindIdentity(context.Background(), user.ID, identity.FlowGoogle); err != nil {
		t.Fatal(err)
	}
	var preserved User
	if err := db.First(&preserved, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if preserved.ExternalIssuer != user.ExternalIssuer || preserved.ExternalSubject != user.ExternalSubject {
		t.Fatalf("non-registration identity changed: %+v", preserved)
	}
}
