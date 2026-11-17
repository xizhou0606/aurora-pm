package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

const (
	maxTaskTestCases       = 200
	maxTaskTestSteps       = 100
	maxTaskTestEvidence    = 20
	maxTaskTestTextRunes   = 20_000
	maxTaskTestSummaryRuns = 4_000
)

var errTaskTestVersionConflict = errors.New("task test version conflict")

type taskTestDefinitionBody struct {
	Version             *uint          `json:"version"`
	Purpose             string         `json:"purpose"`
	Preconditions       string         `json:"preconditions"`
	EnvironmentGuidance string         `json:"environment_guidance"`
	Cases               []taskTestCase `json:"cases"`
}

type taskTestCase struct {
	Key            string         `json:"key"`
	Title          string         `json:"title"`
	Steps          []taskTestStep `json:"steps"`
	ExpectedResult string         `json:"expected_result"`
}

type taskTestStep struct {
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result"`
}

type taskTestStepResult struct {
	Sequence      int    `json:"sequence"`
	Status        string `json:"status"`
	ActualResult  string `json:"actual_result,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type taskTestEvidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Label     string `json:"label,omitempty"`
}

type optionalTaskTestUint struct {
	Set   bool
	Value *uint
}

func (value *optionalTaskTestUint) UnmarshalJSON(raw []byte) error {
	value.Set = true
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		value.Value = nil
		return nil
	}
	var id uint
	if json.Unmarshal(raw, &id) != nil || id == 0 {
		return errors.New("value must be a positive integer or null")
	}
	value.Value = &id
	return nil
}

func normalizeTaskTestDefinitionBody(body taskTestDefinitionBody) (taskTestDefinitionBody, error) {
	body.Purpose = strings.TrimSpace(body.Purpose)
	body.Preconditions = strings.TrimSpace(body.Preconditions)
	body.EnvironmentGuidance = strings.TrimSpace(body.EnvironmentGuidance)
	for name, value := range map[string]string{
		"purpose": body.Purpose, "preconditions": body.Preconditions,
		"environment_guidance": body.EnvironmentGuidance,
	} {
		if utf8.RuneCountInString(value) > maxTaskTestTextRunes {
			return body, fmt.Errorf("%s is too long", name)
		}
	}
	if len(body.Cases) == 0 || len(body.Cases) > maxTaskTestCases {
		return body, fmt.Errorf("cases must contain 1-%d items", maxTaskTestCases)
	}
	seen := make(map[string]struct{}, len(body.Cases))
	for index := range body.Cases {
		row := &body.Cases[index]
		row.Key = strings.TrimSpace(row.Key)
		row.Title = strings.TrimSpace(row.Title)
		row.ExpectedResult = strings.TrimSpace(row.ExpectedResult)
		if row.Key == "" || utf8.RuneCountInString(row.Key) > 100 {
			return body, fmt.Errorf("cases[%d].key is invalid", index)
		}
		if _, exists := seen[row.Key]; exists {
			return body, fmt.Errorf("cases[%d].key is duplicated", index)
		}
		seen[row.Key] = struct{}{}
		if row.Title == "" || utf8.RuneCountInString(row.Title) > 240 {
			return body, fmt.Errorf("cases[%d].title is invalid", index)
		}
		if len(row.Steps) == 0 || len(row.Steps) > maxTaskTestSteps {
			return body, fmt.Errorf("cases[%d].steps must contain 1-%d items", index, maxTaskTestSteps)
		}
		if utf8.RuneCountInString(row.ExpectedResult) > maxTaskTestTextRunes {
			return body, fmt.Errorf("cases[%d].expected_result is too long", index)
		}
		for stepIndex := range row.Steps {
			step := &row.Steps[stepIndex]
			step.Action = strings.TrimSpace(step.Action)
			step.ExpectedResult = strings.TrimSpace(step.ExpectedResult)
			if step.Action == "" || utf8.RuneCountInString(step.Action) > maxTaskTestTextRunes ||
				utf8.RuneCountInString(step.ExpectedResult) > maxTaskTestTextRunes {
				return body, fmt.Errorf("cases[%d].steps[%d] is invalid", index, stepIndex)
			}
		}
	}
	return body, nil
}

func taskTestDefinitionCases(row taskgorm.TaskTestDefinition) []taskTestCase {
	result := make([]taskTestCase, 0)
	_ = json.Unmarshal([]byte(row.CasesJSON), &result)
	return result
}

func taskTestDefinitionDTO(row taskgorm.TaskTestDefinition) gin.H {
	return gin.H{
		"id": row.ID, "workspace_id": row.WorkspaceID, "work_task_id": row.WorkTaskID,
		"version": row.Version, "purpose": row.Purpose, "preconditions": row.Preconditions,
		"environment_guidance": row.EnvironmentGuidance, "cases": taskTestDefinitionCases(row),
		"created_by_user_id": row.CreatedByUserID, "updated_by_user_id": row.UpdatedByUserID,
		"created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
	}
}

func taskTestExecutionDTO(row taskgorm.TaskTestExecution, counts map[string]int64) gin.H {
	return gin.H{
		"id": row.ID, "workspace_id": row.WorkspaceID, "work_task_id": row.WorkTaskID,
		"task_test_definition_id": row.TaskTestDefinitionID,
		"definition_version":      row.DefinitionVersion, "environment": row.Environment,
		"build_version": row.BuildVersion, "executor_user_id": row.ExecutorUserID,
		"status": row.Status, "summary": row.Summary, "run_counts": counts,
		"started_at": row.StartedAt, "finished_at": row.FinishedAt,
		"created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
	}
}

func taskTestRunDTO(row taskgorm.TaskTestExecutionRun) gin.H {
	evidence := make([]taskTestEvidence, 0)
	_ = json.Unmarshal([]byte(row.EvidenceJSON), &evidence)
	steps := make([]taskTestStep, 0)
	_ = json.Unmarshal([]byte(row.StepsJSON), &steps)
	stepResults := make([]taskTestStepResult, 0)
	_ = json.Unmarshal([]byte(row.StepResultsJSON), &stepResults)
	return gin.H{
		"id": row.ID, "workspace_id": row.WorkspaceID, "work_task_id": row.WorkTaskID,
		"task_test_execution_id": row.TaskTestExecutionID, "version": row.Version,
		"sequence": row.Sequence, "case_key": row.CaseKey, "title": row.Title,
		"steps": steps, "step_results": stepResults,
		"expected_result": row.ExpectedResult, "status": row.Status,
		"actual_result": row.ActualResult, "failure_reason": row.FailureReason,
		"evidence": evidence, "quality_bug_id": row.QualityBugID,
		"executed_at": row.ExecutedAt, "created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
	}
}

func validTaskTestRunStatus(value string) bool {
	switch value {
	case "pending", "running", "passed", "failed", "blocked", "skipped":
		return true
	default:
		return false
	}
}

func parseTaskTestTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, errors.New("executed_at must be RFC3339")
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func normalizeTaskTestStepResults(values []taskTestStepResult, stepCount int) ([]taskTestStepResult, error) {
	if len(values) > stepCount {
		return nil, errors.New("step_results exceeds the frozen definition steps")
	}
	seen := make(map[int]struct{}, len(values))
	for index := range values {
		row := &values[index]
		row.Status = strings.ToLower(strings.TrimSpace(row.Status))
		row.ActualResult = strings.TrimSpace(row.ActualResult)
		row.FailureReason = strings.TrimSpace(row.FailureReason)
		if row.Sequence < 1 || row.Sequence > stepCount {
			return nil, fmt.Errorf("step_results[%d].sequence is invalid", index)
		}
		if _, exists := seen[row.Sequence]; exists {
			return nil, fmt.Errorf("step_results[%d].sequence is duplicated", index)
		}
		seen[row.Sequence] = struct{}{}
		if row.Status != "passed" && row.Status != "failed" && row.Status != "blocked" && row.Status != "skipped" {
			return nil, fmt.Errorf("step_results[%d].status is invalid", index)
		}
		if (row.Status == "failed" || row.Status == "blocked") && row.FailureReason == "" {
			return nil, fmt.Errorf("step_results[%d].failure_reason required", index)
		}
		if utf8.RuneCountInString(row.ActualResult) > maxTaskTestTextRunes ||
			utf8.RuneCountInString(row.FailureReason) > maxTaskTestTextRunes {
			return nil, fmt.Errorf("step_results[%d] is too long", index)
		}
	}
	return values, nil
}

func normalizeTaskTestEvidence(values []taskTestEvidence) ([]taskTestEvidence, error) {
	if len(values) > maxTaskTestEvidence {
		return nil, fmt.Errorf("evidence cannot exceed %d items", maxTaskTestEvidence)
	}
	for index := range values {
		row := &values[index]
		row.Kind = strings.ToLower(strings.TrimSpace(row.Kind))
		row.Reference = strings.TrimSpace(row.Reference)
		row.Label = strings.TrimSpace(row.Label)
		if row.Kind != "url" && row.Kind != "attachment" && row.Kind != "note" {
			return nil, fmt.Errorf("evidence[%d].kind is invalid", index)
		}
		if row.Reference == "" || utf8.RuneCountInString(row.Reference) > 2000 ||
			utf8.RuneCountInString(row.Label) > 240 {
			return nil, fmt.Errorf("evidence[%d] is invalid", index)
		}
	}
	return values, nil
}
