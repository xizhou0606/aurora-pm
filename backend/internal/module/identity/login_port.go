package identity

import (
	"context"
	"time"
)

// LoginRepository is the identity module's persistence boundary for
// short-lived OAuth transactions, registered Feishu tenants, and PM users.
type LoginRepository interface {
	CreateLoginTransaction(context.Context, *LoginTransaction) error
	ConsumeLoginTransaction(context.Context, string, string, time.Time) (LoginTransaction, error)
	CreateLoginCode(context.Context, *LoginCode) error
	ConsumeLoginCode(context.Context, string, time.Time) (LoginCode, error)
	RegisterFeishuTenants(context.Context, []string) error
	ProvisionFeishuUser(context.Context, FeishuIdentity) (User, error)
	FindActiveUserByExternalIdentity(context.Context, string, string) (User, error)
	ResolveOrBindGoogleUser(context.Context, GoogleIdentity) (User, error)
	ResolveTelegramUser(context.Context, TelegramIdentity) (User, error)
	FindActiveUserByID(context.Context, uint) (User, error)
}

// GoogleLoginProvider is implemented by the Google OIDC adapter. Signature,
// issuer, audience, expiry, and token parsing are protocol concerns there;
// nonce and PM account admission remain identity-domain decisions.
type GoogleLoginProvider interface {
	AuthorizationURL(state, nonce string) (string, error)
	Exchange(context.Context, string) (GoogleIdentity, error)
}

type TelegramLoginProvider interface {
	AuthorizationURL(state, nonce, codeChallenge string) (string, error)
	Exchange(context.Context, string, string) (TelegramIdentity, error)
}
