// Package auditgorm contains audit-module GORM records.
package auditgorm

import "time"

type AuditLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;index" json:"workspace_id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Username    string    `gorm:"size:80" json:"username,omitempty"`
	Role        string    `gorm:"size:40" json:"role,omitempty"`
	Method      string    `gorm:"size:10" json:"method,omitempty"`
	StatusCode  int       `json:"status_code,omitempty"`
	Resource    string    `gorm:"size:80;index" json:"resource"`
	ResourceID  uint      `gorm:"index" json:"resource_id"`
	ResourceRef string    `gorm:"size:128;not null;default:'';index" json:"resource_ref,omitempty"`
	ProjectID   uint      `gorm:"index" json:"project_id,omitempty"`
	Action      string    `gorm:"size:80;not null" json:"action"`
	Target      string    `gorm:"size:220" json:"target,omitempty"`
	Detail      string    `gorm:"type:text" json:"detail,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type OutboxEvent struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID   uint       `gorm:"not null;index" json:"workspace_id"`
	AggregateType string     `gorm:"size:80;not null;index" json:"aggregate_type"`
	AggregateID   uint       `gorm:"not null;index" json:"aggregate_id"`
	AggregateRef  string     `gorm:"size:128;not null;default:'';index" json:"aggregate_ref,omitempty"`
	EventType     string     `gorm:"size:120;not null;index" json:"event_type"`
	Payload       string     `gorm:"type:text;not null" json:"payload"`
	OccurredAt    time.Time  `gorm:"not null" json:"occurred_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	Attempts      int        `gorm:"not null;default:0" json:"attempts"`
	LastError     string     `gorm:"type:text" json:"last_error,omitempty"`
	ClaimedAt     *time.Time `gorm:"index" json:"claimed_at,omitempty"`
	ClaimedBy     string     `gorm:"size:160;index" json:"claimed_by,omitempty"`
	NextAttemptAt *time.Time `gorm:"index" json:"next_attempt_at,omitempty"`
}
