package workflowgorm

import "time"

type ToolMutationApply struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;index:idx_workflow_tool_apply_workspace"`
	ApplyKey    string    `gorm:"size:128;not null;uniqueIndex:ux_workflow_tool_apply_key"`
	Operation   string    `gorm:"size:80;not null"`
	InputDigest string    `gorm:"size:64;not null"`
	ResultJSON  string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (ToolMutationApply) TableName() string { return "workflow_tool_mutation_applies" }

type WorkflowOpinionInvitation struct {
	ID            uint      `gorm:"primaryKey"`
	WorkspaceID   uint      `gorm:"not null;index:idx_workflow_opinion_invite_scope,priority:1"`
	InstanceID    uint      `gorm:"not null;index:idx_workflow_opinion_invite_scope,priority:2"`
	NodeID        string    `gorm:"size:100;not null;index:idx_workflow_opinion_invite_scope,priority:3"`
	InviterUserID uint      `gorm:"not null;index"`
	InviteeUserID uint      `gorm:"not null;index"`
	ApplyKey      string    `gorm:"size:128;not null;index"`
	CreatedAt     time.Time `gorm:"not null"`
}

func (WorkflowOpinionInvitation) TableName() string { return "workflow_opinion_invitations" }

func MutationMigrationRecords() []any {
	return []any{&ToolMutationApply{}, &WorkflowOpinionInvitation{}}
}
