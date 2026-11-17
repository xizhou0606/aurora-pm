package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	TaskCommentCreate    = "task.comment.create"
	TaskCommentDelete    = "task.comment.delete"
	TaskAttachmentDelete = "task.attachment.delete"
	TaskWatchUpdate      = "task.watch.update"
)

type TaskCollaborationMutationService interface {
	ApplyCollaborationMutation(context.Context, task.CollaborationMutation) (task.CollaborationMutationResult, error)
}

type taskCollaborationWriteHandler struct {
	tasks     TaskQueries
	mutations TaskCollaborationMutationService
	operation task.CollaborationMutationOperation
}

func TaskCollaborationWriteHandlerNames() []string {
	return []string{TaskAttachmentDelete, TaskCommentCreate, TaskCommentDelete, TaskWatchUpdate}
}

func NewTaskCollaborationWriteHandlers(tasks TaskQueries, mutations TaskCollaborationMutationService) (map[string]toolruntime.ToolHandler, error) {
	if tasks == nil || mutations == nil {
		return nil, fmt.Errorf("task collaboration mutation dependencies are required")
	}
	return map[string]toolruntime.ToolHandler{
		TaskCommentCreate:    taskCollaborationWriteHandler{tasks: tasks, mutations: mutations, operation: task.CommentCreate},
		TaskCommentDelete:    taskCollaborationWriteHandler{tasks: tasks, mutations: mutations, operation: task.CommentDelete},
		TaskAttachmentDelete: taskCollaborationWriteHandler{tasks: tasks, mutations: mutations, operation: task.AttachmentDelete},
		TaskWatchUpdate:      taskCollaborationWriteHandler{tasks: tasks, mutations: mutations, operation: task.WatchUpdate},
	}, nil
}

func (handler taskCollaborationWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler taskCollaborationWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.tasks == nil || handler.mutations == nil || call.Authorization.WorkspaceID == 0 ||
		call.Authorization.PrincipalID == 0 || call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, task.ErrCollaborationMutationInvalid
	}
	command := task.CollaborationMutation{
		Operation: handler.operation, WorkspaceID: call.Authorization.WorkspaceID, PrincipalID: call.Authorization.PrincipalID,
		ApplyKey: call.ApplyKey, ChangeSetID: call.ChangeSetID,
	}
	digest := sha256.Sum256(call.Input)
	command.InputDigest = hex.EncodeToString(digest[:])
	var taskID uint
	switch handler.operation {
	case task.CommentCreate:
		var input struct {
			ID             uint   `json:"id"`
			Body           string `json:"body"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := decodeInput(call.Input, &input); err != nil {
			return nil, task.ErrCollaborationMutationInvalid
		}
		taskID, command.Body = input.ID, input.Body
	case task.CommentDelete:
		var input struct {
			ID             uint   `json:"id"`
			CommentID      uint   `json:"cid"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := decodeInput(call.Input, &input); err != nil {
			return nil, task.ErrCollaborationMutationInvalid
		}
		taskID, command.CommentID = input.ID, input.CommentID
	case task.AttachmentDelete:
		var input struct {
			ID             uint   `json:"id"`
			AttachmentID   uint   `json:"aid"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := decodeInput(call.Input, &input); err != nil {
			return nil, task.ErrCollaborationMutationInvalid
		}
		taskID, command.AttachmentID = input.ID, input.AttachmentID
	case task.WatchUpdate:
		var input struct {
			ID             uint   `json:"id"`
			Watch          *bool  `json:"watch"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := decodeInput(call.Input, &input); err != nil || input.Watch == nil {
			return nil, task.ErrCollaborationMutationInvalid
		}
		taskID, command.Watch = input.ID, *input.Watch
	default:
		return nil, task.ErrCollaborationMutationInvalid
	}
	if taskID == 0 {
		return nil, task.ErrCollaborationMutationInvalid
	}
	taskItem, err := handler.tasks.GetTask(ctx, call.Authorization.WorkspaceID, task.Lookup{ID: taskID})
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) || errors.Is(err, task.ErrTaskAmbiguous) || errors.Is(err, task.ErrReadQueryInvalid) {
			return nil, task.ErrCollaborationMutationNotFound
		}
		return nil, toolruntime.RetryableApplyError(err)
	}
	command.TaskID = taskItem.ID
	command.TaskScope = task.MutationTaskScope{
		TeamID: taskItem.TeamID, ProjectSetID: taskItem.ProjectSetID,
		PlanUnitID: taskItem.PlanUnitID, RequirementID: taskItem.RequirementID,
	}
	result, err := handler.mutations.ApplyCollaborationMutation(ctx, command)
	if err != nil {
		if task.IsCollaborationMutationBusinessError(err) {
			return nil, err
		}
		return nil, toolruntime.RetryableApplyError(err)
	}
	switch handler.operation {
	case task.CommentCreate:
		if result.Comment == nil {
			return nil, task.ErrCollaborationMutationConflict
		}
		return json.Marshal(result.Comment)
	case task.CommentDelete, task.AttachmentDelete:
		if !result.Deleted || result.DeletedID == 0 {
			return nil, task.ErrCollaborationMutationConflict
		}
		return json.Marshal(struct {
			ID      uint `json:"id"`
			Deleted bool `json:"deleted"`
		}{ID: result.DeletedID, Deleted: true})
	case task.WatchUpdate:
		if result.Watched == nil || result.WorkTaskID == 0 {
			return nil, task.ErrCollaborationMutationConflict
		}
		return json.Marshal(struct {
			Watched    bool `json:"watched"`
			WorkTaskID uint `json:"work_task_id"`
		}{Watched: *result.Watched, WorkTaskID: result.WorkTaskID})
	}
	return nil, task.ErrCollaborationMutationInvalid
}

var _ toolruntime.ToolHandler = taskCollaborationWriteHandler{}
var _ toolruntime.IdempotentMutationHandler = taskCollaborationWriteHandler{}
