package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type taskTestExecutionCreateBody struct {
	DefinitionVersion uint     `json:"definition_version"`
	Environment       string   `json:"environment"`
	BuildVersion      string   `json:"build_version"`
	ExecutorUserID    *uint    `json:"executor_user_id"`
	Summary           string   `json:"summary"`
	CaseKeys          []string `json:"case_keys"`
}

func (s *Server) createTaskTestExecution(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, project, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	var body taskTestExecutionCreateBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	body.Environment = strings.TrimSpace(body.Environment)
	body.BuildVersion = strings.TrimSpace(body.BuildVersion)
	body.Summary = strings.TrimSpace(body.Summary)
	if body.DefinitionVersion == 0 {
		bad(c, "definition_version required")
		return
	}
	if body.Environment == "" || utf8.RuneCountInString(body.Environment) > 160 {
		bad(c, "environment is invalid")
		return
	}
	if utf8.RuneCountInString(body.BuildVersion) > 160 ||
		utf8.RuneCountInString(body.Summary) > maxTaskTestSummaryRuns {
		bad(c, "build_version or summary is too long")
		return
	}
	executorID := currentFactUserID(c)
	if body.ExecutorUserID != nil {
		executorID = *body.ExecutorUserID
	}
	if executorID == 0 {
		bad(c, "executor_user_id is invalid")
		return
	}
	if err := s.validateActivePMUserReferences(c, []uint{executorID}); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	var definition taskgorm.TaskTestDefinition
	if err := s.db.Where(
		"workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID,
	).First(&definition).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusConflict, "task test definition is required")
			return
		}
		dbFail(c)
		return
	}
	if definition.Version != body.DefinitionVersion {
		fail(c, http.StatusConflict, "task test definition version conflict")
		return
	}
	selectedCases, err := selectTaskTestExecutionCases(taskTestDefinitionCases(definition), body.CaseKeys)
	if err != nil {
		bad(c, err.Error())
		return
	}
	now := time.Now().UTC()
	execution := taskgorm.TaskTestExecution{
		WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID,
		TaskTestDefinitionID: definition.ID, DefinitionVersion: definition.Version,
		Environment: body.Environment, BuildVersion: body.BuildVersion,
		ExecutorUserID: executorID, Status: "running", Summary: body.Summary, StartedAt: &now,
	}
	runs := make([]taskgorm.TaskTestExecutionRun, 0, len(selectedCases))
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current taskgorm.TaskTestDefinition
		if err := tx.Where(
			"id = ? AND workspace_id = ? AND work_task_id = ? AND version = ?",
			definition.ID, task.WorkspaceID, task.ID, definition.Version,
		).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errTaskTestVersionConflict
			}
			return err
		}
		if err := tx.Create(&execution).Error; err != nil {
			return err
		}
		for index, testCase := range selectedCases {
			stepsJSON, _ := json.Marshal(testCase.Steps)
			runs = append(runs, taskgorm.TaskTestExecutionRun{
				WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID,
				TaskTestExecutionID: execution.ID, Version: 1, Sequence: index + 1,
				CaseKey: testCase.Key, Title: testCase.Title,
				ExpectedResult: testCase.ExpectedResult, Status: "pending",
				StepsJSON: string(stepsJSON), StepResultsJSON: "[]", EvidenceJSON: "[]",
			})
		}
		if err := tx.Create(&runs).Error; err != nil {
			return err
		}
		payload := gin.H{
			"execution": taskTestExecutionDTO(execution, map[string]int64{"pending": int64(len(runs)), "total": int64(len(runs))}),
			"runs":      taskTestRunDTOs(runs),
		}
		return s.recordMutation(
			tx, c, project.ID, "task_test_execution", execution.ID, "created", task.Title, payload,
		)
	})
	if errors.Is(err, errTaskTestVersionConflict) {
		fail(c, http.StatusConflict, "task test definition version conflict")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	created(c, gin.H{
		"task":      s.taskTestingTaskContract(task),
		"execution": taskTestExecutionDTO(execution, map[string]int64{"pending": int64(len(runs)), "total": int64(len(runs))}),
		"runs":      taskTestRunDTOs(runs),
	})
}

func selectTaskTestExecutionCases(cases []taskTestCase, requested []string) ([]taskTestCase, error) {
	if len(requested) == 0 {
		return cases, nil
	}
	byKey := make(map[string]taskTestCase, len(cases))
	for _, row := range cases {
		byKey[row.Key] = row
	}
	selected := make([]taskTestCase, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, raw := range requested {
		key := strings.TrimSpace(raw)
		if key == "" {
			return nil, errors.New("case_keys contains an empty key")
		}
		if _, exists := seen[key]; exists {
			return nil, errors.New("case_keys contains a duplicated key")
		}
		row, exists := byKey[key]
		if !exists {
			return nil, errors.New("case_keys contains a key outside the definition")
		}
		seen[key] = struct{}{}
		selected = append(selected, row)
	}
	return selected, nil
}

func taskTestRunDTOs(rows []taskgorm.TaskTestExecutionRun) []gin.H {
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		result = append(result, taskTestRunDTO(row))
	}
	return result
}

func (s *Server) listTaskTestExecutionRuns(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, _, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	executionID := queryUint(c, "execution_id")
	if executionID == 0 {
		bad(c, "execution_id required")
		return
	}
	var execution taskgorm.TaskTestExecution
	if err := s.db.Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", executionID, task.WorkspaceID, task.ID,
	).First(&execution).Error; err != nil {
		fail(c, http.StatusNotFound, "task test execution not found")
		return
	}
	query := s.db.Where(
		"workspace_id = ? AND work_task_id = ? AND task_test_execution_id = ?",
		task.WorkspaceID, task.ID, execution.ID,
	)
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		if !validTaskTestRunStatus(status) {
			bad(c, "status is invalid")
			return
		}
		query = query.Where("status = ?", status)
	}
	var total int64
	if query.Model(&taskgorm.TaskTestExecutionRun{}).Count(&total).Error != nil {
		dbFail(c)
		return
	}
	limit, offset := pagination(c, 200)
	var runs []taskgorm.TaskTestExecutionRun
	if query.Order("sequence ASC, id ASC").Limit(limit).Offset(offset).Find(&runs).Error != nil {
		dbFail(c)
		return
	}
	counts, err := s.taskTestRunCounts(task.WorkspaceID, []taskgorm.TaskTestExecution{execution})
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"task":      s.taskTestingTaskContract(task),
		"execution": taskTestExecutionDTO(execution, counts[execution.ID]),
		"items":     taskTestRunDTOs(runs), "total": total,
	})
}
