package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeAccountRepository struct {
	createOperationCalls int
	operation            IdentityOperation
	consumeErr           error
	bindErr              error
}

func (f *fakeAccountRepository) CreateExternalRegistration(context.Context, *ExternalRegistration) error {
	return nil
}
func (f *fakeAccountRepository) CompleteExternalRegistration(context.Context, ExternalRegistrationComplete) (User, error) {
	return User{}, nil
}
func (f *fakeAccountRepository) Account(context.Context, uint) (Account, error) {
	return Account{}, nil
}
func (f *fakeAccountRepository) UpdateProfile(context.Context, uint, string) (User, error) {
	return User{}, nil
}
func (f *fakeAccountRepository) UpdatePassword(context.Context, uint, string, string) error {
	return nil
}
func (f *fakeAccountRepository) UnbindIdentity(context.Context, uint, string) error { return nil }
func (f *fakeAccountRepository) CreateIdentityOperation(_ context.Context, operation *IdentityOperation) error {
	f.createOperationCalls++
	f.operation = *operation
	return nil
}
func (f *fakeAccountRepository) ConsumeIdentityOperation(context.Context, string, string, time.Time) (IdentityOperation, error) {
	return f.operation, f.consumeErr
}
func (f *fakeAccountRepository) BindIdentity(context.Context, uint, string, GoogleIdentity, TelegramIdentity) error {
	return f.bindErr
}

func TestAccountServiceStartBindDoesNotPersistWhenProviderUnavailable(t *testing.T) {
	repo := &fakeAccountRepository{}
	service := NewAccountService(repo, NewLoginService(newFakeLoginRepository(), LoginOptions{}), false)
	if _, err := service.StartBind(context.Background(), 7, FlowGoogle, "/workspace/profile"); !errors.Is(err, ErrLoginProviderUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if repo.createOperationCalls != 0 {
		t.Fatalf("persisted %d operation rows before provider validation", repo.createOperationCalls)
	}
}

func TestAccountServiceBindReturnsTrustedReturnToAfterConsumedStateFailure(t *testing.T) {
	repo := &fakeAccountRepository{operation: IdentityOperation{
		ActorUserID: 7, Provider: FlowGoogle, ReturnTo: "/workspace/profile", NonceHash: digest("expected-nonce"),
	}, bindErr: ErrLoginGoogleIdentityConflict}
	provider := &fakeGoogleProvider{identity: GoogleIdentity{
		Issuer: "https://accounts.google.com", Subject: "subject", Email: "user@example.com",
		EmailVerified: true, Nonce: "expected-nonce",
	}}
	service := NewAccountService(repo, NewLoginService(newFakeLoginRepository(), LoginOptions{}), false).WithProviders(provider, nil)
	returnTo, err := service.CompleteGoogleBind(context.Background(), "state", "code")
	if returnTo != "/workspace/profile" || !errors.Is(err, ErrLoginGoogleIdentityConflict) {
		t.Fatalf("returnTo=%q err=%v", returnTo, err)
	}
	provider.identity.Nonce = "wrong"
	returnTo, err = service.CompleteGoogleBind(context.Background(), "state", "code")
	if returnTo != "/workspace/profile" || !errors.Is(err, ErrLoginGoogleIdentityInvalid) {
		t.Fatalf("validation returnTo=%q err=%v", returnTo, err)
	}
}

func TestAccountServiceRequiresUserChosenDisplayNameForExternalRegistration(t *testing.T) {
	service := NewAccountService(
		&fakeAccountRepository{},
		NewLoginService(newFakeLoginRepository(), LoginOptions{}),
		true,
	)
	if _, err := service.CompleteRegistration(
		context.Background(), "registration-ticket", "new.user", " ",
		"private-value-12", "private-value-12",
	); !errors.Is(err, ErrAccessNameInvalid) {
		t.Fatalf("blank display name err=%v", err)
	}
}
