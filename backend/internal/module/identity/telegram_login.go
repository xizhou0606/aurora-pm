package identity

import (
	"context"
	"errors"
	"strings"
)

const telegramIssuer = "https://oauth.telegram.org"

type TelegramLoginService struct {
	login    *LoginService
	provider TelegramLoginProvider
	account  *AccountService
}

func (s *TelegramLoginService) WithAccountService(account *AccountService) *TelegramLoginService {
	s.account = account
	return s
}

func NewTelegramLoginService(login *LoginService, provider TelegramLoginProvider) *TelegramLoginService {
	return &TelegramLoginService{login: login, provider: provider}
}

func (s *TelegramLoginService) Start(ctx context.Context, returnTo string) (string, error) {
	if s == nil || s.login == nil || s.provider == nil {
		return "", ErrLoginProviderUnavailable
	}
	state, nonce, challenge, err := s.login.startTelegram(ctx, returnTo)
	if err != nil {
		return "", err
	}
	return s.provider.AuthorizationURL(state, nonce, challenge)
}

func (s *TelegramLoginService) Complete(ctx context.Context, state, code string) (TelegramLoginResult, error) {
	if s == nil || s.login == nil || s.provider == nil {
		return TelegramLoginResult{}, ErrLoginProviderUnavailable
	}
	transaction, err := s.login.consumeTelegramState(ctx, state)
	if err != nil || strings.TrimSpace(code) == "" {
		return TelegramLoginResult{}, ErrInvalidLoginState
	}
	providerIdentity, err := s.provider.Exchange(ctx, code, transaction.PKCEVerifier)
	if err != nil {
		if errors.Is(err, ErrLoginProviderUnavailable) {
			return TelegramLoginResult{}, ErrLoginProviderUnavailable
		}
		return TelegramLoginResult{}, ErrInvalidLoginState
	}
	if err := validateTelegramIdentity(providerIdentity, transaction.NonceHash); err != nil {
		return TelegramLoginResult{}, err
	}
	user, err := s.login.repo.ResolveTelegramUser(ctx, providerIdentity)
	if err != nil {
		if errors.Is(err, ErrLoginUserNotFound) && s.account != nil {
			uid := providerIdentity.TelegramUserID
			code, registrationErr := s.account.CreateRegistration(ctx, ExternalRegistration{
				Provider: FlowTelegram, Issuer: providerIdentity.Issuer, Subject: providerIdentity.Subject,
				TelegramUserID: &uid, DisplayName: providerIdentity.Name, AvatarURL: providerIdentity.PictureURL,
			})
			if registrationErr != nil {
				return TelegramLoginResult{}, registrationErr
			}
			return TelegramLoginResult{ReturnTo: transaction.ReturnTo, ExternalRegistrationCode: code}, nil
		}
		return TelegramLoginResult{}, err
	}
	if !user.Active {
		return TelegramLoginResult{}, ErrLoginUserDisabled
	}
	loginCode, err := s.login.IssueCode(ctx, FlowTelegram, user.ID)
	if err != nil {
		return TelegramLoginResult{}, err
	}
	return TelegramLoginResult{ReturnTo: transaction.ReturnTo, LoginCode: loginCode}, nil
}

func validateTelegramIdentity(value TelegramIdentity, nonceHash string) error {
	if strings.TrimSpace(value.Issuer) != telegramIssuer || strings.TrimSpace(value.Subject) == "" ||
		value.TelegramUserID <= 0 || strings.TrimSpace(value.Nonce) == "" || digest(value.Nonce) != nonceHash {
		return ErrLoginTelegramIdentityInvalid
	}
	return nil
}
