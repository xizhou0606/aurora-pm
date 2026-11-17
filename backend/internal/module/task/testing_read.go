package task

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrTestingReadInvalid     = errors.New("task testing read query is invalid")
	ErrTestDefinitionNotFound = errors.New("task test definition not found")
	ErrTestExecutionNotFound  = errors.New("task test execution not found")
	ErrTestingDataInvalid     = errors.New("task testing stored data is invalid")
)

type TestStep struct {
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result,omitempty"`
}

type TestCase struct {
	Key            string     `json:"key"`
	Title          string     `json:"title"`
	Steps          []TestStep `json:"steps"`
	ExpectedResult string     `json:"expected_result,omitempty"`
}

type TestDefinitionSnapshot struct {
	ID                  uint       `json:"id"`
	WorkspaceID         uint       `json:"workspace_id"`
	WorkTaskID          uint       `json:"work_task_id"`
	Version             uint       `json:"version"`
	Purpose             string     `json:"purpose,omitempty"`
	Preconditions       string     `json:"preconditions,omitempty"`
	EnvironmentGuidance string     `json:"environment_guidance,omitempty"`
	Cases               []TestCase `json:"cases"`
	CreatedByUserID     uint       `json:"created_by_user_id"`
	UpdatedByUserID     uint       `json:"updated_by_user_id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type TestExecutionSnapshot struct {
	ID                   uint             `json:"id"`
	WorkspaceID          uint             `json:"workspace_id"`
	WorkTaskID           uint             `json:"work_task_id"`
	TaskTestDefinitionID uint             `json:"task_test_definition_id"`
	DefinitionVersion    uint             `json:"definition_version"`
	Environment          string           `json:"environment"`
	BuildVersion         string           `json:"build_version,omitempty"`
	ExecutorUserID       uint             `json:"executor_user_id"`
	Status               string           `json:"status"`
	Summary              string           `json:"summary,omitempty"`
	RunCounts            map[string]int64 `json:"run_counts"`
	StartedAt            *time.Time       `json:"started_at,omitempty"`
	FinishedAt           *time.Time       `json:"finished_at,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

type TestEvidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Label     string `json:"label,omitempty"`
}

type TestStepResult struct {
	Sequence      int    `json:"sequence"`
	Status        string `json:"status"`
	ActualResult  string `json:"actual_result,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type TestExecutionRunSnapshot struct {
	ID                  uint             `json:"id"`
	WorkspaceID         uint             `json:"workspace_id"`
	WorkTaskID          uint             `json:"work_task_id"`
	TaskTestExecutionID uint             `json:"task_test_execution_id"`
	Version             uint             `json:"version"`
	Sequence            int              `json:"sequence"`
	CaseKey             string           `json:"case_key"`
	Title               string           `json:"title"`
	Steps               []TestStep       `json:"steps"`
	StepResults         []TestStepResult `json:"step_results"`
	ExpectedResult      string           `json:"expected_result,omitempty"`
	Status              string           `json:"status"`
	ActualResult        string           `json:"actual_result,omitempty"`
	FailureReason       string           `json:"failure_reason,omitempty"`
	Evidence            []TestEvidence   `json:"evidence"`
	QualityBugID        *uint            `json:"quality_bug_id,omitempty"`
	ExecutedAt          *time.Time       `json:"executed_at,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
}

type TestExecutionPage struct {
	Items []TestExecutionSnapshot `json:"items"`
	Total int64                   `json:"total"`
}

type TestExecutionRunPage struct {
	Items []TestExecutionRunSnapshot `json:"items"`
	Total int64                      `json:"total"`
}

type TestRunFilter struct {
	Status string
	Limit  int
	Offset int
}

type TestingReadStore interface {
	GetTestDefinition(context.Context, uint, uint) (TestDefinitionSnapshot, error)
	ListTestExecutions(context.Context, uint, uint, int, int) (TestExecutionPage, error)
	GetTestExecution(context.Context, uint, uint, uint) (TestExecutionSnapshot, error)
	ListTestExecutionRuns(context.Context, uint, uint, uint, TestRunFilter) (TestExecutionRunPage, error)
}

type TestingReadService struct{ store TestingReadStore }

func NewTestingReadService(store TestingReadStore) (*TestingReadService, error) {
	if store == nil {
		return nil, ErrTestingReadInvalid
	}
	return &TestingReadService{store: store}, nil
}

func (service *TestingReadService) GetDefinition(ctx context.Context, workspaceID, taskID uint) (TestDefinitionSnapshot, error) {
	if workspaceID == 0 || taskID == 0 {
		return TestDefinitionSnapshot{}, ErrTestingReadInvalid
	}
	return service.store.GetTestDefinition(ctx, workspaceID, taskID)
}

func (service *TestingReadService) ListExecutions(ctx context.Context, workspaceID, taskID uint, limit, offset int) (TestExecutionPage, error) {
	if workspaceID == 0 || taskID == 0 || offset < 0 {
		return TestExecutionPage{}, ErrTestingReadInvalid
	}
	return service.store.ListTestExecutions(ctx, workspaceID, taskID, testingLimit(limit, 50), offset)
}

func (service *TestingReadService) GetExecution(ctx context.Context, workspaceID, taskID, executionID uint) (TestExecutionSnapshot, error) {
	if workspaceID == 0 || taskID == 0 || executionID == 0 {
		return TestExecutionSnapshot{}, ErrTestingReadInvalid
	}
	return service.store.GetTestExecution(ctx, workspaceID, taskID, executionID)
}

func (service *TestingReadService) ListExecutionRuns(ctx context.Context, workspaceID, taskID, executionID uint, filter TestRunFilter) (TestExecutionRunPage, error) {
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if workspaceID == 0 || taskID == 0 || executionID == 0 || filter.Offset < 0 ||
		(filter.Status != "" && !validTestRunStatus(filter.Status)) {
		return TestExecutionRunPage{}, ErrTestingReadInvalid
	}
	filter.Limit = testingLimit(filter.Limit, 200)
	return service.store.ListTestExecutionRuns(ctx, workspaceID, taskID, executionID, filter)
}

func validTestRunStatus(value string) bool {
	switch value {
	case "pending", "running", "passed", "failed", "blocked", "skipped":
		return true
	default:
		return false
	}
}

func testingLimit(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value > 500 {
		return 500
	}
	return value
}
