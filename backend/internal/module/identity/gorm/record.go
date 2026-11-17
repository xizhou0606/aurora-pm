// Package identitygorm contains identity-module GORM records.
package identitygorm

import "time"

// User is the deprecated PM-local user record. It remains persisted during the
// staged One2All identity migration for rollback compatibility, but shared-auth
// business paths must not use it as a user directory.
type User struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	ExternalSubject string  `gorm:"size:191;uniqueIndex" json:"-"`
	ExternalIssuer  string  `gorm:"size:191;index" json:"-"`
	Username        string  `gorm:"size:128;uniqueIndex;not null" json:"username"`
	DisplayName     string  `gorm:"size:128;uniqueIndex:ux_users_feishu_tenant_display_name,priority:2" json:"display_name"`
	Email           string  `gorm:"size:255" json:"email,omitempty"`
	AvatarURL       string  `gorm:"size:512" json:"avatar_url,omitempty"`
	FeishuOpenID    *string `gorm:"size:191;uniqueIndex:ux_users_feishu_open_id" json:"-"`
	FeishuTenantID  *uint   `gorm:"index;uniqueIndex:ux_users_feishu_tenant_display_name,priority:1" json:"-"`
	// FeishuDepartmentID is the mapped open_department_id from the Feishu
	// contact org sync (empty when unmapped). Display facts only — it never
	// carries authorization meaning.
	FeishuDepartmentID    string     `gorm:"size:64;not null;default:'';index" json:"-"`
	PasswordHash          string     `gorm:"size:255" json:"-"`
	Role                  string     `gorm:"size:40;not null;default:user" json:"role"`
	Active                bool       `gorm:"not null;default:true" json:"active"`
	Status                string     `gorm:"size:32;not null;default:active;index" json:"status"`
	StatusReason          string     `gorm:"size:1000" json:"status_reason,omitempty"`
	ApprovedAt            *time.Time `gorm:"index" json:"approved_at,omitempty"`
	ApprovedByUserID      *uint      `gorm:"index" json:"approved_by_user_id,omitempty"`
	BannedAt              *time.Time `gorm:"index" json:"banned_at,omitempty"`
	BannedByUserID        *uint      `gorm:"index" json:"banned_by_user_id,omitempty"`
	SessionVersion        uint64     `gorm:"not null;default:1" json:"session_version"`
	PasswordResetRequired bool       `gorm:"not null;default:false;index" json:"password_reset_required"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// PasswordResetTransaction stores only a digest of the single-use browser
// token. Creating one also marks the target user as requiring a password reset.
type PasswordResetTransaction struct {
	ID                uint       `gorm:"primaryKey"`
	TokenHash         string     `gorm:"size:64;not null;uniqueIndex"`
	UserID            uint       `gorm:"not null;index"`
	RequestedByUserID uint       `gorm:"not null;index"`
	ExpiresAt         time.Time  `gorm:"not null;index"`
	ConsumedAt        *time.Time `gorm:"index"`
	CreatedAt         time.Time
}

// FeishuDepartment is the synced snapshot of one Feishu contact department.
// It is a display projection for directory grouping; PM authorization never
// reads it.
type FeishuDepartment struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	OpenDepartmentID   string    `gorm:"size:64;not null;uniqueIndex:ux_feishu_departments_open_id" json:"open_department_id"`
	ParentDepartmentID string    `gorm:"size:64;not null;default:'';index" json:"parent_department_id"`
	Name               string    `gorm:"size:255;not null" json:"name"`
	DeptOrder          int64     `gorm:"not null;default:0" json:"dept_order"`
	MemberCount        int       `gorm:"not null;default:0" json:"member_count"`
	SyncedAt           time.Time `gorm:"not null" json:"synced_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// FeishuTenant is the PM-owned registry of organizations allowed to use the
