// Package projectgorm contains project-module GORM records.
package projectgorm

import "time"

type ProjectSet struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint       `gorm:"not null;index:idx_project_workspace_team" json:"workspace_id"`
	TeamID          uint       `gorm:"not null;index:idx_project_workspace_team" json:"team_id"`
	Name            string     `gorm:"size:180;not null" json:"name"`
	Code            string     `gorm:"size:40;index" json:"code,omitempty"`
	Description     string     `gorm:"type:text" json:"description,omitempty"`
	OwnerUserID     *uint      `gorm:"index" json:"owner_user_id,omitempty"`
	CreatedByUserID *uint      `gorm:"column:created_by_user_id;index" json:"created_by_user_id,omitempty"`
	PlannedStartAt  *time.Time `gorm:"column:planned_start_at" json:"starts_at,omitempty"`
	PlannedEndAt    *time.Time `gorm:"column:planned_end_at" json:"due_at,omitempty"`
	Status          string     `gorm:"size:30;not null;default:active" json:"status"`
	MutationState   string     `gorm:"size:24;not null;default:'';index" json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type PlanUnit struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID           uint       `gorm:"not null;index" json:"workspace_id"`
	TeamID                uint       `gorm:"not null;index" json:"team_id"`
	ProjectSetID          uint       `gorm:"column:project_new_id;not null;index" json:"project_new_id"`
	Title                 string     `gorm:"size:180;not null" json:"title"`
	Type                  string     `gorm:"size:30;not null;default:version" json:"type"`
	Goal                  string     `gorm:"type:text" json:"goal,omitempty"`
	OwnerUserID           *uint      `gorm:"index" json:"owner_user_id,omitempty"`
	PlannedStartAt        *time.Time `json:"starts_at,omitempty"`
	PlannedEndAt          *time.Time `json:"due_at,omitempty"`
	DesignFreezeDueAt     *time.Time `json:"design_freeze_due_at,omitempty"`
	DevelopmentCompleteAt *time.Time `gorm:"column:development_complete_due_at" json:"development_complete_due_at,omitempty"`
	TestAcceptanceDueAt   *time.Time `json:"test_acceptance_due_at,omitempty"`
	ActualStartAt         *time.Time `json:"actual_start_at,omitempty"`
	ActualCompletedAt     *time.Time `json:"actual_completed_at,omitempty"`
	RiskLevel             string     `gorm:"size:30" json:"risk_level,omitempty"`
	Status                string     `gorm:"size:30;not null;default:planning" json:"status"`
	MutationState         string     `gorm:"size:24;not null;default:'';index" json:"-"`
	// RequirementsIntakeLock records that requirement intake was completed and
	// confirmed for this plan unit: non-nil since = no new requirements may be
	// created here, while existing requirements can still move in or reschedule.
	RequirementsLockedAt       *time.Time `gorm:"column:requirements_locked_at" json:"requirements_locked_at,omitempty"`
	RequirementsLockedByUserID *uint      `gorm:"column:requirements_locked_by_user_id" json:"requirements_locked_by_user_id,omitempty"`
	// DeliveryScopeRevision increments whenever the current requirement set of
	// this plan unit changes (create / move / schedule / delete). It is a
	// monotonic fact for traceability and future cache invalidation, never a
	// rule that restricts requirement moves.
	DeliveryScopeRevision int64     `gorm:"column:delivery_scope_revision;not null;default:0" json:"delivery_scope_revision"`
	SortOrder             int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type ProjectNews struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;index" json:"workspace_id"`
	ProjectSetID uint      `gorm:"not null;index" json:"project_set_id"`
	PlanUnitID   uint      `gorm:"not null;index" json:"plan_unit_id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	Title        string    `gorm:"size:180;not null" json:"title"`
	Content      string    `gorm:"type:text" json:"content"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DeliveryNodeLabel struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;index" json:"workspace_id"`
	ProjectSetID uint      `gorm:"not null;index" json:"project_set_id"`
	PlanUnitID   uint      `gorm:"not null;index" json:"plan_unit_id"`
	ClientKey    string    `gorm:"size:80" json:"client_key,omitempty"`
	Name         string    `gorm:"size:120;not null" json:"name"`
	SystemKey    string    `gorm:"size:80" json:"system_key,omitempty"`
	SortOrder    int       `gorm:"not null;default:0" json:"sort_order"`
	Deleted      bool      `gorm:"not null;default:false" json:"deleted"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DeliveryNode struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID       uint       `gorm:"not null;index" json:"workspace_id"`
	PlanUnitID        uint       `gorm:"not null;index" json:"plan_unit_id"`
	LabelID           *uint      `gorm:"index" json:"label_id,omitempty"`
	LabelClientKey    string     `gorm:"size:80" json:"label_client_key,omitempty"`
	LabelName         string     `gorm:"size:120" json:"label_name,omitempty"`
	Type              string     `gorm:"size:80" json:"type,omitempty"`
	DueAt             *time.Time `json:"due_at,omitempty"`
	ActualCompletedAt *time.Time `json:"actual_completed_at,omitempty"`
	OwnerUserID       *uint      `gorm:"index" json:"owner_user_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
