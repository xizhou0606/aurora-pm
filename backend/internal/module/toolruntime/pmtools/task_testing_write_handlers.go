package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	TaskTestingDefinitionUpdate   = "task.testing.definition.update"
	TaskTestingExecutionCreate    = "task.testing.execution.create"
	TaskTestingExecutionRunUpdate = "task.testing.execution_run.update"
)

type TaskTestingMutationService interface {
	ApplyTestingMutation(context.Context, task.TestingMutation) (task.TestingMutationResult, error)
}

type taskTestingWriteHandler struct {
	tasks     TaskQueries
	mutations TaskTestingMutationService
	operation task.TestingMutationOperation
}

func TaskTestingWriteHandlerNames() []string {
	return []string{TaskTestingDefinitionUpdate, TaskTestingExecutionCreate, TaskTestingExecutionRunUpdate}
}

func NewTaskTestingWriteHandlers(tasks TaskQueries, mutations TaskTestingMutationService) (map[string]toolruntime.ToolHandler, error) {
	if tasks == nil || mutations == nil {
		return nil, fmt.Errorf("task testing mutation dependencies are required")
	}
	return map[string]toolruntime.ToolHandler{
		TaskTestingDefinitionUpdate:   taskTestingWriteHandler{tasks: tasks, mutations: mutations, operation: task.TestingDefinitionUpdate},
		TaskTestingExecutionCreate:    taskTestingWriteHandler{tasks: tasks, mutations: mutations, operation: task.TestingExecutionCreate},
		TaskTestingExecutionRunUpdate: taskTestingWriteHandler{tasks: tasks, mutations: mutations, operation: task.TestingExecutionRunUpdate},
	}, nil
}

