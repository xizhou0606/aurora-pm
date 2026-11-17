package toolruntime

import (
	"context"
	"errors"
	"io"
	"time"
)

const (
	DeviceAuthorizationGrantType  = "urn:ietf:params:oauth:grant-type:device_code"
	DefaultDeviceAuthorizationTTL = 10 * time.Minute
	DefaultDevicePollInterval     = 5 * time.Second
	deviceSlowDownIncrement       = 5 * time.Second
)

type DeviceAuthorizationStatus string

const (
	DeviceAuthorizationPending   DeviceAuthorizationStatus = "pending"
	deviceAuthorizationApproving DeviceAuthorizationStatus = "approving"
	DeviceAuthorizationApproved  DeviceAuthorizationStatus = "approved"
	DeviceAuthorizationDenied    DeviceAuthorizationStatus = "denied"
	DeviceAuthorizationConsumed  DeviceAuthorizationStatus = "consumed"
	DeviceAuthorizationExpired   DeviceAuthorizationStatus = "expired"
)

var (
	ErrDeviceAuthorizationInvalid  = errors.New("device authorization is invalid")
	ErrDeviceAuthorizationPending  = errors.New("device authorization is pending")
	ErrDeviceAuthorizationSlowDown = errors.New("device authorization polling is too frequent")
	ErrDeviceAuthorizationDenied   = errors.New("device authorization was denied")
	ErrDeviceAuthorizationExpired  = errors.New("device authorization expired")
)

type DeviceAuthorization struct {
	UserCode               string
	DeviceCodeDigest       string
	UserCodeDigest         string
	WorkspaceID            uint
	ClientID               string
	Channel                DeliveryChannel
	Audience               TokenAudience
	Resource               string
	Scope                  string
	Status                 DeviceAuthorizationStatus
	AuthorizationRequestID string
	PrincipalID            uint
	GrantID                uint
	ExpiresAt              time.Time
	PollInterval           time.Duration
	LastPolledAt           *time.Time
	DecidedAt              *time.Time
	ConsumedAt             *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type DeviceAuthorizationRepository interface {
	Create(context.Context, DeviceAuthorization) error
	FindByUserCode(context.Context, string) (DeviceAuthorization, error)
	ClaimDecision(context.Context, string, uint, AuthorizationDecision, time.Time) (DeviceAuthorization, error)
	CompleteApproval(context.Context, string, CredentialBinding, string, time.Time) (DeviceAuthorization, error)
	FailApproval(context.Context, string, time.Time) error
	Poll(context.Context, string, time.Time) (DeviceAuthorization, error)
	Consume(context.Context, string, time.Time, CredentialFamily, AccessCredential, RefreshCredential) error
}

type DeviceAuthorizationPolicy struct {
	TTL          time.Duration
	PollInterval time.Duration
}

func DefaultDeviceAuthorizationPolicy() DeviceAuthorizationPolicy {
	return DeviceAuthorizationPolicy{TTL: DefaultDeviceAuthorizationTTL, PollInterval: DefaultDevicePollInterval}
}

type StartDeviceAuthorizationInput struct {
	ClientID    string
	WorkspaceID uint
	Resource    string
	Scope       string
}

type StartedDeviceAuthorization struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               time.Duration
	Interval                time.Duration
}

type DeviceAuthorizationReview struct {
	UserCode         string
	Status           DeviceAuthorizationStatus
	ClientID         string
	ClientName       string
	WorkspaceID      uint
	WorkspaceName    string
	Mode             WorkspaceConfigurationMode
	Tools            []Definition
	EffectiveFrom    time.Time
	GrantExpiresAt   time.Time
	RequestExpiresAt time.Time
}

type DeviceAuthorizationService struct {
	authorizations *AuthorizationService
	credentials    *CredentialService
	repository     DeviceAuthorizationRepository
	configuration  OAuthServerConfiguration
	policy         DeviceAuthorizationPolicy
	now            func() time.Time
	random         io.Reader
}
