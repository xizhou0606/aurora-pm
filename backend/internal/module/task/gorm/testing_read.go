package taskgorm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type TestingReadRepository struct{ db *gorm.DB }

func NewTestingReadRepository(db *gorm.DB) *TestingReadRepository {
	return &TestingReadRepository{db: db}
}

func (repository *TestingReadRepository) GetTestDefinition(ctx context.Context, workspaceID, taskID uint) (task.TestDefinitionSnapshot, error) {
	var row TaskTestDefinition
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND work_task_id = ?", workspaceID, taskID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.TestDefinitionSnapshot{}, task.ErrTestDefinitionNotFound
	}
	if err != nil {
		return task.TestDefinitionSnapshot{}, err
	}
	return testDefinitionSnapshot(row)
}

func (repository *TestingReadRepository) ListTestExecutions(ctx context.Context, workspaceID, taskID uint, limit, offset int) (task.TestExecutionPage, error) {
	query := repository.db.WithContext(ctx).Where("workspace_id = ? AND work_task_id = ?", workspaceID, taskID)
	var total int64
	if err := query.Model(&TaskTestExecution{}).Count(&total).Error; err != nil {
		return task.TestExecutionPage{}, err
	}
	var rows []TaskTestExecution
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return task.TestExecutionPage{}, err
	}
	counts, err := repository.testRunCounts(ctx, workspaceID, rows)
	if err != nil {
		return task.TestExecutionPage{}, err
	}
	items := make([]task.TestExecutionSnapshot, 0, len(rows))
	for _, row := range rows {
		item, err := testExecutionSnapshot(row, counts[row.ID])
		if err != nil {
			return task.TestExecutionPage{}, err
		}
		items = append(items, item)
	}
	return task.TestExecutionPage{Items: items, Total: total}, nil
}

func (repository *TestingReadRepository) GetTestExecution(ctx context.Context, workspaceID, taskID, executionID uint) (task.TestExecutionSnapshot, error) {
	var row TaskTestExecution
	err := repository.db.WithContext(ctx).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", executionID, workspaceID, taskID,
	).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.TestExecutionSnapshot{}, task.ErrTestExecutionNotFound
	}
	if err != nil {
		return task.TestExecutionSnapshot{}, err
	}
	counts, err := repository.testRunCounts(ctx, workspaceID, []TaskTestExecution{row})
	if err != nil {
		return task.TestExecutionSnapshot{}, err
	}
	return testExecutionSnapshot(row, counts[row.ID])
}

func (repository *TestingReadRepository) ListTestExecutionRuns(
	ctx context.Context,
	workspaceID, taskID, executionID uint,
	filter task.TestRunFilter,
) (task.TestExecutionRunPage, error) {
	query := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND work_task_id = ? AND task_test_execution_id = ?", workspaceID, taskID, executionID,
	)
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Model(&TaskTestExecutionRun{}).Count(&total).Error; err != nil {
		return task.TestExecutionRunPage{}, err
	}
	var rows []TaskTestExecutionRun
	if err := query.Order("sequence ASC, id ASC").Limit(filter.Limit).Offset(filter.Offset).Find(&rows).Error; err != nil {
		return task.TestExecutionRunPage{}, err
	}
	items := make([]task.TestExecutionRunSnapshot, 0, len(rows))
	for _, row := range rows {
		item, err := testExecutionRunSnapshot(row)
		if err != nil {
			return task.TestExecutionRunPage{}, err
		}
		items = append(items, item)
	}
	return task.TestExecutionRunPage{Items: items, Total: total}, nil
}

type testRunCountRow struct {
	TaskTestExecutionID uint
	Status              string
	Total               int64
}

func (repository *TestingReadRepository) testRunCounts(ctx context.Context, workspaceID uint, executions []TaskTestExecution) (map[uint]map[string]int64, error) {
	result := make(map[uint]map[string]int64, len(executions))
	ids := make([]uint, 0, len(executions))
	for _, execution := range executions {
		ids = append(ids, execution.ID)
		result[execution.ID] = map[string]int64{}
	}
	if len(ids) == 0 {
		return result, nil
	}
	var rows []testRunCountRow
	if err := repository.db.WithContext(ctx).Model(&TaskTestExecutionRun{}).
		Select("task_test_execution_id, status, COUNT(*) AS total").
		Where("workspace_id = ? AND task_test_execution_id IN ?", workspaceID, ids).
		Group("task_test_execution_id, status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TaskTestExecutionID][row.Status] = row.Total
		result[row.TaskTestExecutionID]["total"] += row.Total
	}
	return result, nil
}

