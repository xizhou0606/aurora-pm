package identity

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeTelegramProvider struct {
	identity  TelegramIdentity
	err       error
	state     string
	nonce     string
	challenge string
	verifier  string
}

func (provider *fakeTelegramProvider) AuthorizationURL(state, nonce, challenge string) (string, error) {
	provider.state, provider.nonce, provider.challenge = state, nonce, challenge
	values := url.Values{"state": {state}, "nonce": {nonce}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	return "https://oauth.telegram.org/auth?" + values.Encode(), provider.err
}

func (provider *fakeTelegramProvider) Exchange(_ context.Context, _ string, verifier string) (TelegramIdentity, error) {
	provider.verifier = verifier
	value := provider.identity
	if value.Nonce == "$START_NONCE" {
		value.Nonce = provider.nonce
	}
	return value, provider.err
}

func TestTelegramLoginServiceUsesPKCES256AndOnlyPreboundUser(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	repo := newFakeLoginRepository()
	user := User{ID: 9, Username: "telegram-user", Active: true}
	repo.users[user.ID] = user
	repo.google[FlowTelegram+"\x00"+telegramIssuer+"\x00tg-subject"] = user.ID
	provider := &fakeTelegramProvider{identity: TelegramIdentity{
		Issuer: telegramIssuer, Subject: "tg-subject", TelegramUserID: 987654321,
		Name: "Telegram User", PreferredUsername: "tg_user", PictureURL: "https://t.me/i/userpic/320/photo.jpg",
		Nonce: "$START_NONCE",
	}}
	login := NewLoginService(repo, LoginOptions{Now: func() time.Time { return now }, Random: strings.NewReader(strings.Repeat("0123456789abcdef", 16))})
	service := NewTelegramLoginService(login, provider)
	authorizationURL, err := service.Start(context.Background(), "/login?redirect=%2Fworkspace%2Fprojects")
	if err != nil || !strings.Contains(authorizationURL, "oauth.telegram.org/auth") {
		t.Fatalf("start url=%q err=%v", authorizationURL, err)
	}
	transaction := repo.transactions[digest(provider.state)]
	parsedAuthorizationURL, parseErr := url.Parse(authorizationURL)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if transaction.NonceHash != digest(provider.nonce) || transaction.PKCEVerifier == "" ||
		provider.challenge != pkceS256Challenge(transaction.PKCEVerifier) || parsedAuthorizationURL.Query().Get("code_verifier") != "" {
		t.Fatalf("transaction=%+v challenge=%q url=%q", transaction, provider.challenge, authorizationURL)
	}
	result, err := service.Complete(context.Background(), provider.state, "authorization-code")
	if err != nil || result.LoginCode == "" || provider.verifier != transaction.PKCEVerifier {
		t.Fatalf("complete=%+v verifier=%q err=%v", result, provider.verifier, err)
	}
	resolved, flow, err := login.ExchangeCode(context.Background(), result.LoginCode)
	if err != nil || resolved.ID != user.ID || flow != FlowTelegram {
		t.Fatalf("resolved=%+v flow=%q err=%v", resolved, flow, err)
	}
}

func TestTelegramLoginServiceRejectsUnknownBindingAndInvalidClaims(t *testing.T) {
	for name, providerIdentity := range map[string]struct {
		identity TelegramIdentity
		err      error
	}{
		"unknown binding": {identity: TelegramIdentity{Issuer: telegramIssuer, Subject: "unknown", TelegramUserID: 1, Nonce: "$START_NONCE"}, err: ErrLoginUserNotFound},
		"wrong issuer":    {identity: TelegramIdentity{Issuer: "https://evil.example", Subject: "subject", TelegramUserID: 1, Nonce: "$START_NONCE"}, err: ErrLoginTelegramIdentityInvalid},
		"wrong nonce":     {identity: TelegramIdentity{Issuer: telegramIssuer, Subject: "subject", TelegramUserID: 1, Nonce: "wrong"}, err: ErrLoginTelegramIdentityInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeLoginRepository()
			provider := &fakeTelegramProvider{identity: providerIdentity.identity}
			service := NewTelegramLoginService(
				NewLoginService(repo, LoginOptions{Random: strings.NewReader(strings.Repeat("fedcba9876543210", 16))}), provider,
			)
			if _, err := service.Start(context.Background(), "/"); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Complete(context.Background(), provider.state, "code"); !errors.Is(err, providerIdentity.err) {
				t.Fatalf("err=%v want=%v", err, providerIdentity.err)
			}
		})
	}
}
