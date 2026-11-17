package pmtools

import (
	"bytes"
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
	TaskCreate       = "task.create"
	TaskUpdate       = "task.update"
	TaskStatusUpdate = "task.status.update"
	TaskDelete       = "task.delete"
	TaskClone        = "task.clone"
)

type TaskCoreMutationService interface {
	ApplyCoreMutation(context.Context, task.CoreMutation) (task.CoreMutationResult, error)
}

type taskCoreWriteHandler struct {
	tasks     TaskQueries
	mutations TaskCoreMutationService
	deletions task.TaskDeletionExecutor
	operation string
}

func TaskCoreWriteHandlerNames() []string {
	return []string{TaskClone, TaskCreate, TaskDelete, TaskStatusUpdate, TaskUpdate}
}

func NewTaskCoreWriteHandlers(tasks TaskQueries, mutations TaskCoreMutationService, deletions task.TaskDeletionExecutor) (map[string]toolruntime.ToolHandler, error) {
	if tasks == nil || mutations == nil || deletions == nil {
		return nil, fmt.Errorf("task core mutation dependencies are required")
	}
	return map[string]toolruntime.ToolHandler{
		TaskCreate:       taskCoreWriteHandler{tasks: tasks, mutations: mutations, deletions: deletions, operation: TaskCreate},
		TaskUpdate:       taskCoreWriteHandler{tasks: tasks, mutations: mutations, deletions: deletions, operation: TaskUpdate},
		TaskStatusUpdate: taskCoreWriteHandler{tasks: tasks, mutations: mutations, deletions: deletions, operation: TaskStatusUpdate},
		TaskDelete:       taskCoreWriteHandler{tasks: tasks, mutations: mutations, deletions: deletions, operation: TaskDelete},
		TaskClone:        taskCoreWriteHandler{tasks: tasks, mutations: mutations, deletions: deletions, operation: TaskClone},
	}, nil
}

