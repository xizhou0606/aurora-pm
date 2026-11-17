package taskgorm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TestingMutationRepository struct{ db *gorm.DB }

func NewTestingMutationRepository(db *gorm.DB) (*TestingMutationRepository, error) {
	if db == nil {
		return nil, task.ErrTestingMutationInvalid
	}
	return &TestingMutationRepository{db: db}, nil
}

func (repository *TestingMutationRepository) ApplyTestingMutation(
	ctx context.Context, command task.TestingMutation,
) (task.TestingMutationResult, error) {
	if repository == nil || repository.db == nil {
		return task.TestingMutationResult{}, task.ErrTestingMutationInvalid
	}
	if replay, found, err := repository.replay(ctx, command); err != nil || found {
		return replay, err
	}
	var result task.TestingMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := TestingToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID,
			Operation: string(command.Operation), InputDigest: command.InputDigest,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		if err := lockTestingTask(tx, command); err != nil {
			return err
		}
		var err error
		switch command.Operation {
		case task.TestingDefinitionUpdate:
			var definition task.TestDefinitionSnapshot
			definition, err = applyTestingDefinition(tx, command)
			result.Definition = &definition
		case task.TestingExecutionCreate:
			var execution task.TestExecutionSnapshot
			execution, result.Runs, err = applyTestingExecution(tx, command)
			result.Execution = &execution
		case task.TestingExecutionRunUpdate:
			var run task.TestExecutionRunSnapshot
			var execution task.TestExecutionSnapshot
			run, execution, err = applyTestingRun(tx, command)
			result.Run, result.Execution = &run, &execution
		default:
			err = task.ErrTestingMutationInvalid
		}
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		updated := tx.Model(&TestingToolMutationApply{}).Where("id = ? AND result_json = ''", ledger.ID).
			Update("result_json", string(encoded))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return task.ErrTestingMutationConflict
		}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replay(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return task.TestingMutationResult{}, normalizeTestingMutationError(err)
}

func (repository *TestingMutationRepository) replay(
	ctx context.Context, command task.TestingMutation,
) (task.TestingMutationResult, bool, error) {
	var ledger TestingToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.TestingMutationResult{}, false, nil
	}
	if err != nil {
		return task.TestingMutationResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.ApplyKey != command.ApplyKey || ledger.ChangeSetID != command.ChangeSetID ||
		ledger.Operation != string(command.Operation) || ledger.InputDigest != command.InputDigest || ledger.ResultJSON == "" {
		return task.TestingMutationResult{}, true, task.ErrTestingMutationConflict
	}
	var result task.TestingMutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return task.TestingMutationResult{}, true, task.ErrTestingMutationConflict
	}
	return result, true, nil
}

func lockTestingTask(tx *gorm.DB, command task.TestingMutation) error {
	var row WorkTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND mutation_state = ?", command.TaskID, command.WorkspaceID, workTaskStateActive).First(&row).Error
	if err != nil {
		return normalizeTestingMutationError(err)
	}
	if row.TeamID != command.TaskScope.TeamID || row.ProjectSetID != command.TaskScope.ProjectSetID ||
		row.ProjectID != command.TaskScope.PlanUnitID || row.RequirementID != command.TaskScope.RequirementID {
		return task.ErrTestingMutationConflict
	}
	return nil
}

func applyTestingDefinition(tx *gorm.DB, command task.TestingMutation) (task.TestDefinitionSnapshot, error) {
	var row TaskTestDefinition
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND work_task_id = ?", command.WorkspaceID, command.TaskID,
	).First(&row).Error
	casesJSON, marshalErr := json.Marshal(command.Definition.Cases)
	if marshalErr != nil {
		return task.TestDefinitionSnapshot{}, task.ErrTestingMutationInvalid
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if command.Definition.Version != 0 {
			return task.TestDefinitionSnapshot{}, task.ErrTestingMutationConflict
		}
		row = TaskTestDefinition{
			WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, Version: 1,
			Purpose: command.Definition.Purpose, Preconditions: command.Definition.Preconditions,
			EnvironmentGuidance: command.Definition.EnvironmentGuidance, CasesJSON: string(casesJSON),
			CreatedByUserID: command.PrincipalID, UpdatedByUserID: command.PrincipalID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return task.TestDefinitionSnapshot{}, err
		}
		return testDefinitionSnapshot(row)
	}
	if err != nil {
		return task.TestDefinitionSnapshot{}, err
	}
	if command.Definition.Version == 0 || command.Definition.Version != row.Version {
		return task.TestDefinitionSnapshot{}, task.ErrTestingMutationConflict
	}
	updates := map[string]any{
		"version": row.Version + 1, "purpose": command.Definition.Purpose,
		"preconditions": command.Definition.Preconditions, "environment_guidance": command.Definition.EnvironmentGuidance,
		"cases_json": string(casesJSON), "updated_by_user_id": command.PrincipalID,
	}
	updated := tx.Model(&TaskTestDefinition{}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ? AND version = ?", row.ID, command.WorkspaceID, command.TaskID, row.Version,
	).Updates(updates)
	if updated.Error != nil {
		return task.TestDefinitionSnapshot{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return task.TestDefinitionSnapshot{}, task.ErrTestingMutationConflict
	}
	if err := tx.Where("id = ? AND workspace_id = ?", row.ID, command.WorkspaceID).First(&row).Error; err != nil {
		return task.TestDefinitionSnapshot{}, err
	}
	return testDefinitionSnapshot(row)
}

