package identity

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type passwordResetRepositoryStub struct {
	created      PasswordResetRequest
	tokenHash    string
	passwordHash string
}

func (repository *passwordResetRepositoryStub) CreatePasswordReset(_ context.Context, input PasswordResetRequest) error {
	repository.created = input
	return nil
}

func (repository *passwordResetRepositoryStub) CompletePasswordReset(_ context.Context, tokenHash, passwordHash string, _ time.Time) error {
	repository.tokenHash = tokenHash
	repository.passwordHash = passwordHash
	return nil
}

func TestPasswordResetCreatesSingleUseLinkAndCompletesWithoutCurrentPassword(t *testing.T) {
	now := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	repository := &passwordResetRepositoryStub{}
	service := NewPasswordResetService(repository, PasswordResetOptions{
		PublicBaseURL: "https://pm.example.test/", Now: func() time.Time { return now },
		Random: bytes.NewReader(bytes.Repeat([]byte{7}, 32)),
	})
	result, err := service.Create(context.Background(), 11, 22, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.URL, "https://pm.example.test/reset-password#token=") ||
		!result.NotificationQueued || !result.ExpiresAt.Equal(now.Add(defaultPasswordResetTTL)) {
		t.Fatalf("result=%+v", result)
	}
	if repository.created.ActorUserID != 11 || repository.created.TargetUserID != 22 || repository.created.TokenHash == "" {
		t.Fatalf("created=%+v", repository.created)
	}
	token := strings.TrimPrefix(result.URL, "https://pm.example.test/reset-password#token=")
	if err := service.Complete(context.Background(), token, "long-enough-password", "long-enough-password"); err != nil {
		t.Fatal(err)
	}
	if repository.tokenHash != repository.created.TokenHash {
		t.Fatalf("completion hash=%q want %q", repository.tokenHash, repository.created.TokenHash)
	}
	if bcrypt.CompareHashAndPassword([]byte(repository.passwordHash), []byte("long-enough-password")) != nil {
		t.Fatal("completion did not persist the new password hash")
	}
}

func TestPasswordResetValidatesNotificationScopeAndConfirmation(t *testing.T) {
	service := NewPasswordResetService(&passwordResetRepositoryStub{}, PasswordResetOptions{
		PublicBaseURL: "https://pm.example.test", Random: bytes.NewReader(bytes.Repeat([]byte{1}, 32)),
	})
	if _, err := service.Create(context.Background(), 1, 2, 0, true); err != ErrPasswordResetScope {
		t.Fatalf("scope err=%v", err)
	}
	if err := service.Complete(context.Background(), "token", "long-enough-password", "different-password"); err != ErrIdentityPasswordMismatch {
		t.Fatalf("confirmation err=%v", err)
	}
}
