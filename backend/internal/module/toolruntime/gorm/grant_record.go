package toolruntimegorm

import "time"

// ExternalAuthorizationGrant stores delegation facts only. Token material is
// intentionally absent; future credential records may persist digests only.
type ExternalAuthorizationGrant struct {
	ID            uint       `gorm:"primaryKey"`
	PrincipalID   uint       `gorm:"not null;index:idx_tool_grant_principal"`
	WorkspaceID   uint       `gorm:"not null;index:idx_tool_grant_workspace"`
	ClientID      string     `gorm:"size:255;not null;index:idx_tool_grant_client"`
	Channel       string     `gorm:"size:16;not null;index:idx_tool_grant_channel"`
	AllowedTools  string     `gorm:"type:text;not null"`
	AllowedScopes string     `gorm:"type:text;not null"`
	EffectiveFrom time.Time  `gorm:"not null;index:idx_tool_grant_window"`
	ExpiresAt     time.Time  `gorm:"not null;index:idx_tool_grant_window"`
	RevokedAt     *time.Time `gorm:"index:idx_tool_grant_revoked"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (ExternalAuthorizationGrant) TableName() string {
	return "pm_external_authorization_grants"
}