func applyTestingExecution(tx *gorm.DB, command task.TestingMutation) (
	task.TestExecutionSnapshot, []task.TestExecutionRunSnapshot, error,
) {
	var definition TaskTestDefinition
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND work_task_id = ?", command.WorkspaceID, command.TaskID,
	).First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.TestExecutionSnapshot{}, nil, task.ErrTestingMutationConflict
	}
	if err != nil {
		return task.TestExecutionSnapshot{}, nil, err
	}
	if definition.Version != command.Execution.DefinitionVersion {
		return task.TestExecutionSnapshot{}, nil, task.ErrTestingMutationConflict
	}
	definitionSnapshot, err := testDefinitionSnapshot(definition)
	if err != nil {
		return task.TestExecutionSnapshot{}, nil, task.ErrTestingMutationConflict
	}
	selected, err := selectTestingCases(definitionSnapshot.Cases, command.Execution.CaseKeys)
	if err != nil {
		return task.TestExecutionSnapshot{}, nil, err
	}
	now := time.Now().UTC()
	execution := TaskTestExecution{
		WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, TaskTestDefinitionID: definition.ID,
		DefinitionVersion: definition.Version, Environment: command.Execution.Environment,
		BuildVersion: command.Execution.BuildVersion, ExecutorUserID: command.Execution.ExecutorUserID,
		Status: "running", Summary: command.Execution.Summary, StartedAt: &now,
	}
	if err := tx.Create(&execution).Error; err != nil {
		return task.TestExecutionSnapshot{}, nil, err
	}
	runs := make([]TaskTestExecutionRun, 0, len(selected))
	for index, item := range selected {
		steps, err := json.Marshal(item.Steps)
		if err != nil {
			return task.TestExecutionSnapshot{}, nil, task.ErrTestingMutationInvalid
		}
		runs = append(runs, TaskTestExecutionRun{
			WorkspaceID: command.WorkspaceID, WorkTaskID: command.TaskID, TaskTestExecutionID: execution.ID,
			Version: 1, Sequence: index + 1, CaseKey: item.Key, Title: item.Title, StepsJSON: string(steps),
			StepResultsJSON: "[]", ExpectedResult: item.ExpectedResult, Status: "pending", EvidenceJSON: "[]",
		})
	}
	if len(runs) == 0 {
		return task.TestExecutionSnapshot{}, nil, task.ErrTestingMutationInvalid
	}
	if err := tx.Create(&runs).Error; err != nil {
		return task.TestExecutionSnapshot{}, nil, err
	}
	counts := map[string]int64{"pending": int64(len(runs)), "total": int64(len(runs))}
	executionSnapshot, err := testExecutionSnapshot(execution, counts)
	if err != nil {
		return task.TestExecutionSnapshot{}, nil, err
	}
	runSnapshots := make([]task.TestExecutionRunSnapshot, 0, len(runs))
	for _, row := range runs {
		item, err := testExecutionRunSnapshot(row)
		if err != nil {
			return task.TestExecutionSnapshot{}, nil, err
		}
		runSnapshots = append(runSnapshots, item)
	}
	return executionSnapshot, runSnapshots, nil
}

func selectTestingCases(cases []task.TestCase, requested []string) ([]task.TestCase, error) {
	if len(requested) == 0 {
		return cases, nil
	}
	byKey := make(map[string]task.TestCase, len(cases))
	for _, item := range cases {
		byKey[item.Key] = item
	}
	selected := make([]task.TestCase, 0, len(requested))
	for _, key := range requested {
		item, exists := byKey[key]
		if !exists {
			return nil, task.ErrTestingMutationInvalid
		}
		selected = append(selected, item)
	}
	return selected, nil
}

