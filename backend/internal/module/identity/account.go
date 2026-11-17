package identity

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const externalRegistrationTTL = 10 * time.Minute

var (
	ErrExternalRegistrationDisabled = errors.New("external registration is disabled")
	ErrExternalRegistrationInvalid  = errors.New("external registration is invalid or expired")
	ErrIdentityUsernameInvalid      = errors.New("username is invalid")
	ErrIdentityUsernameConflict     = errors.New("username already exists")
	ErrIdentityPasswordInvalid      = errors.New("password is invalid")
	ErrIdentityPasswordMismatch     = errors.New("password confirmation does not match")
	ErrIdentityCurrentPassword      = errors.New("current password is invalid")
	ErrIdentityDisplayNameReadOnly  = errors.New("display name is managed by Feishu")
	ErrIdentityLoginMethodRequired  = errors.New("another login method is required")
	ErrIdentityProviderInvalid      = errors.New("identity provider is invalid")
)

var canonicalUsernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,79}$`)

type ExternalRegistration struct {
	ID             uint
	TicketHash     string
	Provider       string
	Issuer         string
	Subject        string
	Email          string
	TelegramUserID *int64
	DisplayName    string
	AvatarURL      string
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	CreatedAt      time.Time
}

type ExternalRegistrationComplete struct {
	TicketHash, Username, DisplayName, PasswordHash, LoginCodeHash string
	Now, LoginCodeExpiresAt                                        time.Time
}

type LoginIdentity struct {
	Provider       string `json:"provider"`
	Email          string `json:"email,omitempty"`
	TelegramUserID *int64 `json:"telegram_user_id,omitempty"`
}

type Account struct {
	EmailBinding        EmailBinding    `json:"email_binding"`
	User                User            `json:"user"`
	DisplayNameEditable bool            `json:"display_name_editable"`
	PasswordConfigured  bool            `json:"password_configured"`
	Identities          []LoginIdentity `json:"identities"`
}

type AccountRepository interface {
	CreateExternalRegistration(context.Context, *ExternalRegistration) error
	CompleteExternalRegistration(context.Context, ExternalRegistrationComplete) (User, error)
	Account(context.Context, uint) (Account, error)
	UpdateProfile(context.Context, uint, string) (User, error)
	UpdatePassword(context.Context, uint, string, string) error
	UnbindIdentity(context.Context, uint, string) error
	CreateIdentityOperation(context.Context, *IdentityOperation) error
	ConsumeIdentityOperation(context.Context, string, string, time.Time) (IdentityOperation, error)
	BindIdentity(context.Context, uint, string, GoogleIdentity, TelegramIdentity) error
}

type IdentityOperation struct {
	ID                                 uint
	StateHash, NonceHash, PKCEVerifier string
	ActorUserID                        uint
	Provider, ReturnTo                 string
	ExpiresAt                          time.Time
	ConsumedAt                         *time.Time
	CreatedAt                          time.Time
}

type AccountService struct {
	repo                AccountRepository
	login               *LoginService
	registrationEnabled bool
	google              GoogleLoginProvider
	telegram            TelegramLoginProvider
}

func (s *AccountService) WithProviders(google GoogleLoginProvider, telegram TelegramLoginProvider) *AccountService {
	s.google, s.telegram = google, telegram
	return s
}

func (s *AccountService) StartBind(ctx context.Context, actorID uint, provider, returnTo string) (string, error) {
	if s == nil || s.login == nil || s.repo == nil {
		return "", ErrLoginProviderUnavailable
	}
	if actorID == 0 || (provider != FlowGoogle && provider != FlowTelegram) {
		return "", ErrIdentityProviderInvalid
	}
	if (provider == FlowGoogle && s.google == nil) || (provider == FlowTelegram && s.telegram == nil) {
		return "", ErrLoginProviderUnavailable
	}
	state, stateHash, err := s.login.secret()
	if err != nil {
		return "", err
	}
	nonce, nonceHash, err := s.login.secret()
	if err != nil {
		return "", err
	}
	op := IdentityOperation{StateHash: stateHash, NonceHash: nonceHash, ActorUserID: actorID, Provider: provider, ReturnTo: NormalizeReturnTo(returnTo), CreatedAt: s.login.now().UTC()}
	op.ExpiresAt = op.CreatedAt.Add(defaultStateTTL)
	if provider == FlowTelegram {
		verifier, _, secretErr := s.login.secret()
		if secretErr != nil {
			return "", secretErr
		}
		op.PKCEVerifier = verifier
	}
	if err := s.repo.CreateIdentityOperation(ctx, &op); err != nil {
		return "", err
	}
	if provider == FlowGoogle && s.google != nil {
		return s.google.AuthorizationURL(state, nonce)
	}
	if provider == FlowTelegram && s.telegram != nil {
		return s.telegram.AuthorizationURL(state, nonce, pkceS256Challenge(op.PKCEVerifier))
	}
	return "", ErrLoginProviderUnavailable
}

func (s *AccountService) CompleteGoogleBind(ctx context.Context, state, code string) (string, error) {
	op, err := s.repo.ConsumeIdentityOperation(ctx, FlowGoogle, digest(state), s.login.now().UTC())
	if err != nil {
		return "", ErrInvalidLoginState
	}
	value, err := s.google.Exchange(ctx, code)
	if err != nil {
		return op.ReturnTo, err
	}
	if err := validateGoogleIdentity(value, op.NonceHash); err != nil {
		return op.ReturnTo, err
	}
	if err := s.repo.BindIdentity(ctx, op.ActorUserID, FlowGoogle, value, TelegramIdentity{}); err != nil {
		return op.ReturnTo, err
	}
	return op.ReturnTo, nil
}

func (s *AccountService) CompleteTelegramBind(ctx context.Context, state, code string) (string, error) {
	op, err := s.repo.ConsumeIdentityOperation(ctx, FlowTelegram, digest(state), s.login.now().UTC())
	if err != nil {
		return "", ErrInvalidLoginState
	}
	value, err := s.telegram.Exchange(ctx, code, op.PKCEVerifier)
	if err != nil {
		return op.ReturnTo, err
	}
	if err := validateTelegramIdentity(value, op.NonceHash); err != nil {
		return op.ReturnTo, err
	}
	if err := s.repo.BindIdentity(ctx, op.ActorUserID, FlowTelegram, GoogleIdentity{}, value); err != nil {
		return op.ReturnTo, err
	}
	return op.ReturnTo, nil
}

func NewAccountService(repo AccountRepository, login *LoginService, registrationEnabled bool) *AccountService {
	return &AccountService{repo: repo, login: login, registrationEnabled: registrationEnabled}
}

func (s *AccountService) CreateRegistration(ctx context.Context, identity ExternalRegistration) (string, error) {
	if s == nil || !s.registrationEnabled {
		return "", ErrExternalRegistrationDisabled
	}
	raw, hash, err := s.login.secret()
	if err != nil {
		return "", err
	}
	now := s.login.now().UTC()
	identity.TicketHash, identity.ExpiresAt, identity.CreatedAt = hash, now.Add(externalRegistrationTTL), now
	if err := s.repo.CreateExternalRegistration(ctx, &identity); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *AccountService) CompleteRegistration(ctx context.Context, ticket, username, displayName, password, confirmation string) (string, error) {
	if s == nil || !s.registrationEnabled {
		return "", ErrExternalRegistrationDisabled
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !canonicalUsernamePattern.MatchString(username) {
		return "", ErrIdentityUsernameInvalid
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 128 {
		return "", ErrAccessNameInvalid
	}
	if password != confirmation {
		return "", ErrIdentityPasswordMismatch
	}
	if len([]byte(password)) < 12 || len([]byte(password)) > 72 {
		return "", ErrIdentityPasswordInvalid
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	loginCode, loginCodeHash, err := s.login.secret()
	if err != nil {
		return "", err
	}
	now := s.login.now().UTC()
	_, err = s.repo.CompleteExternalRegistration(ctx, ExternalRegistrationComplete{
		TicketHash: digest(ticket), Username: username, DisplayName: displayName,
		PasswordHash: string(passwordHash), LoginCodeHash: loginCodeHash,
		Now: now, LoginCodeExpiresAt: now.Add(s.login.codeTTL),
	})
	if err != nil {
		return "", err
	}
	return loginCode, nil
}

func (s *AccountService) Account(ctx context.Context, actorID uint) (Account, error) {
	account, err := s.repo.Account(ctx, actorID)
	if err != nil {
		return Account{}, err
	}
	account.EmailBinding = HistoricalEmailBinding(account.User.Email)
	return account, nil
}

func (s *AccountService) UpdateProfile(ctx context.Context, actorID uint, displayName string) (User, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 128 {
		return User{}, ErrAccessNameInvalid
	}
	return s.repo.UpdateProfile(ctx, actorID, displayName)
}

func (s *AccountService) UpdatePassword(ctx context.Context, actorID uint, current, next, confirmation string) error {
	if next != confirmation {
		return ErrIdentityPasswordMismatch
	}
	if len([]byte(next)) < 12 || len([]byte(next)) > 72 {
		return ErrIdentityPasswordInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, actorID, current, string(hash))
}

func (s *AccountService) Unbind(ctx context.Context, actorID uint, provider string) error {
	provider = strings.TrimSpace(provider)
	if provider != FlowGoogle && provider != FlowTelegram {
		return ErrIdentityProviderInvalid
	}
	return s.repo.UnbindIdentity(ctx, actorID, provider)
}
