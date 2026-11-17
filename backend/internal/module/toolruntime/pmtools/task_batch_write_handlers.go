package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	TaskBatchCreate         = "task.batch_create"
	TaskChildrenBatchCreate = "task.children.batch_create"
	TaskBatchDelete         = "task.batch_delete"
	TaskBatchUpdate         = "task.batch_update"
	TaskBatchStatusUpdate   = "task.status.batch_update"
)

type TaskBatchMutationService interface {
	ApplyTaskBatchMutation(context.Context, task.BatchMutation) (task.BatchMutationResult, error)
}

type taskBatchWriteHandler struct {
	mutations  TaskBatchMutationService
	operation  task.BatchMutationOperation
	statusOnly bool
}

func TaskBatchWriteHandlerNames() []string {
	return []string{TaskBatchCreate, TaskBatchDelete, TaskBatchStatusUpdate, TaskBatchUpdate, TaskChildrenBatchCreate}
}

func NewTaskBatchWriteHandlers(mutations TaskBatchMutationService) (map[string]toolruntime.ToolHandler, error) {
	if mutations == nil {
		return nil, fmt.Errorf("task batch mutation service is required")
	}
	return map[string]toolruntime.ToolHandler{
		TaskBatchCreate:         taskBatchWriteHandler{mutations: mutations, operation: task.BatchCreate},
		TaskChildrenBatchCreate: taskBatchWriteHandler{mutations: mutations, operation: task.ChildrenBatchCreate},
		TaskBatchDelete:         taskBatchWriteHandler{mutations: mutations, operation: task.BatchDelete},
		TaskBatchUpdate:         taskBatchWriteHandler{mutations: mutations, operation: task.BatchUpdate},
		TaskBatchStatusUpdate:   taskBatchWriteHandler{mutations: mutations, operation: task.BatchUpdate, statusOnly: true},
	}, nil
}

func (handler taskBatchWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler taskBatchWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.mutations == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 ||
		strings.TrimSpace(call.ChangeSetID) == "" || strings.TrimSpace(call.ApplyKey) == "" {
		return nil, taskBatchPublicError(task.ErrBatchMutationInvalid)
	}
	command, err := decodeTaskBatchMutation(call.Input, handler.operation)
	if err != nil {
		return nil, taskBatchPublicError(err)
	}
	if handler.statusOnly && !batchPatchOnlyStatus(command.Updates) {
		return nil, taskBatchPublicError(task.ErrBatchMutationInvalid)
	}
	digest := sha256.Sum256(call.Input)
	command.WorkspaceID, command.PrincipalID = call.Authorization.WorkspaceID, call.Authorization.PrincipalID
	command.ChangeSetID, command.ApplyKey, command.InputDigest = call.ChangeSetID, call.ApplyKey, hex.EncodeToString(digest[:])
	result, err := handler.mutations.ApplyTaskBatchMutation(ctx, command)
	if err != nil {
		if task.IsBatchMutationBusinessError(err) {
			return nil, taskBatchPublicError(err)
		}
		return nil, toolruntime.RetryableApplyError(err)
	}
	return marshalTaskBatchMutationResult(handler.operation, result)
}

func batchPatchOnlyStatus(patch task.BatchTaskPatch) bool {
	return patch.Status.Set && patch.Status.Value != nil &&
		!patch.TaskClientID.Set && !patch.Title.Set && !patch.Description.Set && !patch.Priority.Set && !patch.AssigneeID.Set &&
		!patch.ReporterUserID.Set && !patch.EstimatedMinutes.Set && !patch.StartsAt.Set && !patch.DueAt.Set
}

