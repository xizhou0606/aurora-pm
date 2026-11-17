package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type taskTestRunPatchBody struct {
	Version       *uint                 `json:"version"`
	Status        *string               `json:"status"`
	ActualResult  *string               `json:"actual_result"`
	FailureReason *string               `json:"failure_reason"`
	Evidence      *[]taskTestEvidence   `json:"evidence"`
	StepResults   *[]taskTestStepResult `json:"step_results"`
	QualityBugID  optionalTaskTestUint  `json:"quality_bug_id"`
	ExecutedAt    *string               `json:"executed_at"`
}

func (s *Server) patchTaskTestExecutionRun(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	runID, valid := paramID(c, "runId")
	if !valid {
		return
	}
	task, project, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	var body taskTestRunPatchBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.Version == nil || *body.Version == 0 {
		bad(c, "version required")
		return
	}
	if body.Status == nil && body.ActualResult == nil && body.FailureReason == nil &&
		body.Evidence == nil && body.StepResults == nil && !body.QualityBugID.Set && body.ExecutedAt == nil {
		bad(c, "no updates provided")
		return
	}
	var current taskgorm.TaskTestExecutionRun
	if err := s.db.Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", runID, task.WorkspaceID, task.ID,
	).First(&current).Error; err != nil {
		fail(c, http.StatusNotFound, "task test execution run not found")
		return
	}
	if current.Version != *body.Version {
		fail(c, http.StatusConflict, "task test execution run version conflict")
		return
	}
	updates, err := s.taskTestRunUpdates(task, current, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	var result taskgorm.TaskTestExecutionRun
	var execution taskgorm.TaskTestExecution
	err = s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		updates["version"] = current.Version + 1
		updates["updated_at"] = now
		updated := tx.Model(&taskgorm.TaskTestExecutionRun{}).
			Where("id = ? AND workspace_id = ? AND work_task_id = ? AND version = ?",
				current.ID, task.WorkspaceID, task.ID, current.Version).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errTaskTestVersionConflict
		}
		if err := tx.Where(
			"id = ? AND workspace_id = ? AND work_task_id = ?",
			current.ID, task.WorkspaceID, task.ID,
		).First(&result).Error; err != nil {
			return err
		}
		if err := tx.Where(
			"id = ? AND workspace_id = ? AND work_task_id = ?",
			result.TaskTestExecutionID, task.WorkspaceID, task.ID,
		).First(&execution).Error; err != nil {
			return err
		}
		status, finishedAt, err := deriveTaskTestExecutionStatus(tx, execution)
		if err != nil {
			return err
		}
		executionUpdates := map[string]any{"status": status, "finished_at": finishedAt, "updated_at": now}
		if err := tx.Model(&taskgorm.TaskTestExecution{}).
			Where("id = ? AND workspace_id = ?", execution.ID, task.WorkspaceID).
			Updates(executionUpdates).Error; err != nil {
			return err
		}
		execution.Status = status
		execution.FinishedAt = finishedAt
		execution.UpdatedAt = now
		return s.recordMutation(
			tx, c, project.ID, "task_test_execution_run", result.ID, "updated", task.Title,
			gin.H{"run": taskTestRunDTO(result), "execution_status": status},
		)
	})
	if errors.Is(err, errTaskTestVersionConflict) {
		fail(c, http.StatusConflict, "task test execution run version conflict")
		return
	}
	if err != nil {
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
		"run":       taskTestRunDTO(result),
		"execution": taskTestExecutionDTO(execution, counts[execution.ID]),
	})
}

func (s *Server) taskTestRunUpdates(
	task taskgorm.WorkTask,
	current taskgorm.TaskTestExecutionRun,
	body taskTestRunPatchBody,
) (map[string]any, error) {
	updates := map[string]any{}
	status := current.Status
	if body.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*body.Status))
		if !validTaskTestRunStatus(status) {
			return nil, errors.New("status is invalid")
		}
		updates["status"] = status
	}
	actualResult := current.ActualResult
	if body.ActualResult != nil {
		actualResult = strings.TrimSpace(*body.ActualResult)
		if utf8.RuneCountInString(actualResult) > maxTaskTestTextRunes {
			return nil, errors.New("actual_result is too long")
		}
		updates["actual_result"] = actualResult
	}
	failureReason := current.FailureReason
	if body.FailureReason != nil {
		failureReason = strings.TrimSpace(*body.FailureReason)
		if utf8.RuneCountInString(failureReason) > maxTaskTestTextRunes {
			return nil, errors.New("failure_reason is too long")
		}
		updates["failure_reason"] = failureReason
	}
	if (status == "failed" || status == "blocked") && failureReason == "" {
		return nil, errors.New("failure_reason required for failed or blocked run")
	}
	if body.Evidence != nil {
		evidence, err := normalizeTaskTestEvidence(*body.Evidence)
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(evidence)
		updates["evidence_json"] = string(raw)
	}
	if body.StepResults != nil {
		steps := make([]taskTestStep, 0)
		if json.Unmarshal([]byte(current.StepsJSON), &steps) != nil || len(steps) == 0 {
			return nil, errors.New("frozen definition steps are invalid")
		}
		stepResults, err := normalizeTaskTestStepResults(*body.StepResults, len(steps))
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(stepResults)
		updates["step_results_json"] = string(raw)
	}
	if body.QualityBugID.Set {
		if body.QualityBugID.Value == nil {
			updates["quality_bug_id"] = nil
		} else {
			var bug qualitygorm.QualityBug
			if err := s.db.Where(
				"id = ? AND workspace_id = ? AND project_set_id = ? AND requirement_id = ?",
				*body.QualityBugID.Value, task.WorkspaceID, task.ProjectSetID, task.RequirementID,
			).First(&bug).Error; err != nil {
				return nil, errors.New("quality_bug_id is outside the task workspace or hierarchy")
			}
			updates["quality_bug_id"] = bug.ID
		}
	}
	if body.ExecutedAt != nil {
		executedAt, err := parseTaskTestTime(*body.ExecutedAt)
		if err != nil {
			return nil, err
		}
		updates["executed_at"] = executedAt
	} else if status == "passed" || status == "failed" || status == "blocked" || status == "skipped" {
		if current.ExecutedAt == nil {
			now := time.Now().UTC()
			updates["executed_at"] = &now
		}
	}
	return updates, nil
}

func deriveTaskTestExecutionStatus(
	tx *gorm.DB,
	execution taskgorm.TaskTestExecution,
) (string, *time.Time, error) {
	var rows []taskgorm.TaskTestExecutionRun
	if err := tx.Select("status").Where(
		"workspace_id = ? AND task_test_execution_id = ?", execution.WorkspaceID, execution.ID,
	).Find(&rows).Error; err != nil {
		return "", nil, err
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Status]++
	}
	status := "running"
	terminal := false
	switch {
	case counts["failed"] > 0:
		status, terminal = "failed", true
	case counts["blocked"] > 0:
		status, terminal = "blocked", true
	case len(rows) > 0 && counts["passed"]+counts["skipped"] == len(rows):
		status, terminal = "passed", true
	}
	if !terminal {
		return status, nil, nil
	}
	if execution.Status == status && execution.FinishedAt != nil {
		return status, execution.FinishedAt, nil
	}
	now := time.Now().UTC()
	return status, &now, nil
}
