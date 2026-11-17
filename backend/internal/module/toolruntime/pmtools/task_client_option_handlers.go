package pmtools

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const TaskClientOptionList = "task.client_option.list"

type TaskClientOptionQueries interface {
	List(context.Context, uint, uint) ([]task.ClientOption, error)
}

func TaskClientOptionHandlerNames() []string { return []string{TaskClientOptionList} }

func NewTaskClientOptionHandlers(queries TaskClientOptionQueries) (map[string]toolruntime.ToolHandler, error) {
	if queries == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		TaskClientOptionList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			items, err := queries.List(
				ctx,
				call.Authorization.WorkspaceID,
				call.Authorization.PrincipalID,
			)
			return taskListEnvelope(items), err
		}),
	}, nil
}
