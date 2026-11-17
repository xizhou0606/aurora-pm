package identity

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeGoogleProvider struct {
	identity GoogleIdentity
	err      error
	state    string
	nonce    string
}

func (f *fakeGoogleProvider) AuthorizationURL(state, nonce string) (string, error) {
	f.state, f.nonce = state, nonce
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + url.QueryEscape(state) + "&nonce=" + url.QueryEscape(nonce), f.err
}

func (f *fakeGoogleProvider) Exchange(_ context.Context, _ string) (GoogleIdentity, error) {
	value := f.identity
	if value.Nonce == "$START_NONCE" {
		value.Nonce = f.nonce
	}
	return value, f.err
}

func TestGoogleLoginServiceBindsVerifiedUniqueEmailAndIssuesOneTimeCode(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	repo := newFakeLoginRepository()
	repo.users[7] = User{ID: 7, Username: "alice", Email: "Alice@Example.com", Active: true}
	repo.google["https://accounts.google.com\x00google-user-7"] = 7
	login := NewLoginService(repo, LoginOptions{Now: func() time.Time { return now }, Random: strings.NewReader(strings.Repeat("ab", 64))})
	provider := &fakeGoogleProvider{identity: GoogleIdentity{
		Issuer: "https://accounts.google.com", Subject: "google-user-7",
		Email: "alice@example.com", EmailVerified: true, Nonce: "$START_NONCE",
	}}
	service := NewGoogleLoginService(login, provider)
	authorizationURL, err := service.Start(context.Background(), "/login?redirect=%2Fworkspace%2Fprojects")
	if err != nil || provider.state == "" || provider.nonce == "" || !strings.Contains(authorizationURL, "accounts.google.com") {
		t.Fatalf("start url=%q state=%q nonce=%q err=%v", authorizationURL, provider.state, provider.nonce, err)
	}
	transaction := repo.transactions[digest(provider.state)]
	if transaction.NonceHash == "" || transaction.NonceHash == provider.nonce || transaction.NonceHash != digest(provider.nonce) {
		t.Fatalf("nonce was not retained as a digest: %+v", transaction)
	}
	result, err := service.Complete(context.Background(), provider.state, "authorization-code")
	if err != nil || result.ReturnTo != "/login?redirect=%2Fworkspace%2Fprojects" || result.LoginCode == "" {
		t.Fatalf("complete=%+v err=%v", result, err)
	}
	user, flow, err := login.ExchangeCode(context.Background(), result.LoginCode)
	if err != nil || flow != FlowGoogle || user.ID != 7 {
		t.Fatalf("exchange user=%+v flow=%q err=%v", user, flow, err)
	}
	if _, _, err := login.ExchangeCode(context.Background(), result.LoginCode); !errors.Is(err, ErrInvalidLoginCode) {
		t.Fatalf("replayed code err=%v", err)
	}
}

func TestGoogleLoginServiceRejectsWrongNonceUnverifiedOrUnknownEmail(t *testing.T) {
	for name, identityValue := range map[string]struct {
		identity GoogleIdentity
		err      error
	}{
		"wrong nonce":   {identity: GoogleIdentity{Issuer: "https://accounts.google.com", Subject: "s", Email: "a@example.com", EmailVerified: true, Nonce: "wrong"}, err: ErrLoginGoogleIdentityInvalid},
		"unverified":    {identity: GoogleIdentity{Issuer: "https://accounts.google.com", Subject: "s", Email: "a@example.com", Nonce: "$START_NONCE"}, err: ErrLoginGoogleEmailUnverified},
		"unknown email": {identity: GoogleIdentity{Issuer: "https://accounts.google.com", Subject: "s", Email: "missing@example.com", EmailVerified: true, Nonce: "$START_NONCE"}, err: ErrLoginUserNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeLoginRepository()
			repo.users[1] = User{ID: 1, Email: "a@example.com", Active: true}
			provider := &fakeGoogleProvider{identity: identityValue.identity}
			service := NewGoogleLoginService(NewLoginService(repo, LoginOptions{Random: strings.NewReader(strings.Repeat("xy", 64))}), provider)
			if _, err := service.Start(context.Background(), "/"); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Complete(context.Background(), provider.state, "code"); !errors.Is(err, identityValue.err) {
				t.Fatalf("complete err=%v want=%v", err, identityValue.err)
			}
		})
	}
}