func testDefinitionSnapshot(row TaskTestDefinition) (task.TestDefinitionSnapshot, error) {
	cases := make([]task.TestCase, 0)
	if row.ID == 0 || row.WorkspaceID == 0 || row.WorkTaskID == 0 || row.Version == 0 ||
		row.CreatedByUserID == 0 || row.UpdatedByUserID == 0 || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() ||
		json.Unmarshal([]byte(row.CasesJSON), &cases) != nil || len(cases) == 0 {
		return task.TestDefinitionSnapshot{}, task.ErrTestingDataInvalid
	}
	for _, testCase := range cases {
		if testCase.Key == "" || testCase.Title == "" || len(testCase.Steps) == 0 {
			return task.TestDefinitionSnapshot{}, task.ErrTestingDataInvalid
		}
		for _, step := range testCase.Steps {
			if step.Action == "" {
				return task.TestDefinitionSnapshot{}, task.ErrTestingDataInvalid
			}
		}
	}
	return task.TestDefinitionSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, WorkTaskID: row.WorkTaskID, Version: row.Version,
		Purpose: row.Purpose, Preconditions: row.Preconditions, EnvironmentGuidance: row.EnvironmentGuidance,
		Cases: cases, CreatedByUserID: row.CreatedByUserID, UpdatedByUserID: row.UpdatedByUserID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func testExecutionSnapshot(row TaskTestExecution, counts map[string]int64) (task.TestExecutionSnapshot, error) {
	if row.ID == 0 || row.WorkspaceID == 0 || row.WorkTaskID == 0 || row.TaskTestDefinitionID == 0 ||
		row.DefinitionVersion == 0 || row.Environment == "" || row.ExecutorUserID == 0 ||
		!validStoredTestingStatus(row.Status) || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return task.TestExecutionSnapshot{}, task.ErrTestingDataInvalid
	}
	if counts == nil {
		counts = map[string]int64{}
	}
	return task.TestExecutionSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, WorkTaskID: row.WorkTaskID,
		TaskTestDefinitionID: row.TaskTestDefinitionID, DefinitionVersion: row.DefinitionVersion,
		Environment: row.Environment, BuildVersion: row.BuildVersion, ExecutorUserID: row.ExecutorUserID,
		Status: row.Status, Summary: row.Summary, RunCounts: counts, StartedAt: row.StartedAt,
		FinishedAt: row.FinishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func testExecutionRunSnapshot(row TaskTestExecutionRun) (task.TestExecutionRunSnapshot, error) {
	steps := make([]task.TestStep, 0)
	stepResults := make([]task.TestStepResult, 0)
	evidence := make([]task.TestEvidence, 0)
	if json.Unmarshal([]byte(row.StepsJSON), &steps) != nil || len(steps) == 0 ||
		json.Unmarshal([]byte(row.StepResultsJSON), &stepResults) != nil ||
		json.Unmarshal([]byte(row.EvidenceJSON), &evidence) != nil {
		return task.TestExecutionRunSnapshot{}, task.ErrTestingDataInvalid
	}
	if row.ID == 0 || row.WorkspaceID == 0 || row.WorkTaskID == 0 || row.TaskTestExecutionID == 0 ||
		row.Version == 0 || row.Sequence < 1 || row.CaseKey == "" || row.Title == "" ||
		!validStoredTestingStatus(row.Status) || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return task.TestExecutionRunSnapshot{}, task.ErrTestingDataInvalid
	}
	for _, step := range steps {
		if step.Action == "" {
			return task.TestExecutionRunSnapshot{}, task.ErrTestingDataInvalid
		}
	}
	for _, result := range stepResults {
		if result.Sequence < 1 || (result.Status != "passed" && result.Status != "failed" && result.Status != "blocked" && result.Status != "skipped") {
			return task.TestExecutionRunSnapshot{}, task.ErrTestingDataInvalid
		}
	}
	for _, item := range evidence {
		if (item.Kind != "url" && item.Kind != "attachment" && item.Kind != "note") || item.Reference == "" {
			return task.TestExecutionRunSnapshot{}, task.ErrTestingDataInvalid
		}
	}
	return task.TestExecutionRunSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, WorkTaskID: row.WorkTaskID,
		TaskTestExecutionID: row.TaskTestExecutionID, Version: row.Version, Sequence: row.Sequence,
		CaseKey: row.CaseKey, Title: row.Title, Steps: steps, StepResults: stepResults,
		ExpectedResult: row.ExpectedResult, Status: row.Status, ActualResult: row.ActualResult,
		FailureReason: row.FailureReason, Evidence: evidence, QualityBugID: row.QualityBugID,
		ExecutedAt: row.ExecutedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func validStoredTestingStatus(value string) bool {
	switch value {
	case "pending", "running", "passed", "failed", "blocked", "skipped":
		return true
	default:
		return false
	}
}

var _ task.TestingReadStore = (*TestingReadRepository)(nil)