// standalone Feishu login. Users reference this company record, while PM
// business data remains independent of it.
type FeishuTenant struct {
	ID        uint   `gorm:"primaryKey"`
	TenantKey string `gorm:"size:191;not null;uniqueIndex"`
	Enabled   bool   `gorm:"not null;default:true;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LoginTransaction retains digests of browser state and OIDC nonce. Telegram's
// raw PKCE verifier is server-side only and is cleared as the state is consumed.
type LoginTransaction struct {
	ID           uint       `gorm:"primaryKey"`
	StateHash    string     `gorm:"size:64;uniqueIndex;not null"`
	NonceHash    string     `gorm:"size:64"`
	PKCEVerifier string     `gorm:"size:128"`
	Flow         string     `gorm:"size:24;index;not null"`
	ReturnTo     string     `gorm:"size:2048;not null"`
	ExpiresAt    time.Time  `gorm:"index;not null"`
	ConsumedAt   *time.Time `gorm:"index"`
	CreatedAt    time.Time
}

// UserLoginIdentity stores provider bindings independently from the legacy
// users.external_* columns so adding providers never overwrites an existing
// SSO identity. A user can bind at most one identity per provider; Telegram's
// verified numeric UID is retained for later Bot API recipient resolution.
type UserLoginIdentity struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"not null;index;uniqueIndex:ux_user_login_provider,priority:1"`
	Provider       string `gorm:"size:32;not null;index;uniqueIndex:ux_user_login_provider,priority:2"`
	Issuer         string `gorm:"size:191;not null;uniqueIndex:ux_login_identity_subject,priority:1"`
	Subject        string `gorm:"size:191;not null;uniqueIndex:ux_login_identity_subject,priority:2"`
	EmailSnapshot  string `gorm:"size:255;not null"`
	TelegramUserID *int64 `gorm:"uniqueIndex:ux_user_login_telegram_user_id"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// LoginCode is a PM-owned, short-lived one-time code that lets a browser
// finish an OAuth/SSO callback without placing a bearer credential in a URL.
type LoginCode struct {
	ID         uint       `gorm:"primaryKey"`
	CodeHash   string     `gorm:"size:64;uniqueIndex;not null"`
	UserID     uint       `gorm:"index;not null"`
	Flow       string     `gorm:"size:24;index;not null"`
	ExpiresAt  time.Time  `gorm:"index;not null"`
	ConsumedAt *time.Time `gorm:"index"`
	CreatedAt  time.Time
}

type ExternalRegistrationTransaction struct {
	ID             uint   `gorm:"primaryKey"`
	TicketHash     string `gorm:"size:64;not null;uniqueIndex"`
	Provider       string `gorm:"size:32;not null;index"`
	Issuer         string `gorm:"size:191;not null"`
	Subject        string `gorm:"size:191;not null"`
	Email          string `gorm:"size:255"`
	TelegramUserID *int64
	DisplayName    string     `gorm:"size:128"`
	AvatarURL      string     `gorm:"size:512"`
	ExpiresAt      time.Time  `gorm:"not null;index"`
	ConsumedAt     *time.Time `gorm:"index"`
	CreatedAt      time.Time
}

type IdentityOperationTransaction struct {
	ID           uint       `gorm:"primaryKey"`
	StateHash    string     `gorm:"size:64;not null;uniqueIndex"`
	NonceHash    string     `gorm:"size:64;not null"`
	PKCEVerifier string     `gorm:"size:128"`
	ActorUserID  uint       `gorm:"not null;index"`
	Provider     string     `gorm:"size:32;not null;index"`
	ReturnTo     string     `gorm:"size:2048;not null"`
	ExpiresAt    time.Time  `gorm:"not null;index"`
	ConsumedAt   *time.Time `gorm:"index"`
	CreatedAt    time.Time
}

type Workspace struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Slug        string    `gorm:"size:80;uniqueIndex;not null" json:"slug"`
	Description string    `gorm:"size:500" json:"description,omitempty"`
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	IsDefault   bool      `gorm:"not null;default:false;uniqueIndex:ux_workspaces_single_default,where:is_default = true;check:ck_workspaces_default_enabled,NOT is_default OR enabled" json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkspaceMember struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_workspace_user;index" json:"workspace_id"`
	UserID      uint      `gorm:"not null;uniqueIndex:ux_workspace_user;index" json:"user_id"`
	Role        string    `gorm:"size:30;not null;default:member" json:"role"`
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Team struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_workspace_team_name;index" json:"workspace_id"`
	Name        string    `gorm:"size:128;not null;uniqueIndex:ux_workspace_team_name" json:"name"`
	Description string    `gorm:"size:500" json:"description,omitempty"`
	OwnerUserID *uint     `gorm:"index" json:"owner_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TeamMember struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_team_user;index" json:"workspace_id"`
	TeamID      uint      `gorm:"not null;uniqueIndex:ux_team_user;index" json:"team_id"`
	UserID      uint      `gorm:"not null;uniqueIndex:ux_team_user;index" json:"user_id"`
	Role        string    `gorm:"size:30;not null;default:member" json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