func applyTestingRun(tx *gorm.DB, command task.TestingMutation) (
	task.TestExecutionRunSnapshot, task.TestExecutionSnapshot, error,
) {
	var row TaskTestExecutionRun
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", command.Run.RunID, command.WorkspaceID, command.TaskID,
	).First(&row).Error
	if err != nil {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, normalizeTestingMutationError(err)
	}
	if row.Version != command.Run.Version {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, task.ErrTestingMutationConflict
	}
	updates := map[string]any{}
	status, failureReason := row.Status, row.FailureReason
	if command.Run.Fields["status"] {
		status = command.Run.Status
		updates["status"] = status
	}
	if command.Run.Fields["actual_result"] {
		updates["actual_result"] = command.Run.ActualResult
	}
	if command.Run.Fields["failure_reason"] {
		failureReason = command.Run.FailureReason
		updates["failure_reason"] = failureReason
	}
	if (status == "failed" || status == "blocked") && failureReason == "" {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, task.ErrTestingMutationInvalid
	}
	if command.Run.Fields["executed_at"] {
		updates["executed_at"] = command.Run.ExecutedAt
	} else if isTerminalTestingStatus(status) && row.ExecutedAt == nil {
		now := time.Now().UTC()
		updates["executed_at"] = &now
	}
	updates["version"] = row.Version + 1
	updated := tx.Model(&TaskTestExecutionRun{}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ? AND version = ?", row.ID, command.WorkspaceID, command.TaskID, row.Version,
	).Updates(updates)
	if updated.Error != nil {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, task.ErrTestingMutationConflict
	}
	if err := tx.Where("id = ? AND workspace_id = ? AND work_task_id = ?", row.ID, command.WorkspaceID, command.TaskID).First(&row).Error; err != nil {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, err
	}
	var execution TaskTestExecution
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", row.TaskTestExecutionID, command.WorkspaceID, command.TaskID,
	).First(&execution).Error; err != nil {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, normalizeTestingMutationError(err)
	}
	status, finishedAt, counts, err := deriveTestingExecutionStatus(tx, execution)
	if err != nil {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, err
	}
	if updated := tx.Model(&TaskTestExecution{}).Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", execution.ID, command.WorkspaceID, command.TaskID,
	).Updates(map[string]any{"status": status, "finished_at": finishedAt}); updated.Error != nil || updated.RowsAffected != 1 {
		if updated.Error != nil {
			return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, updated.Error
		}
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, task.ErrTestingMutationConflict
	}
	if err := tx.Where("id = ? AND workspace_id = ?", execution.ID, command.WorkspaceID).First(&execution).Error; err != nil {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, err
	}
	runSnapshot, err := testExecutionRunSnapshot(row)
	if err != nil {
		return task.TestExecutionRunSnapshot{}, task.TestExecutionSnapshot{}, err
	}
	executionSnapshot, err := testExecutionSnapshot(execution, counts)
	return runSnapshot, executionSnapshot, err
}

func deriveTestingExecutionStatus(tx *gorm.DB, execution TaskTestExecution) (string, *time.Time, map[string]int64, error) {
	var rows []TaskTestExecutionRun
	if err := tx.Where("workspace_id = ? AND work_task_id = ? AND task_test_execution_id = ?",
		execution.WorkspaceID, execution.WorkTaskID, execution.ID).Find(&rows).Error; err != nil {
		return "", nil, nil, err
	}
	counts := map[string]int64{"total": int64(len(rows))}
	for _, row := range rows {
		counts[row.Status]++
	}
	status, terminal := "running", false
	switch {
	case counts["failed"] > 0:
		status, terminal = "failed", true
	case counts["blocked"] > 0:
		status, terminal = "blocked", true
	case len(rows) > 0 && counts["passed"]+counts["skipped"] == int64(len(rows)):
		status, terminal = "passed", true
	}
	if !terminal {
		return status, nil, counts, nil
	}
	if execution.Status == status && execution.FinishedAt != nil {
		return status, execution.FinishedAt, counts, nil
	}
	now := time.Now().UTC()
	return status, &now, counts, nil
}

func isTerminalTestingStatus(status string) bool {
	return status == "passed" || status == "failed" || status == "blocked" || status == "skipped"
}

func normalizeTestingMutationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrTestingMutationNotFound
	}
	return err
}

var _ task.TestingMutationStore = (*TestingMutationRepository)(nil)
