// Package qualitygorm contains quality-module GORM records.
package qualitygorm

import (
	"time"

	"gorm.io/gorm"
)

// QualityBug stores the PM-owned defect lifecycle independently from work tasks.
// Every row is workspace scoped so the standalone service never needs to query
// the One-2-All database to render quality, todo, or BI views.
type QualityBug struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID   uint           `gorm:"not null;index;uniqueIndex:ux_quality_bug_key_scope" json:"workspace_id"`
	TeamID        uint           `gorm:"not null;index" json:"team_id"`
	ProjectSetID  uint           `gorm:"not null;index" json:"project_set_id"`
	ProjectID     uint           `gorm:"not null;index" json:"project_id"`
	RequirementID uint           `gorm:"not null;index" json:"requirement_id"`
	BugKey        string         `gorm:"size:64;not null;index;uniqueIndex:ux_quality_bug_key_scope" json:"bug_key"`
	Title         string         `gorm:"size:255;not null" json:"title"`
	Status        string         `gorm:"size:32;not null;default:triage;index" json:"status"`
	Priority      string         `gorm:"size:32;not null;default:medium;index" json:"priority"`
	BugType       string         `gorm:"size:64;index" json:"bug_type,omitempty"`
	DefectDomain  string         `gorm:"size:32;index" json:"defect_domain,omitempty"`
	SeverityKey   string         `gorm:"size:32;index" json:"severity_key"`
	DueAt         *time.Time     `gorm:"index" json:"due_at,omitempty"`

	FoundEnvironment          string `gorm:"type:text" json:"found_environment,omitempty"`
	OperatingSystem           string `gorm:"type:text" json:"operating_system,omitempty"`
	Browser                   string `gorm:"type:text" json:"browser,omitempty"`
	FoundVersion              string `gorm:"type:text" json:"found_version,omitempty"`
	ReproductionSteps         string `gorm:"type:text" json:"reproduction_steps,omitempty"`
	ExpectedResult            string `gorm:"type:text" json:"expected_result,omitempty"`
	ActualResult              string `gorm:"type:text" json:"actual_result,omitempty"`
	ReproductionMaterialsJSON string `gorm:"type:text" json:"-"`
	ResolutionReason          string `gorm:"size:64;index" json:"resolution_reason,omitempty"`
	ResolutionNote            string `gorm:"type:text" json:"resolution_note,omitempty"`
	ResolutionMaterialsJSON   string `gorm:"type:text" json:"-"`

	AssigneeUserID    *uint      `gorm:"index" json:"assignee_user_id,omitempty"`
	ReporterUserID    *uint      `gorm:"index" json:"reporter_user_id,omitempty"`
	ReviewerUserID    *uint      `gorm:"index" json:"reviewer_user_id,omitempty"`
	SourceExecutionID *uint      `gorm:"index" json:"source_execution_id,omitempty"`
	SourceTestID      *uint      `gorm:"index" json:"source_test_id,omitempty"`
	ResolvedAt        *time.Time `gorm:"index" json:"resolved_at,omitempty"`
	ReviewedAt        *time.Time `gorm:"index" json:"reviewed_at,omitempty"`
	CompletedAt       *time.Time `gorm:"index" json:"completed_at,omitempty"`
	CanceledAt        *time.Time `gorm:"index" json:"canceled_at,omitempty"`
	StatusChangedAt   time.Time  `gorm:"not null;index" json:"status_changed_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type QualityBugActivity struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;index" json:"workspace_id"`
	QualityBugID uint      `gorm:"not null;index" json:"quality_bug_id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	Action       string    `gorm:"size:40;not null;index" json:"action"`
	FieldKey     string    `gorm:"size:128" json:"field_key,omitempty"`
	OldValue     string    `gorm:"type:text" json:"old_value,omitempty"`
	NewValue     string    `gorm:"type:text" json:"new_value,omitempty"`
	Summary      string    `gorm:"type:text" json:"summary,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// QualityBugComment is a PM-owned quality comment. UserID stores the shared
// platform fact id so comment authors and notification recipients use the same
// identity boundary as task collaboration.
type QualityBugComment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;index:idx_quality_comment_scope,priority:1" json:"workspace_id"`
	QualityBugID uint      `gorm:"not null;index:idx_quality_comment_scope,priority:2;index" json:"quality_bug_id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	Body         string    `gorm:"type:text;not null" json:"body"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

// QualitySceneOption stores workspace-owned environment, operating-system and
// browser options. Built-in options are supplied by the domain and not stored.
type QualitySceneOption struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint      `gorm:"not null;index;uniqueIndex:ux_quality_scene_option_scope" json:"workspace_id"`
	Kind            string    `gorm:"size:32;not null;index;uniqueIndex:ux_quality_scene_option_scope" json:"kind"`
	Name            string    `gorm:"size:64;not null" json:"name"`
	NormalizedName  string    `gorm:"size:64;not null;uniqueIndex:ux_quality_scene_option_scope" json:"-"`
	CreatedByUserID uint      `gorm:"not null;index" json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
}
