package requirementgorm

import "time"

type ToolMutationApply struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;index"`
	ApplyKey    string    `gorm:"size:128;not null;uniqueIndex"`
	ChangeSetID string    `gorm:"size:64;not null;index"`
	Operation   string    `gorm:"size:80;not null"`
	InputDigest string    `gorm:"size:64;not null"`
	ResultJSON  string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (ToolMutationApply) TableName() string { return "requirement_tool_mutation_applies" }
func MutationMigrationRecords() []any       { return []any{&Requirement{}, &ToolMutationApply{}} }
