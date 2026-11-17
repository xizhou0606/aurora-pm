package projectgorm

import "time"

type ToolMutationApply struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;index:idx_project_tool_apply_workspace"`
	ApplyKey    string    `gorm:"size:128;not null;uniqueIndex:ux_project_tool_apply_key"`
	Operation   string    `gorm:"size:80;not null"`
	InputDigest string    `gorm:"size:64;not null"`
	ResultJSON  string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (ToolMutationApply) TableName() string { return "project_tool_mutation_applies" }

type ProjectDeletionSaga struct {
	ID                 uint      `gorm:"primaryKey"`
	WorkspaceID        uint      `gorm:"not null;index:idx_project_delete_saga_scope"`
	ProjectSetID       uint      `gorm:"not null;index:idx_project_delete_saga_scope"`
	ApplyKey           string    `gorm:"size:128;not null;uniqueIndex:ux_project_delete_saga_apply"`
	InputDigest        string    `gorm:"size:64;not null"`
	SubjectDigest      string    `gorm:"size:64;not null"`
	PlanUnitIDsJSON    string    `gorm:"type:text;not null"`
	RequirementIDsJSON string    `gorm:"type:text;not null"`
	NextStep           int       `gorm:"not null"`
	Status             string    `gorm:"size:24;not null;index"`
	ResultJSON         string    `gorm:"type:text;not null"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (ProjectDeletionSaga) TableName() string { return "project_deletion_sagas" }

type PlanDeletionSaga struct {
	ID           uint      `gorm:"primaryKey"`
	WorkspaceID  uint      `gorm:"not null;index"`
	ProjectSetID uint      `gorm:"not null;index"`
	PlanUnitID   uint      `gorm:"not null;index"`
	ApplyKey     string    `gorm:"size:128;not null;uniqueIndex"`
	InputDigest  string    `gorm:"size:64;not null"`
	NextStep     int       `gorm:"not null"`
	Status       string    `gorm:"size:24;not null;index"`
	ResultJSON   string    `gorm:"type:text;not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (PlanDeletionSaga) TableName() string { return "project_plan_deletion_sagas" }

type PlanCreationSaga struct {
	ID                 uint      `gorm:"primaryKey"`
	WorkspaceID        uint      `gorm:"not null;index"`
	ProjectSetID       uint      `gorm:"not null;index"`
	PlanUnitID         uint      `gorm:"not null;index"`
	WorkflowTemplateID uint      `gorm:"not null;index"`
	WorkflowInstanceID uint      `gorm:"not null;default:0;index"`
	ApplyKey           string    `gorm:"size:128;not null;uniqueIndex"`
	InputDigest        string    `gorm:"size:64;not null"`
	SubjectStatus      string    `gorm:"size:30;not null"`
	NextStep           int       `gorm:"not null"`
	Status             string    `gorm:"size:24;not null;index"`
	ResultJSON         string    `gorm:"type:text;not null"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (PlanCreationSaga) TableName() string { return "project_plan_creation_sagas" }

func MutationMigrationRecords() []any { return []any{&ToolMutationApply{}} }

// DeletionMigrationRecords is versioned separately because project deletion
// adds both the durable saga row and ProjectSet.MutationState.
func DeletionMigrationRecords() []any { return []any{&ProjectDeletionSaga{}, &ProjectSet{}} }

func PlanMutationMigrationRecords() []any {
	return []any{&PlanUnit{}, &PlanDeletionSaga{}, &PlanCreationSaga{}}
}
