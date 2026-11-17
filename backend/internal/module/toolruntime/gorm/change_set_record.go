package toolruntimegorm

import "time"

type ToolChangeSet struct {
	ID                string     `gorm:"primaryKey;size:128"`
	Status            string     `gorm:"size:24;not null;index:idx_tool_change_set_status"`
	ToolName          string     `gorm:"size:160;not null;uniqueIndex:ux_tool_change_set_idempotency,priority:8"`
	ToolVersion       string     `gorm:"size:40;not null"`
	Risk              string     `gorm:"size:24;not null"`
	AuditEvent        string     `gorm:"size:180;not null"`
	PrincipalID       uint       `gorm:"not null;uniqueIndex:ux_tool_change_set_idempotency,priority:1;index:idx_tool_change_set_binding"`
	WorkspaceID       uint       `gorm:"not null;uniqueIndex:ux_tool_change_set_idempotency,priority:2;index:idx_tool_change_set_binding"`
	GrantID           uint       `gorm:"not null;uniqueIndex:ux_tool_change_set_idempotency,priority:3;index:idx_tool_change_set_binding"`
	ClientID          string     `gorm:"size:255;not null;uniqueIndex:ux_tool_change_set_idempotency,priority:4"`
	Channel           string     `gorm:"size:16;not null;uniqueIndex:ux_tool_change_set_idempotency,priority:5"`
	Audience          string     `gorm:"size:32;not null;uniqueIndex:ux_tool_change_set_idempotency,priority:6"`
	IdempotencyDigest string     `gorm:"size:64;not null;uniqueIndex:ux_tool_change_set_idempotency,priority:7"`
	InputDigest       string     `gorm:"size:64;not null"`
	InputSize         int        `gorm:"not null"`
	InputJSON         string     `gorm:"type:text;not null"`
	ResultJSON        string     `gorm:"type:text;not null"`
	ErrorCode         string     `gorm:"size:80;not null"`
	ApplyClaimID      string     `gorm:"size:128;not null"`
	ApplyClaimedAt    *time.Time `gorm:"index:idx_tool_change_set_claim"`
	ExpiresAt         time.Time  `gorm:"not null;index:idx_tool_change_set_expiry"`
	ConfirmedAt       *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ToolChangeSet) TableName() string { return "pm_tool_change_sets" }

type ToolChangeSetEvent struct {
	ID          uint      `gorm:"primaryKey"`
	ChangeSetID string    `gorm:"size:128;not null;index:idx_tool_change_set_event"`
	WorkspaceID uint      `gorm:"not null;index:idx_tool_change_set_event_workspace"`
	PrincipalID uint      `gorm:"not null;index:idx_tool_change_set_event_principal"`
	Status      string    `gorm:"size:24;not null;index"`
	AuditEvent  string    `gorm:"size:180;not null;index"`
	ErrorCode   string    `gorm:"size:80;not null"`
	OccurredAt  time.Time `gorm:"not null;index"`
}

func (ToolChangeSetEvent) TableName() string { return "pm_tool_change_set_events" }

func ChangeSetMigrationRecords() []any {
	return []any{&ToolChangeSet{}, &ToolChangeSetEvent{}}
}
