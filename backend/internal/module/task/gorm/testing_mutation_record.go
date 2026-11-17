package taskgorm

import "time"

type TestingToolMutationApply struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;index:idx_task_testing_tool_apply_workspace"`
	ApplyKey    string    `gorm:"size:128;not null;uniqueIndex:ux_task_testing_tool_apply_key"`
	ChangeSetID string    `gorm:"size:128;not null;uniqueIndex:ux_task_testing_tool_change_set"`
	Operation   string    `gorm:"size:80;not null"`
	InputDigest string    `gorm:"size:64;not null"`
	ResultJSON  string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (TestingToolMutationApply) TableName() string { return "task_testing_tool_mutation_applies" }

func TestingMutationMigrationRecords() []any { return []any{&TestingToolMutationApply{}} }