func marshalTaskBatchMutationResult(operation task.BatchMutationOperation, result task.BatchMutationResult) (json.RawMessage, error) {
	switch operation {
	case task.BatchCreate:
		return json.Marshal(struct {
			Created      []task.BatchCreatedTask `json:"created"`
			CreatedCount int                     `json:"created_count"`
			Atomic       bool                    `json:"atomic"`
		}{Created: result.Created, CreatedCount: result.CreatedCount, Atomic: result.Atomic})
	case task.ChildrenBatchCreate:
		return json.Marshal(struct {
			Created      []task.BatchCreatedTask `json:"created"`
			CreatedCount int                     `json:"created_count"`
			ParentID     uint                    `json:"parent_id"`
			Atomic       bool                    `json:"atomic"`
		}{Created: result.Created, CreatedCount: result.CreatedCount, ParentID: result.ParentID, Atomic: result.Atomic})
	case task.BatchDelete:
		return json.Marshal(struct {
			DeletedIDs   []uint `json:"deleted_ids"`
			DeletedCount int    `json:"deleted_count"`
			AffectedIDs  []uint `json:"affected_ids"`
			Atomic       bool   `json:"atomic"`
		}{
			DeletedIDs: taskBatchIDList(result.DeletedIDs), DeletedCount: result.DeletedCount,
			AffectedIDs: taskBatchIDList(result.AffectedIDs), Atomic: result.Atomic,
		})
	case task.BatchUpdate:
		return json.Marshal(struct {
			UpdatedIDs                 []uint `json:"updated_ids"`
			UpdatedCount               int    `json:"updated_count"`
			AffectedIDs                []uint `json:"affected_ids"`
			AutoCompletedAncestorIDs   []uint `json:"auto_completed_ancestor_ids"`
			DuePropagatedDescendantIDs []uint `json:"due_propagated_descendant_ids"`
			Atomic                     bool   `json:"atomic"`
		}{
			UpdatedIDs: taskBatchIDList(result.UpdatedIDs), UpdatedCount: result.UpdatedCount,
			AffectedIDs:                taskBatchIDList(result.AffectedIDs),
			AutoCompletedAncestorIDs:   taskBatchIDList(result.AutoCompletedAncestorIDs),
			DuePropagatedDescendantIDs: taskBatchIDList(result.DuePropagatedDescendantIDs), Atomic: result.Atomic,
		})
	default:
		return nil, task.ErrBatchMutationInvalid
	}
}

func taskBatchIDList(values []uint) []uint {
	if values == nil {
		return []uint{}
	}
	return values
}

type taskBatchWriteInput struct {
	ClientID         string                `json:"client_id"`
	RequirementID    uint                  `json:"requirement_id"`
	TaskClientID     *uint                 `json:"task_client_id"`
	Title            string                `json:"title"`
	Description      *string               `json:"description"`
	Status           *string               `json:"status"`
	Priority         *string               `json:"priority"`
	AssigneeID       *uint                 `json:"assignee_id"`
	ReporterUserID   *uint                 `json:"reporter_user_id"`
	EstimatedMinutes *int                  `json:"estimated_minutes"`
	StartsAt         *string               `json:"starts_at"`
	DueAt            *string               `json:"due_at"`
	Children         []taskBatchWriteInput `json:"children"`
}

type taskBatchWriteSelector struct {
	IDs           []uint   `json:"ids"`
	TaskRefs      []string `json:"task_refs"`
	ProjectSetID  uint     `json:"project_set_id"`
	PlanUnitID    uint     `json:"project_new_id"`
	RequirementID uint     `json:"requirement_id"`
}

func (input taskBatchWriteSelector) selector() task.BatchTaskSelector {
	return task.BatchTaskSelector{
		IDs: input.IDs, TaskRefs: input.TaskRefs, ProjectSetID: input.ProjectSetID,
		PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID,
	}
}

type taskBatchWritePatch struct {
	TaskClientID     optionalJSON[uint]   `json:"task_client_id"`
	Title            optionalJSON[string] `json:"title"`
	Description      optionalJSON[string] `json:"description"`
	Status           optionalJSON[string] `json:"status"`
	Priority         optionalJSON[string] `json:"priority"`
	AssigneeID       optionalJSON[uint]   `json:"assignee_id"`
	ReporterUserID   optionalJSON[uint]   `json:"reporter_user_id"`
	EstimatedMinutes optionalJSON[int]    `json:"estimated_minutes"`
	StartsAt         optionalJSON[string] `json:"starts_at"`
	DueAt            optionalJSON[string] `json:"due_at"`
}

