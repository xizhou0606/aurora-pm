package pmtools

import (
	"context"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

const (
	WorkbenchTodoSummary = "workbench.todo.summary"
	WorkbenchTodoFeed    = "workbench.todo.feed"
)

var workbenchReadHandlerNames = []string{WorkbenchTodoSummary, WorkbenchTodoFeed}

type WorkbenchReadQueries interface {
	TodoSummary(context.Context, uint, uint, workbench.TodoFilter) (workbench.TodoSummaryOutput, error)
	TodoFeed(context.Context, uint, uint, workbench.TodoFilter) (workbench.TodoFeedOutput, error)
}

func WorkbenchReadHandlerNames() []string {
	return append([]string(nil), workbenchReadHandlerNames...)
}

func NewWorkbenchReadHandlers(queries WorkbenchReadQueries) (map[string]toolruntime.ToolHandler, error) {
	if queries == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		WorkbenchTodoSummary: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			if call.Authorization.PrincipalID == 0 {
				return nil, fmt.Errorf("%w: principal is required", ErrReadHandlersInvalid)
			}
			var input workbenchTodoSummaryInput
			if err := decodeInput(call.Input, &input); err != nil {
				return nil, err
			}
			return queries.TodoSummary(ctx, call.Authorization.WorkspaceID, call.Authorization.PrincipalID, workbench.TodoFilter{
				ProjectSetID: input.ProjectSetID, PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID,
				Query: input.Query, Domains: input.Domains, IncludeMentions: input.IncludeMentions,
			})
		}),
		WorkbenchTodoFeed: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			if call.Authorization.PrincipalID == 0 {
				return nil, fmt.Errorf("%w: principal is required", ErrReadHandlersInvalid)
			}
			var input workbenchTodoFeedInput
			if err := decodeInput(call.Input, &input); err != nil || input.Offset < 0 {
				return nil, fmt.Errorf("%w: invalid workbench todo feed", ErrReadHandlersInvalid)
			}
			return queries.TodoFeed(ctx, call.Authorization.WorkspaceID, call.Authorization.PrincipalID, workbench.TodoFilter{
				ProjectSetID: input.ProjectSetID, PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID,
				Query: input.Query, Type: input.Type, Limit: input.Limit, Offset: input.Offset,
			})
		}),
	}, nil
}

type workbenchTodoSummaryInput struct {
	ProjectSetID    uint   `json:"project_set_id"`
	PlanUnitID      uint   `json:"project_new_id"`
	RequirementID   uint   `json:"requirement_id"`
	Query           string `json:"q"`
	Domains         string `json:"domains"`
	IncludeMentions bool   `json:"include_mentions"`
}

type workbenchTodoFeedInput struct {
	ProjectSetID  uint   `json:"project_set_id"`
	PlanUnitID    uint   `json:"project_new_id"`
	RequirementID uint   `json:"requirement_id"`
	Query         string `json:"q"`
	Type          string `json:"type"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}
