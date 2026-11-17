package workbenchgorm

import "time"

// BITeamMetricSettings selects the requirement workflow approval node used by
// workspace-scoped product review metrics. Changes use optimistic concurrency
// through Version and are audited by the HTTP mutation boundary.
type BITeamMetricSettings struct {
	ID                            uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID                   uint      `gorm:"not null;uniqueIndex" json:"workspace_id"`
	Version                       int       `gorm:"not null;default:1" json:"version"`
	RequirementWorkflowTemplateID uint      `gorm:"not null;index" json:"requirement_workflow_template_id"`
	ApprovalNodeID                string    `gorm:"size:100;not null" json:"approval_node_id"`
	UpdatedByUserID               uint      `gorm:"not null;index" json:"updated_by_user_id"`
	CreatedAt                     time.Time `json:"created_at"`
	UpdatedAt                     time.Time `json:"updated_at"`
}