func decodeTaskBatchMutation(raw json.RawMessage, operation task.BatchMutationOperation) (task.BatchMutation, error) {
	command := task.BatchMutation{Operation: operation}
	switch operation {
	case task.BatchCreate:
		var input struct {
			Tasks          []taskBatchWriteInput `json:"tasks"`
			IdempotencyKey string                `json:"idempotency_key"`
		}
		if decodeInput(raw, &input) != nil {
			return command, task.ErrBatchMutationInvalid
		}
		command.Tasks = make([]task.BatchTaskInput, 0, len(input.Tasks))
		for _, value := range input.Tasks {
			decoded, err := value.taskInput()
			if err != nil {
				return command, err
			}
			command.Tasks = append(command.Tasks, decoded)
		}
	case task.ChildrenBatchCreate:
		var input struct {
			ID             uint                  `json:"id"`
			Children       []taskBatchWriteInput `json:"children"`
			IdempotencyKey string                `json:"idempotency_key"`
		}
		if decodeInput(raw, &input) != nil {
			return command, task.ErrBatchMutationInvalid
		}
		command.ParentID, command.Tasks = input.ID, make([]task.BatchTaskInput, 0, len(input.Children))
		for _, value := range input.Children {
			decoded, err := value.taskInput()
			if err != nil {
				return command, err
			}
			command.Tasks = append(command.Tasks, decoded)
		}
	case task.BatchDelete:
		var input struct {
			taskBatchWriteSelector
			Cascade        bool   `json:"cascade"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if decodeInput(raw, &input) != nil {
			return command, task.ErrBatchMutationInvalid
		}
		command.Selector, command.Cascade = input.selector(), input.Cascade
	case task.BatchUpdate:
		var input struct {
			taskBatchWriteSelector
			Updates        taskBatchWritePatch `json:"updates"`
			IdempotencyKey string              `json:"idempotency_key"`
		}
		if decodeInput(raw, &input) != nil {
			return command, task.ErrBatchMutationInvalid
		}
		patch, err := input.Updates.patch()
		if err != nil {
			return command, err
		}
		command.Selector, command.Updates = input.selector(), patch
	default:
		return command, task.ErrBatchMutationInvalid
	}
	return command, nil
}

func (input taskBatchWriteInput) taskInput() (task.BatchTaskInput, error) {
	startsAt, err := parseTaskBatchTime(input.StartsAt)
	if err != nil {
		return task.BatchTaskInput{}, task.ErrBatchMutationInvalid
	}
	dueAt, err := parseTaskBatchTime(input.DueAt)
	if err != nil {
		return task.BatchTaskInput{}, task.ErrBatchMutationInvalid
	}
	result := task.BatchTaskInput{
		ClientID: input.ClientID, RequirementID: input.RequirementID, TaskClientID: input.TaskClientID, Title: input.Title,
		Description: input.Description, Status: input.Status, Priority: input.Priority,
		AssigneeID: input.AssigneeID, ReporterUserID: input.ReporterUserID, EstimatedMinutes: input.EstimatedMinutes,
		StartsAt: startsAt, DueAt: dueAt, Children: make([]task.BatchTaskInput, 0, len(input.Children)),
	}
	for _, value := range input.Children {
		child, childErr := value.taskInput()
		if childErr != nil {
			return task.BatchTaskInput{}, childErr
		}
		result.Children = append(result.Children, child)
	}
	return result, nil
}

func (input taskBatchWritePatch) patch() (task.BatchTaskPatch, error) {
	result := task.BatchTaskPatch{
		TaskClientID:     task.Optional[uint]{Set: input.TaskClientID.Set, Value: input.TaskClientID.Value},
		Title:            task.Optional[string]{Set: input.Title.Set, Value: input.Title.Value},
		Description:      task.Optional[string]{Set: input.Description.Set, Value: input.Description.Value},
		Status:           task.Optional[string]{Set: input.Status.Set, Value: input.Status.Value},
		Priority:         task.Optional[string]{Set: input.Priority.Set, Value: input.Priority.Value},
		AssigneeID:       task.Optional[uint]{Set: input.AssigneeID.Set, Value: input.AssigneeID.Value},
		ReporterUserID:   task.Optional[uint]{Set: input.ReporterUserID.Set, Value: input.ReporterUserID.Value},
		EstimatedMinutes: task.Optional[int]{Set: input.EstimatedMinutes.Set, Value: input.EstimatedMinutes.Value},
	}
	var err error
	result.StartsAt.Set = input.StartsAt.Set
	result.StartsAt.Value, err = parseTaskBatchTime(input.StartsAt.Value)
	if err != nil {
		return task.BatchTaskPatch{}, task.ErrBatchMutationInvalid
	}
	result.DueAt.Set = input.DueAt.Set
	result.DueAt.Value, err = parseTaskBatchTime(input.DueAt.Value)
	if err != nil {
		return task.BatchTaskPatch{}, task.ErrBatchMutationInvalid
	}
	return result, nil
}

func parseTaskBatchTime(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

var _ toolruntime.ToolHandler = taskBatchWriteHandler{}
var _ toolruntime.IdempotentMutationHandler = taskBatchWriteHandler{}