func (handler taskTestingWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

type taskTestingDefinitionUpdateInput struct {
	taskGetInput
	Version             uint            `json:"version"`
	Purpose             string          `json:"purpose"`
	Preconditions       string          `json:"preconditions"`
	EnvironmentGuidance string          `json:"environment_guidance"`
	Cases               []task.TestCase `json:"cases"`
	IdempotencyKey      string          `json:"idempotency_key"`
}

type taskTestingExecutionCreateInput struct {
	taskGetInput
	DefinitionVersion uint     `json:"definition_version"`
	Environment       string   `json:"environment"`
	BuildVersion      string   `json:"build_version"`
	ExecutorUserID    *uint    `json:"executor_user_id"`
	Summary           string   `json:"summary"`
	CaseKeys          []string `json:"case_keys"`
	IdempotencyKey    string   `json:"idempotency_key"`
}

type taskTestingExecutionRunUpdateInput struct {
	taskGetInput
	RunID          uint    `json:"run_id"`
	Version        uint    `json:"version"`
	Status         string  `json:"status"`
	ActualResult   string  `json:"actual_result"`
	FailureReason  string  `json:"failure_reason"`
	ExecutedAt     *string `json:"executed_at"`
	IdempotencyKey string  `json:"idempotency_key"`
}

func (handler taskTestingWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.tasks == nil || handler.mutations == nil || call.Authorization.WorkspaceID == 0 ||
		call.Authorization.PrincipalID == 0 || call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, task.ErrTestingMutationInvalid
	}
	command := task.TestingMutation{
		Operation: handler.operation, WorkspaceID: call.Authorization.WorkspaceID, PrincipalID: call.Authorization.PrincipalID,
		ApplyKey: call.ApplyKey, ChangeSetID: call.ChangeSetID,
	}
	digest := sha256.Sum256(call.Input)
	command.InputDigest = hex.EncodeToString(digest[:])
	var lookup task.Lookup
	switch handler.operation {
	case task.TestingDefinitionUpdate:
		var input taskTestingDefinitionUpdateInput
		if err := decodeInput(call.Input, &input); err != nil {
			return nil, task.ErrTestingMutationInvalid
		}
		lookup = input.taskGetInput.lookup()
		command.Definition = task.TestingDefinitionValues{
			Version: input.Version, Purpose: input.Purpose, Preconditions: input.Preconditions,
			EnvironmentGuidance: input.EnvironmentGuidance, Cases: input.Cases,
		}
	case task.TestingExecutionCreate:
		var input taskTestingExecutionCreateInput
		if err := decodeInput(call.Input, &input); err != nil {
			return nil, task.ErrTestingMutationInvalid
		}
		lookup = input.taskGetInput.lookup()
		executorID := uint(0)
		if input.ExecutorUserID != nil {
			executorID = *input.ExecutorUserID
		}
		command.Execution = task.TestingExecutionValues{
			DefinitionVersion: input.DefinitionVersion, Environment: input.Environment, BuildVersion: input.BuildVersion,
			ExecutorUserID: executorID, Summary: input.Summary, CaseKeys: input.CaseKeys,
		}
	case task.TestingExecutionRunUpdate:
		var input taskTestingExecutionRunUpdateInput
		fields, err := decodeTaskTestingObject(call.Input, &input)
		if err != nil {
			return nil, task.ErrTestingMutationInvalid
		}
		lookup = input.taskGetInput.lookup()
		values := task.TestingRunValues{
			RunID: input.RunID, Version: input.Version, Status: input.Status,
			ActualResult: input.ActualResult, FailureReason: input.FailureReason, Fields: map[string]bool{},
		}
		for _, field := range []string{"status", "actual_result", "failure_reason", "executed_at"} {
			if _, exists := fields[field]; exists {
				values.Fields[field] = true
			}
		}
		if values.Fields["executed_at"] && input.ExecutedAt == nil {
			return nil, task.ErrTestingMutationInvalid
		}
		if input.ExecutedAt != nil {
			value := strings.TrimSpace(*input.ExecutedAt)
			if value != "" {
				parsed, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return nil, task.ErrTestingMutationInvalid
				}
				values.ExecutedAt = &parsed
			}
		}
		command.Run = values
	default:
		return nil, task.ErrTestingMutationInvalid
	}
	taskItem, err := handler.tasks.GetTask(ctx, call.Authorization.WorkspaceID, lookup)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) || errors.Is(err, task.ErrTaskAmbiguous) || errors.Is(err, task.ErrReadQueryInvalid) {
			return nil, task.ErrTestingMutationNotFound
		}
		return nil, toolruntime.RetryableApplyError(err)
	}
	command.TaskID = taskItem.ID
	command.TaskScope = task.MutationTaskScope{
		TeamID: taskItem.TeamID, ProjectSetID: taskItem.ProjectSetID,
		PlanUnitID: taskItem.PlanUnitID, RequirementID: taskItem.RequirementID,
	}
	result, err := handler.mutations.ApplyTestingMutation(ctx, command)
	if err != nil {
		if task.IsTestingMutationBusinessError(err) {
			return nil, err
		}
		return nil, toolruntime.RetryableApplyError(err)
	}
	switch handler.operation {
	case task.TestingDefinitionUpdate:
		if result.Definition == nil {
			return nil, task.ErrTestingMutationConflict
		}
		return json.Marshal(result.Definition)
	case task.TestingExecutionCreate:
		if result.Execution == nil || len(result.Runs) == 0 {
			return nil, task.ErrTestingMutationConflict
		}
		return json.Marshal(struct {
			Execution task.TestExecutionSnapshot      `json:"execution"`
			Runs      []task.TestExecutionRunSnapshot `json:"runs"`
		}{*result.Execution, result.Runs})
	case task.TestingExecutionRunUpdate:
		if result.Run == nil || result.Execution == nil {
			return nil, task.ErrTestingMutationConflict
		}
		return json.Marshal(struct {
			Run       task.TestExecutionRunSnapshot `json:"run"`
			Execution task.TestExecutionSnapshot    `json:"execution"`
		}{*result.Run, *result.Execution})
	}
	return nil, task.ErrTestingMutationInvalid
}

func decodeTaskTestingObject(raw json.RawMessage, destination any) (map[string]json.RawMessage, error) {
	if err := decodeInput(raw, destination); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, task.ErrTestingMutationInvalid
	}
	return fields, nil
}

var _ toolruntime.ToolHandler = taskTestingWriteHandler{}
var _ toolruntime.IdempotentMutationHandler = taskTestingWriteHandler{}
