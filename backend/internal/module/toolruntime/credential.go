package toolruntime

import (
	"context"
	"errors"
	"io"
	"time"
)

const PKCEMethodS256 = "S256"

var (
	ErrCredentialConfigInvalid  = errors.New("external credential configuration is invalid")
	ErrAuthorizationCodeInvalid = errors.New("external authorization code is invalid")
	ErrRefreshTokenInvalid      = errors.New("external refresh token is invalid")
	ErrRefreshTokenReplay       = errors.New("external refresh token replay detected")
)

type CredentialBinding struct {
	PrincipalID uint
	WorkspaceID uint
	GrantID     uint
	ClientID    string
	Channel     DeliveryChannel
	Audience    TokenAudience
}

type AuthorizationCodeCredential struct {
	CodeDigest             string
	AuthorizationRequestID string
	Binding                CredentialBinding
	PKCEChallenge          string
	RedirectURI            string
	ExpiresAt              time.Time
	ConsumedAt             *time.Time
	CreatedAt              time.Time
}

type CredentialFamily struct {
	ID                string
	Binding           CredentialBinding
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AccessCredential struct {
	TokenDigest string
	FamilyID    string
	Binding     CredentialBinding
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

type RefreshCredential struct {
	TokenDigest string
	FamilyID    string
	Binding     CredentialBinding
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

type CredentialRepository interface {
	CreateAuthorizationCode(context.Context, AuthorizationCodeCredential) error
	FindAuthorizationCode(context.Context, string) (AuthorizationCodeCredential, error)
	ExchangeAuthorizationCode(context.Context, string, time.Time, CredentialFamily, AccessCredential, RefreshCredential) error
	FindCredentialFamily(context.Context, string) (CredentialFamily, error)
	FindAccessCredential(context.Context, string) (AccessCredential, error)
	FindRefreshCredential(context.Context, string) (RefreshCredential, error)
	RotateRefreshToken(context.Context, string, time.Time, AccessCredential, RefreshCredential) error
	RevokeRefreshFamily(context.Context, string, time.Time) error
}

type WorkspaceMembershipVerifier interface {
	VerifyWorkspaceMembership(context.Context, uint, uint) error
}

type WorkspaceMembershipVerifierFunc func(context.Context, uint, uint) error

func (function WorkspaceMembershipVerifierFunc) VerifyWorkspaceMembership(ctx context.Context, principalID, workspaceID uint) error {
	return function(ctx, principalID, workspaceID)
}

type CredentialPolicy struct {
	AuthorizationCodeTTL time.Duration
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	RefreshAbsoluteTTL   time.Duration
}

func DefaultCredentialPolicy() CredentialPolicy {
	// Refresh credentials remain usable for the configured Grant/UAT lifetime.
	// Every refresh and access verification still rechecks the Grant, workspace
	// membership, and current workspace Tool configuration, so revocation,
	// disablement, or UAT expiry takes effect immediately.
	const grantLifetimeCeiling = 200 * 365 * 24 * time.Hour
	return CredentialPolicy{
		AuthorizationCodeTTL: 5 * time.Minute,
		AccessTokenTTL:       7 * 24 * time.Hour,
		RefreshTokenTTL:      grantLifetimeCeiling,
		RefreshAbsoluteTTL:   grantLifetimeCeiling,
	}
}

type IssueAuthorizationCodeInput struct {
	PrincipalID   uint
	WorkspaceID   uint
	RequestID     string
	PKCEMethod    string
	PKCEChallenge string
	RedirectURI   string
}

type IssuedAuthorizationCode struct {
	Code      string
	ExpiresAt time.Time
}

type ExchangeAuthorizationCodeInput struct {
	Code         string
	PKCEVerifier string
	RedirectURI  string
	ClientID     string
	Channel      DeliveryChannel
}

type RefreshAccessTokenInput struct {
	RefreshToken string
	ClientID     string
	Channel      DeliveryChannel
}

type RevokeRefreshTokenInput struct {
	RefreshToken string
	ClientID     string
	Channel      DeliveryChannel
}

type IssuedTokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type CredentialService struct {
	grants      GrantRepository
	requests    AuthorizationRequestRepository
	credentials CredentialRepository
	members     WorkspaceMembershipVerifier
	policy      CredentialPolicy
	now         func() time.Time
	random      io.Reader
}
