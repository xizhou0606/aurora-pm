package toolruntimegorm

import "time"

type ExternalDeviceAuthorization struct {
	RecordID               *string   `gorm:"size:128;uniqueIndex"`
	UserCode               *string   `gorm:"size:16"`
	DeviceCodeDigest       string    `gorm:"primaryKey;size:64"`
	UserCodeDigest         string    `gorm:"size:64;not null;uniqueIndex:ux_tool_device_user_code"`
	WorkspaceID            uint      `gorm:"not null;index:idx_tool_device_workspace_status"`
	ClientID               string    `gorm:"size:255;not null"`
	Channel                string    `gorm:"size:16;not null"`
	Audience               string    `gorm:"size:32;not null"`
	Resource               string    `gorm:"size:2048;not null"`
	Scope                  string    `gorm:"size:255;not null"`
	Status                 string    `gorm:"size:16;not null;index:idx_tool_device_workspace_status"`
	AuthorizationRequestID *string   `gorm:"size:128;index"`
	PrincipalID            *uint     `gorm:"index"`
	GrantID                *uint     `gorm:"index"`
	ExpiresAt              time.Time `gorm:"not null;index:idx_tool_device_expiry"`
	PollIntervalSeconds    uint16    `gorm:"not null"`
	LastPolledAt           *time.Time
	DecidedAt              *time.Time
	ConsumedAt             *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (ExternalDeviceAuthorization) TableName() string {
	return "pm_external_device_authorizations"
}

func DeviceAuthorizationMigrationRecords() []any {
	return []any{&ExternalDeviceAuthorization{}}
}
