package qualitygorm

import "time"

type ToolMutationApply struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;index:idx_quality_tool_apply_workspace"`
	ApplyKey    string    `gorm:"size:128;not null;uniqueIndex:ux_quality_tool_apply_key"`
	ChangeSetID string    `gorm:"size:128;not null;uniqueIndex:ux_quality_tool_change_set"`
	Operation   string    `gorm:"size:80;not null"`
	InputDigest string    `gorm:"size:64;not null"`
	ResultJSON  string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (ToolMutationApply) TableName() string { return "quality_tool_mutation_applies" }

func MutationMigrationRecords() []any { return []any{&ToolMutationApply{}} }
