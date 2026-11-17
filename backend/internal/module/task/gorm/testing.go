package taskgorm

import "time"

// TaskTestDefinition is the single versioned testing definition owned by a
// task. User identifiers are authoritative platform user facts.
type TaskTestDefinition struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID         uint      `gorm:"not null;uniqueIndex:ux_task_test_definition_scope,priority:1;index" json:"workspace_id"`
	WorkTaskID          uint      `gorm:"not null;uniqueIndex:ux_task_test_definition_scope,priority:2;index" json:"work_task_id"`
	Version             uint      `gorm:"not null;default:1" json:"version"`
	Purpose             string    `gorm:"type:text" json:"purpose,omitempty"`
	Preconditions       string    `gorm:"type:text" json:"preconditions,omitempty"`
	EnvironmentGuidance string    `gorm:"type:text" json:"environment_guidance,omitempty"`
	CasesJSON           string    `gorm:"type:text;not null" json:"-"`
	CreatedByUserID     uint      `gorm:"not null;index" json:"created_by_user_id"`
	UpdatedByUserID     uint      `gorm:"not null;index" json:"updated_by_user_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`

	WorkTask WorkTask `gorm:"foreignKey:WorkTaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

// TaskTestExecution records one execution of a frozen definition version.
type TaskTestExecution struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID          uint       `gorm:"not null;index:idx_task_test_execution_scope,priority:1;index" json:"workspace_id"`
	WorkTaskID           uint       `gorm:"not null;index:idx_task_test_execution_scope,priority:2;index" json:"work_task_id"`
	TaskTestDefinitionID uint       `gorm:"not null;index" json:"task_test_definition_id"`
	DefinitionVersion    uint       `gorm:"not null" json:"definition_version"`
	Environment          string     `gorm:"size:160;not null" json:"environment"`
	BuildVersion         string     `gorm:"size:160" json:"build_version,omitempty"`
	ExecutorUserID       uint       `gorm:"not null;index" json:"executor_user_id"`
	Status               string     `gorm:"size:30;not null;default:pending;index" json:"status"`
	Summary              string     `gorm:"type:text" json:"summary,omitempty"`
	StartedAt            *time.Time `json:"started_at,omitempty"`
	FinishedAt           *time.Time `json:"finished_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	WorkTask WorkTask `gorm:"foreignKey:WorkTaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

// TaskTestExecutionRun stores the result for one definition case in an
// execution. QualityBugID is optional and may only point at a bug in the same
// workspace and task hierarchy.
type TaskTestExecutionRun struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID         uint       `gorm:"not null;index:idx_task_test_run_scope,priority:1;uniqueIndex:ux_task_test_run_case,priority:1;index" json:"workspace_id"`
	WorkTaskID          uint       `gorm:"not null;index:idx_task_test_run_scope,priority:2;index" json:"work_task_id"`
	TaskTestExecutionID uint       `gorm:"not null;index:idx_task_test_run_scope,priority:3;uniqueIndex:ux_task_test_run_case,priority:2;index" json:"task_test_execution_id"`
	Version             uint       `gorm:"not null;default:1" json:"version"`
	Sequence            int        `gorm:"not null" json:"sequence"`
	CaseKey             string     `gorm:"size:100;not null;uniqueIndex:ux_task_test_run_case,priority:3" json:"case_key"`
	Title               string     `gorm:"size:240;not null" json:"title"`
	StepsJSON           string     `gorm:"type:text;not null" json:"-"`
	StepResultsJSON     string     `gorm:"type:text;not null" json:"-"`
	ExpectedResult      string     `gorm:"type:text" json:"expected_result,omitempty"`
	Status              string     `gorm:"size:30;not null;default:pending;index" json:"status"`
	ActualResult        string     `gorm:"type:text" json:"actual_result,omitempty"`
	FailureReason       string     `gorm:"type:text" json:"failure_reason,omitempty"`
	EvidenceJSON        string     `gorm:"type:text;not null" json:"-"`
	QualityBugID        *uint      `gorm:"index" json:"quality_bug_id,omitempty"`
	ExecutedAt          *time.Time `json:"executed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`

	Execution TaskTestExecution `gorm:"foreignKey:TaskTestExecutionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
