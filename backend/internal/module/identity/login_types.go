package identity

import "time"

// User is the identity projection needed by login and session issuance. The
// persistence adapter owns the database record; the domain never imports it.
type User struct {
	ID                    uint          `json:"id"`
	ExternalSubject       string        `json:"-"`
	ExternalIssuer        string        `json:"-"`
	Username              string        `json:"username"`
	DisplayName           string        `json:"display_name"`
	Email                 string        `json:"email,omitempty"`
	AvatarURL             string        `json:"avatar_url,omitempty"`
	PasswordHash          string        `json:"-"`
	Role                  string        `json:"role"`
	Active                bool          `json:"active"`
	Status                AccountStatus `json:"status"`
	SessionVersion        uint64        `json:"session_version"`
	PasswordResetRequired bool          `json:"password_reset_required"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
}

// FeishuIdentity is the provider-verified profile used to admit a PM user
// and bind the dedicated login application's open_id for IM delivery.
type FeishuIdentity struct {
	TenantKey   string
	OpenID      string
	DisplayName string
	AvatarURL   string
}

// GoogleIdentity contains only claims verified by the OIDC adapter. The
// domain still validates nonce and account-linking requirements before use.
type GoogleIdentity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	Nonce         string
}

type GoogleLoginResult struct {
	ReturnTo                 string
	LoginCode                string
	ExternalRegistrationCode string
}

type TelegramIdentity struct {
	Issuer            string
	Subject           string
	TelegramUserID    int64
	Name              string
	PreferredUsername string
	PictureURL        string
	Nonce             string
}

type TelegramLoginResult struct {
	ReturnTo                 string
	LoginCode                string
	ExternalRegistrationCode string
}

// LoginTransaction and LoginCode are short-lived, one-time identity values.
// They intentionally contain no storage-specific tags or dependencies.
type LoginTransaction struct {
	ID           uint
	StateHash    string
	NonceHash    string
	PKCEVerifier string
	Flow         string
	ReturnTo     string
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	CreatedAt    time.Time
}

type LoginCode struct {
	ID         uint
	CodeHash   string
	UserID     uint
	Flow       string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}
