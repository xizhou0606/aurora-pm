package toolruntimegorm

import "time"

// ExternalAuthorizationRequest persists a short-lived approval transaction.
// Its opaque ID contains no principal, workspace, or credential material.
type ExternalAuthorizationRequest struct {
	ID                   string    `gorm:"primaryKey;size:128"`
	PrincipalID          uint      `gorm:"not null;index:idx_tool_auth_request_owner"`
	WorkspaceID          uint      `gorm:"not null;index:idx_tool_auth_request_owner"`
	WorkspaceName        string    `gorm:"size:255;not null"`
	ClientID             string    `gorm:"size:255;not null"`
	ClientName           string    `gorm:"size:255;not null"`
	Channel              string    `gorm:"size:16;not null"`
	ConfigurationMode    string    `gorm:"size:16;not null;default:window"`
	ConfigurationVersion uint64    `gorm:"not null;default:0"`
	ToolNames            string    `gorm:"type:text;not null"`
	GrantScopes          string    `gorm:"type:text;not null"`
	EffectiveFrom        time.Time `gorm:"not null"`
	ExpiresAt            time.Time `gorm:"not null"`
	RequestExpiresAt     time.Time `gorm:"not null;index:idx_tool_auth_request_expiry"`
	OAuthState           *string   `gorm:"size:1024"`
	OAuthRedirectURI     *string   `gorm:"size:2048"`
	OAuthPKCEChallenge   *string   `gorm:"size:64"`
	OAuthResource        *string   `gorm:"size:2048"`
	OAuthScope           *string   `gorm:"size:255"`
	Decision             string    `gorm:"size:16;not null;index:idx_tool_auth_request_decision"`
	DecidedAt            *time.Time
	GrantID              *uint `gorm:"index:idx_tool_auth_request_grant"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (ExternalAuthorizationRequest) TableName() string {
	return "pm_external_authorization_requests"
}
