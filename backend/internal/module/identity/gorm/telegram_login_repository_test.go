package identitygorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func TestTelegramLoginRepositoryResolvesOnlyExistingSubjectBinding(t *testing.T) {
	db := googleLoginDB(t)
	repository := NewLoginRepository(db)
	user := User{ExternalSubject: "legacy-telegram", Username: "local-user", DisplayName: "Different Name", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	provider := identity.TelegramIdentity{
		Issuer: "https://oauth.telegram.org", Subject: "telegram-subject", TelegramUserID: 987654321,
		Name: "Same Telegram Name", PreferredUsername: "local-user", Nonce: "verified-nonce",
	}

	// Matching a PM username is deliberately insufficient.
	if _, err := repository.ResolveTelegramUser(context.Background(), provider); !errors.Is(err, identity.ErrLoginUserNotFound) {
		t.Fatalf("unbound username match err=%v", err)
	}
	if err := db.Create(&UserLoginIdentity{
		UserID: user.ID, Provider: identity.FlowTelegram,
		Issuer: provider.Issuer, Subject: provider.Subject, EmailSnapshot: "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	resolved, err := repository.ResolveTelegramUser(context.Background(), provider)
	if err != nil || resolved.ID != user.ID {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	var binding UserLoginIdentity
	if err := db.Where("provider = ? AND subject = ?", identity.FlowTelegram, provider.Subject).First(&binding).Error; err != nil {
		t.Fatal(err)
	}
	if binding.TelegramUserID == nil || *binding.TelegramUserID != provider.TelegramUserID {
		t.Fatalf("verified Telegram uid was not persisted: %+v", binding)
	}
	provider.TelegramUserID++
	if _, err := repository.ResolveTelegramUser(context.Background(), provider); !errors.Is(err, identity.ErrLoginTelegramIdentityConflict) {
		t.Fatalf("changed Telegram uid err=%v", err)
	}
}

func TestTelegramLoginRepositoryKeepsTelegramUIDGloballyUniqueWithoutBindingByUID(t *testing.T) {
	db := googleLoginDB(t)
	repository := NewLoginRepository(db)
	users := []User{
		{ExternalSubject: "telegram-one", Username: "telegram-one", Active: true},
		{ExternalSubject: "telegram-two", Username: "telegram-two", Active: true},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	bindings := []UserLoginIdentity{
		{UserID: users[0].ID, Provider: identity.FlowTelegram, Issuer: "https://oauth.telegram.org", Subject: "subject-one"},
		{UserID: users[1].ID, Provider: identity.FlowTelegram, Issuer: "https://oauth.telegram.org", Subject: "subject-two"},
	}
	if err := db.Create(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ResolveTelegramUser(context.Background(), identity.TelegramIdentity{
		Issuer: "https://oauth.telegram.org", Subject: "subject-one", TelegramUserID: 777,
	}); err != nil {
		t.Fatal(err)
	}
	_, err := repository.ResolveTelegramUser(context.Background(), identity.TelegramIdentity{
		Issuer: "https://oauth.telegram.org", Subject: "subject-two", TelegramUserID: 777,
	})
	if !errors.Is(err, identity.ErrLoginTelegramIdentityConflict) {
		t.Fatalf("duplicate Telegram uid err=%v", err)
	}
}

func TestTelegramLoginRepositoryRejectsDisabledBinding(t *testing.T) {
	db := googleLoginDB(t)
	user := User{ExternalSubject: "disabled-telegram", Username: "disabled-telegram", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user).Update("active", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UserLoginIdentity{
		UserID: user.ID, Provider: identity.FlowTelegram,
		Issuer: "https://oauth.telegram.org", Subject: "disabled-subject", EmailSnapshot: "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := NewLoginRepository(db).ResolveTelegramUser(context.Background(), identity.TelegramIdentity{
		Issuer: "https://oauth.telegram.org", Subject: "disabled-subject", TelegramUserID: 1,
	})
	if !errors.Is(err, identity.ErrLoginUserDisabled) {
		t.Fatalf("err=%v", err)
	}
}

func TestConsumeLoginTransactionReturnsThenClearsPKCEVerifier(t *testing.T) {
	db := googleLoginDB(t)
	repository := NewLoginRepository(db)
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	transaction := identity.LoginTransaction{
		StateHash: "state-hash", NonceHash: "nonce-hash", PKCEVerifier: "raw-server-only-verifier",
		Flow: identity.FlowTelegram, ReturnTo: "/", ExpiresAt: now.Add(time.Minute), CreatedAt: now,
	}
	if err := repository.CreateLoginTransaction(context.Background(), &transaction); err != nil {
		t.Fatal(err)
	}
	consumed, err := repository.ConsumeLoginTransaction(context.Background(), identity.FlowTelegram, "state-hash", now)
	if err != nil || consumed.PKCEVerifier != "raw-server-only-verifier" {
		t.Fatalf("consumed=%+v err=%v", consumed, err)
	}
	var stored LoginTransaction
	if err := db.First(&stored, transaction.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PKCEVerifier != "" || stored.ConsumedAt == nil {
		t.Fatalf("stored verifier was not cleared: %+v", stored)
	}
	if _, err := repository.ConsumeLoginTransaction(context.Background(), identity.FlowTelegram, "state-hash", now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("replay err=%v", err)
	}
}
