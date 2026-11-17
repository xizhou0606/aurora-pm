package identity

import (
	"context"
	"errors"
	"strings"
)

// GoogleLoginService owns the complete Google login use case. HTTP handlers
// never call the provider or repository directly.
type GoogleLoginService struct {
	login    *LoginService
	provider GoogleLoginProvider
	account  *AccountService
}

func (s *GoogleLoginService) WithAccountService(account *AccountService) *GoogleLoginService {
	s.account = account
	return s
}

func NewGoogleLoginService(login *LoginService, provider GoogleLoginProvider) *GoogleLoginService {
	return &GoogleLoginService{login: login, provider: provider}
}

func (s *GoogleLoginService) Start(ctx context.Context, returnTo string) (string, error) {
	if s == nil || s.login == nil || s.provider == nil {
		return "", ErrLoginProviderUnavailable
	}
	state, nonce, err := s.login.startGoogle(ctx, returnTo)
	if err != nil {
		return "", err
	}
	return s.provider.AuthorizationURL(state, nonce)
}

func (s *GoogleLoginService) Complete(ctx context.Context, state, code string) (GoogleLoginResult, error) {
	if s == nil || s.login == nil || s.provider == nil {
		return GoogleLoginResult{}, ErrLoginProviderUnavailable
	}
	transaction, err := s.login.consumeGoogleState(ctx, state)
	if err != nil {
		return GoogleLoginResult{}, err
	}
	if strings.TrimSpace(code) == "" {
		return GoogleLoginResult{}, ErrInvalidLoginState
	}
	providerIdentity, err := s.provider.Exchange(ctx, code)
	if err != nil {
		if errors.Is(err, ErrLoginProviderUnavailable) {
			return GoogleLoginResult{}, ErrLoginProviderUnavailable
		}
		return GoogleLoginResult{}, ErrInvalidLoginState
	}
	if err := validateGoogleIdentity(providerIdentity, transaction.NonceHash); err != nil {
		return GoogleLoginResult{}, err
	}
	user, err := s.login.repo.ResolveOrBindGoogleUser(ctx, providerIdentity)
	if err != nil {
		if errors.Is(err, ErrLoginUserNotFound) && s.account != nil {
			code, registrationErr := s.account.CreateRegistration(ctx, ExternalRegistration{
				Provider: FlowGoogle, Issuer: providerIdentity.Issuer, Subject: providerIdentity.Subject,
				Email: providerIdentity.Email,
			})
			if registrationErr != nil {
				return GoogleLoginResult{}, registrationErr
			}
			return GoogleLoginResult{ReturnTo: transaction.ReturnTo, ExternalRegistrationCode: code}, nil
		}
		return GoogleLoginResult{}, err
	}
	if !user.Active {
		return GoogleLoginResult{}, ErrLoginUserDisabled
	}
	loginCode, err := s.login.IssueCode(ctx, FlowGoogle, user.ID)
	if err != nil {
		return GoogleLoginResult{}, err
	}
	return GoogleLoginResult{ReturnTo: transaction.ReturnTo, LoginCode: loginCode}, nil
}

func validateGoogleIdentity(value GoogleIdentity, nonceHash string) error {
	value.Issuer = strings.TrimSpace(value.Issuer)
	value.Subject = strings.TrimSpace(value.Subject)
	value.Email = strings.TrimSpace(value.Email)
	value.Nonce = strings.TrimSpace(value.Nonce)
	if value.Issuer == "" || value.Subject == "" || value.Email == "" || value.Nonce == "" ||
		!strings.EqualFold(value.Issuer, "https://accounts.google.com") || digest(value.Nonce) != nonceHash {
		return ErrLoginGoogleIdentityInvalid
	}
	if !value.EmailVerified {
		return ErrLoginGoogleEmailUnverified
	}
	return nil
}