func (handler taskCoreWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler taskCoreWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.tasks == nil || handler.mutations == nil || handler.deletions == nil || call.Authorization.WorkspaceID == 0 ||
		call.Authorization.PrincipalID == 0 || call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, task.ErrCoreMutationInvalid
	}
	digest := sha256.Sum256(call.Input)
	inputDigest := hex.EncodeToString(digest[:])
	if handler.operation == TaskDelete {
		var input struct {
			ID             uint   `json:"id"`
			Cascade        bool   `json:"cascade"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if decodeInput(call.Input, &input) != nil || input.ID == 0 {
			return nil, task.ErrTaskDeletionInvalid
		}
		result, err := handler.deletions.DeleteTasks(ctx, task.TaskDeletionCommand{
			WorkspaceID: call.Authorization.WorkspaceID, PrincipalID: call.Authorization.PrincipalID,
			ApplyKey: call.ApplyKey, ChangeSetID: call.ChangeSetID, InputDigest: inputDigest,
			RootIDs: []uint{input.ID}, Cascade: input.Cascade,
		})
		if err != nil {
			if task.IsTaskDeletionBusinessError(err) {
				return nil, err
			}
			return nil, toolruntime.RetryableApplyError(err)
		}
		return json.Marshal(struct {
			Deleted      bool `json:"deleted"`
			DeletedCount int  `json:"deleted_count"`
		}{Deleted: result.Deleted, DeletedCount: result.DeletedCount})
	}
	command, err := handler.decodeCoreCommand(ctx, call, inputDigest)
	if err != nil {
		return nil, err
	}
	result, err := handler.mutations.ApplyCoreMutation(ctx, command)
	if err != nil {
		if task.IsCoreMutationBusinessError(err) {
			return nil, err
		}
		return nil, toolruntime.RetryableApplyError(err)
	}
	if result.Task == nil {
		return nil, task.ErrCoreMutationConflict
	}
	if handler.operation != TaskClone {
		return json.Marshal(result.Task)
	}
	return json.Marshal(struct {
		Task       task.Snapshot   `json:"task"`
		ClonedFrom *task.Reference `json:"cloned_from"`
	}{Task: *result.Task, ClonedFrom: result.ClonedFrom})
}

func (handler taskCoreWriteHandler) decodeCoreCommand(ctx context.Context, call toolruntime.MutationCall, inputDigest string) (task.CoreMutation, error) {
	command := task.CoreMutation{WorkspaceID: call.Authorization.WorkspaceID, PrincipalID: call.Authorization.PrincipalID, ApplyKey: call.ApplyKey, ChangeSetID: call.ChangeSetID, InputDigest: inputDigest}
	switch handler.operation {
	case TaskCreate, TaskUpdate:
		var input taskCoreWriteInput
		if decodeInput(call.Input, &input) != nil {
			return command, task.ErrCoreMutationInvalid
		}
		values, err := input.values()
		if err != nil {
			return command, err
		}
		if input.ReferenceTaskID > 0 || strings.TrimSpace(input.ReferenceTaskRef) != "" {
			reference, lookupErr := handler.tasks.GetTask(ctx, call.Authorization.WorkspaceID, task.Lookup{ID: input.ReferenceTaskID, Reference: input.ReferenceTaskRef})
			if lookupErr != nil {
				return command, mapCoreLookupError(lookupErr)
			}
			if !values.Fields["requirement_id"] {
				values.RequirementID = reference.RequirementID
				values.Fields["requirement_id"] = true
			}
		}
		if handler.operation == TaskCreate {
			command.Operation = task.CoreCreate
			if values.ParentID != nil && !values.Fields["kind"] {
				values.Kind = task.WorkTaskKindSubtask
			}
			values.Fields = nil
		} else {
			if input.ID == 0 {
				return command, task.ErrCoreMutationInvalid
			}
			current, lookupErr := handler.tasks.GetTask(ctx, call.Authorization.WorkspaceID, task.Lookup{ID: input.ID})
			if lookupErr != nil {
				return command, mapCoreLookupError(lookupErr)
			}
			command.Operation, command.ID, command.ExpectedUpdatedAt = task.CoreUpdate, current.ID, current.UpdatedAt
		}
		command.Values = values
	case TaskStatusUpdate:
		var input struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		}
		if decodeInput(call.Input, &input) != nil || input.ID == 0 || strings.TrimSpace(input.Status) == "" {
			return command, task.ErrCoreMutationInvalid
		}
		current, lookupErr := handler.tasks.GetTask(ctx, call.Authorization.WorkspaceID, task.Lookup{ID: input.ID})
		if lookupErr != nil {
			return command, mapCoreLookupError(lookupErr)
		}
		command.Operation, command.ID, command.ExpectedUpdatedAt = task.CoreUpdate, current.ID, current.UpdatedAt
		command.Values = task.CoreMutationValues{
			Status: strings.TrimSpace(input.Status), Fields: map[string]bool{"status": true},
		}
	case TaskClone:
		var input struct {
			ID                  uint                 `json:"id"`
			TitleSuffix         optionalJSON[string] `json:"title_suffix"`
			TargetRequirementID uint                 `json:"target_requirement_id"`
			IdempotencyKey      string               `json:"idempotency_key"`
		}
		if decodeInput(call.Input, &input) != nil || input.ID == 0 || (input.TitleSuffix.Set && input.TitleSuffix.Value == nil) {
			return command, task.ErrCoreMutationInvalid
		}
		current, lookupErr := handler.tasks.GetTask(ctx, call.Authorization.WorkspaceID, task.Lookup{ID: input.ID})
		if lookupErr != nil {
			return command, mapCoreLookupError(lookupErr)
		}
		command.Operation, command.ID, command.ExpectedUpdatedAt = task.CoreClone, current.ID, current.UpdatedAt
		command.TitleSuffixSet, command.TargetRequirementID = input.TitleSuffix.Set, input.TargetRequirementID
		if input.TitleSuffix.Value != nil {
			command.TitleSuffix = *input.TitleSuffix.Value
		}
	default:
		return command, task.ErrCoreMutationInvalid
	}
	return command, nil
}

type taskCoreWriteInput struct {
	ID               uint                 `json:"id"`
	ReferenceTaskRef string               `json:"reference_task_ref"`
	ReferenceTaskID  uint                 `json:"reference_task_id"`
	RequirementID    optionalJSON[uint]   `json:"requirement_id"`
	TaskClientID     optionalJSON[uint]   `json:"task_client_id"`
	Kind             optionalJSON[string] `json:"kind"`
	Title            optionalJSON[string] `json:"title"`
	Description      optionalJSON[string] `json:"description"`
	Status           optionalJSON[string] `json:"status"`
	Priority         optionalJSON[string] `json:"priority"`
	AssigneeID       optionalJSON[uint]   `json:"assignee_id"`
	ReporterUserID   optionalJSON[uint]   `json:"reporter_user_id"`
	ParentID         optionalJSON[uint]   `json:"parent_id"`
	EstimatedMinutes optionalJSON[int]    `json:"estimated_minutes"`
	StartsAt         optionalJSON[string] `json:"starts_at"`
	DueAt            optionalJSON[string] `json:"due_at"`
	IdempotencyKey   string               `json:"idempotency_key"`
}

func (input taskCoreWriteInput) values() (task.CoreMutationValues, error) {
	values := task.CoreMutationValues{Fields: map[string]bool{}}
	if (input.RequirementID.Set && input.RequirementID.Value == nil) ||
		(input.Kind.Set && input.Kind.Value == nil) ||
		(input.Title.Set && input.Title.Value == nil) ||
		(input.Description.Set && input.Description.Value == nil) ||
		(input.Status.Set && input.Status.Value == nil) ||
		(input.Priority.Set && input.Priority.Value == nil) {
		return values, task.ErrCoreMutationInvalid
	}
	if input.RequirementID.Set {
		values.Fields["requirement_id"] = true
		if input.RequirementID.Value != nil {
			values.RequirementID = *input.RequirementID.Value
		}
	}
	if input.TaskClientID.Set {
		values.Fields["task_client_id"] = true
		values.TaskClientID = input.TaskClientID.Value
	}
	if input.Kind.Set {
		values.Fields["kind"] = true
		if input.Kind.Value != nil {
			values.Kind = *input.Kind.Value
		}
	}
	if input.Title.Set {
		values.Fields["title"] = true
		if input.Title.Value != nil {
			values.Title = *input.Title.Value
		}
	}
	if input.Description.Set {
		values.Fields["description"] = true
		if input.Description.Value != nil {
			values.Description = *input.Description.Value
		}
	}
	if input.Status.Set {
		values.Fields["status"] = true
		if input.Status.Value != nil {
			values.Status = *input.Status.Value
		}
	}
	if input.Priority.Set {
		values.Fields["priority"] = true
		if input.Priority.Value != nil {
			values.Priority = *input.Priority.Value
		}
	}
	if input.AssigneeID.Set {
		values.Fields["assignee_id"] = true
		values.AssigneeID = input.AssigneeID.Value
	}
	if input.ReporterUserID.Set {
		values.Fields["reporter_user_id"] = true
		values.ReporterUserID = input.ReporterUserID.Value
	}
	if input.ParentID.Set {
		values.Fields["parent_id"] = true
		values.ParentID = input.ParentID.Value
	}
	if input.EstimatedMinutes.Set {
		values.Fields["estimated_minutes"] = true
		values.EstimatedMinutes = input.EstimatedMinutes.Value
	}
	var err error
	if input.StartsAt.Set {
		values.Fields["starts_at"] = true
		values.StartsAt, err = parseCoreOptionalTime(input.StartsAt.Value)
		if err != nil {
			return values, task.ErrCoreMutationInvalid
		}
	}
	if input.DueAt.Set {
		values.Fields["due_at"] = true
		values.DueAt, err = parseCoreOptionalTime(input.DueAt.Value)
		if err != nil {
			return values, task.ErrCoreMutationInvalid
		}
	}
	return values, nil
}

type optionalJSON[T any] struct {
	Set   bool
	Value *T
}

func (value *optionalJSON[T]) UnmarshalJSON(raw []byte) error {
	value.Set = true
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		value.Value = nil
		return nil
	}
	var decoded T
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	value.Value = &decoded
	return nil
}

func parseCoreOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, task.ErrCoreMutationInvalid
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

func mapCoreLookupError(err error) error {
	if errors.Is(err, task.ErrTaskNotFound) || errors.Is(err, task.ErrTaskAmbiguous) || errors.Is(err, task.ErrReadQueryInvalid) {
		return task.ErrCoreMutationNotFound
	}
	return toolruntime.RetryableApplyError(err)
}

var _ toolruntime.ToolHandler = taskCoreWriteHandler{}
var _ toolruntime.IdempotentMutationHandler = taskCoreWriteHandler{}
