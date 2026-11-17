// Package workflowgorm contains workflow-module GORM records.
package workflowgorm

import "time"

type WorkflowTemplate struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID      uint      `gorm:"not null;index:idx_workflow_template_scope,priority:1" json:"workspace_id"`
	Name             string    `gorm:"size:160;not null;index:idx_workflow_template_scope,priority:2" json:"name"`
	SubjectType      string    `gorm:"size:40;not null;index" json:"subject_type"`
	Description      string    `gorm:"type:text" json:"description,omitempty"`
	DraftDefinition  string    `gorm:"type:text;not null" json:"-"`
	PublishedVersion uint      `gorm:"not null;default:0" json:"published_version"`
	CreatedByUserID  uint      `gorm:"not null;index" json:"created_by_user_id"`
	UpdatedByUserID  uint      `gorm:"not null;index" json:"updated_by_user_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type WorkflowTemplateVersion struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint      `gorm:"not null;index" json:"workspace_id"`
	TemplateID      uint      `gorm:"not null;uniqueIndex:ux_workflow_version,priority:1;index" json:"template_id"`
	Version         uint      `gorm:"not null;uniqueIndex:ux_workflow_version,priority:2" json:"version"`
	Definition      string    `gorm:"type:text;not null" json:"-"`
	DefinitionHash  string    `gorm:"size:64;not null" json:"definition_hash"`
	PublishedByUser uint      `gorm:"not null;index" json:"published_by_user_id"`
	PublishedAt     time.Time `gorm:"not null" json:"published_at"`
}

type WorkflowInstance struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID       uint       `gorm:"not null;index:idx_workflow_instance_subject,priority:1" json:"workspace_id"`
	TemplateID        uint       `gorm:"not null;index" json:"template_id"`
	TemplateVersionID uint       `gorm:"not null;index" json:"template_version_id"`
	SubjectType       string     `gorm:"size:40;not null;index:idx_workflow_instance_subject,priority:2" json:"subject_type"`
	SubjectID         uint       `gorm:"not null;index:idx_workflow_instance_subject,priority:3" json:"subject_id"`
	Status            string     `gorm:"size:30;not null;default:running;index" json:"status"`
	CurrentNodeIDs    string     `gorm:"type:text;not null" json:"-"`
	NodePlansJSON     string     `gorm:"type:text;not null;default:'{}'" json:"-"`
	StartedByUserID   uint       `gorm:"not null;index" json:"started_by_user_id"`
	StartedAt         time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type WorkflowToken struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint      `gorm:"not null;index" json:"workspace_id"`
	InstanceID     uint      `gorm:"not null;index" json:"instance_id"`
	NodeID         string    `gorm:"size:100;not null" json:"node_id"`
	Status         string    `gorm:"size:30;not null;default:active;index" json:"status"`
	AssigneeUserID *uint     `gorm:"index" json:"assignee_user_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type WorkflowNodeRun struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint       `gorm:"not null;index" json:"workspace_id"`
	InstanceID   uint       `gorm:"not null;index" json:"instance_id"`
	NodeID       string     `gorm:"size:100;not null;index" json:"node_id"`
	NodeType     string     `gorm:"size:40;not null" json:"node_type"`
	Status       string     `gorm:"size:30;not null;index" json:"status"`
	InputJSON    string     `gorm:"type:text" json:"-"`
	OutputJSON   string     `gorm:"type:text" json:"-"`
	ErrorMessage string     `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt    time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type WorkflowApproval struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint       `gorm:"not null;index" json:"workspace_id"`
	InstanceID     uint       `gorm:"not null;index" json:"instance_id"`
	NodeRunID      uint       `gorm:"not null;index" json:"node_run_id"`
	Status         string     `gorm:"size:30;not null;default:pending;index" json:"status"`
	ApproverUserID *uint      `gorm:"index" json:"approver_user_id,omitempty"`
	Comment        string     `gorm:"type:text" json:"comment,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// WorkflowNodeOpinion stores multi-person consultation comments on a
// pre-review approval node. AuthorUserID is the platform user fact id.
type WorkflowNodeOpinion struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;index:idx_workflow_node_opinion,priority:1;index" json:"workspace_id"`
	InstanceID   uint      `gorm:"not null;index:idx_workflow_node_opinion,priority:2;index" json:"instance_id"`
	NodeID       string    `gorm:"size:100;not null;index:idx_workflow_node_opinion,priority:3" json:"node_id"`
	AuthorUserID uint      `gorm:"not null;index" json:"author_user_id"`
	Body         string    `gorm:"type:text;not null" json:"body"`
	CreatedAt    time.Time `gorm:"not null;index" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null" json:"updated_at"`
}

type WorkflowJob struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint       `gorm:"not null;index" json:"workspace_id"`
	InstanceID     uint       `gorm:"not null;index" json:"instance_id"`
	NodeRunID      uint       `gorm:"not null;uniqueIndex" json:"node_run_id"`
	Kind           string     `gorm:"size:40;not null;index" json:"kind"`
	Status         string     `gorm:"size:30;not null;default:pending;index" json:"status"`
	PayloadJSON    string     `gorm:"type:text" json:"-"`
	IdempotencyKey string     `gorm:"size:160;not null;uniqueIndex" json:"idempotency_key"`
	AvailableAt    time.Time  `gorm:"not null;index" json:"available_at"`
	ClaimedAt      *time.Time `gorm:"index" json:"claimed_at,omitempty"`
	ClaimedBy      string     `gorm:"size:160" json:"claimed_by,omitempty"`
	Attempts       int        `gorm:"not null;default:0" json:"attempts"`
	LastError      string     `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// WorkflowNodeArtifact records a file delivered for one immutable workflow
// node. Object bytes remain in the configured PM-owned object store.
type WorkflowNodeArtifact struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint      `gorm:"not null;index:idx_workflow_node_artifact,priority:1;index" json:"workspace_id"`
	InstanceID     uint      `gorm:"not null;index:idx_workflow_node_artifact,priority:2;index" json:"instance_id"`
	NodeID         string    `gorm:"size:100;not null;index:idx_workflow_node_artifact,priority:3" json:"node_id"`
	Kind           string    `gorm:"size:40;not null;index" json:"kind"`
	Name           string    `gorm:"size:220;not null" json:"name"`
	ContentType    string    `gorm:"size:160;not null" json:"content_type"`
	Size           int64     `gorm:"not null" json:"size"`
	ObjectKey      string    `gorm:"size:1000;not null;uniqueIndex" json:"object_key"`
	URL            string    `gorm:"size:2000;not null" json:"url"`
	AIRunID        *uint     `gorm:"index" json:"ai_run_id,omitempty"`
	UploaderUserID uint      `gorm:"not null;index" json:"uploader_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}
