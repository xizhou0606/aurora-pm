package toolruntimegorm

import "time"

// ExternalToolingWorkspaceConfiguration is the server-authoritative Tool/UAT
// baseline for one PM workspace. Grants may only narrow this record.
type ExternalToolingWorkspaceConfiguration struct {
	WorkspaceID      uint   `gorm:"primaryKey"`
	Mode             string `gorm:"size:16;not null;default:window"`
	AllowedTools     string `gorm:"type:text;not null"`
	AllowedScopes    string `gorm:"type:text;not null"`
	EffectiveFrom    *time.Time
	ExpiresAt        *time.Time `gorm:"index:idx_tool_configuration_expiry"`
	CLIGrantTTLHours uint16     `gorm:"not null;default:168"`
	UpdatedBy        uint       `gorm:"not null;index:idx_tool_configuration_updater"`
	Version          uint64     `gorm:"not null;default:1"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (ExternalToolingWorkspaceConfiguration) TableName() string {
	return "pm_external_tooling_workspace_configurations"
}

func WorkspaceConfigurationMigrationRecords() []any {
	return []any{&ExternalToolingWorkspaceConfiguration{}}
}
