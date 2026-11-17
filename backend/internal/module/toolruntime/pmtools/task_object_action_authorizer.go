package pmtools

import (
	"context"
	"encoding/json"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const taskCreatorObjectPolicy = "task.creator"

type TaskCreatorPolicy interface {
	AllCreatedBy(context.Context, uint, uint, []uint) (bool, error)
}

type TaskObjectPrincipalResolver interface {
	FactUserID(context.Context, uint) (uint, error)
}

type TaskObjectActionAuthorizer struct {
	tasks    TaskQueries
	creators TaskCreatorPolicy
	users    TaskObjectPrincipalResolver
}

func NewTaskObjectActionAuthorizer(
	tasks TaskQueries,
	creators TaskCreatorPolicy,
	users TaskObjectPrincipalResolver,
) (*TaskObjectActionAuthorizer, error) {
	if tasks == nil || creators == nil || users == nil {
		return nil, task.ErrCreatorPolicyInvalid
	}
	return &TaskObjectActionAuthorizer{tasks: tasks, creators: creators, users: users}, nil
}

func (authorizer *TaskObjectActionAuthorizer) AuthorizeObjectAction(
	ctx context.Context,
	request toolruntime.ObjectActionRequest,
) (bool, error) {
	if authorizer == nil || authorizer.tasks == nil || authorizer.creators == nil || authorizer.users == nil ||
		request.PolicyCode != taskCreatorObjectPolicy || len(request.Input) == 0 {
		return false, nil
	}
	ids, err := authorizer.taskIDs(ctx, request)
	if err != nil || len(ids) == 0 {
		return false, err
	}
	factUserID, err := authorizer.users.FactUserID(ctx, request.Authorization.PrincipalID)
	if err != nil || factUserID == 0 {
		return false, err
	}
	return authorizer.creators.AllCreatedBy(ctx, request.Authorization.WorkspaceID, factUserID, ids)
}

func (authorizer *TaskObjectActionAuthorizer) taskIDs(
	ctx context.Context,
	request toolruntime.ObjectActionRequest,
) ([]uint, error) {
	switch request.ToolName {
	case TaskUpdate, TaskStatusUpdate, TaskDelete, TaskDueConflictList:
		var input struct {
			ID uint `json:"id"`
		}
		if json.Unmarshal(request.Input, &input) != nil || input.ID == 0 {
			return nil, task.ErrCreatorPolicyInvalid
		}
		return []uint{input.ID}, nil
	case TaskBatchUpdate, TaskBatchStatusUpdate, TaskBatchDelete:
		var input taskBatchWriteSelector
		if json.Unmarshal(request.Input, &input) != nil {
			return nil, task.ErrCreatorPolicyInvalid
		}
		ids := append([]uint(nil), input.IDs...)
		seen := make(map[uint]struct{}, len(ids)+len(input.TaskRefs))
		for _, id := range ids {
			if id == 0 {
				return nil, task.ErrCreatorPolicyInvalid
			}
			seen[id] = struct{}{}
		}
		for _, reference := range input.TaskRefs {
			item, err := authorizer.tasks.GetTask(ctx, request.Authorization.WorkspaceID, task.Lookup{
				Reference: reference, ProjectSetID: input.ProjectSetID,
				PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID,
			})
			if err != nil {
				return nil, err
			}
			if _, exists := seen[item.ID]; !exists {
				seen[item.ID] = struct{}{}
				ids = append(ids, item.ID)
			}
		}
		if len(ids) == 0 || len(ids) > 500 {
			return nil, task.ErrCreatorPolicyInvalid
		}
		return ids, nil
	default:
		return nil, nil
	}
}

var _ toolruntime.ObjectActionAuthorizer = (*TaskObjectActionAuthorizer)(nil)
