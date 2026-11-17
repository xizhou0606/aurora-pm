package pmtools

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

const BIScreenGet = "bi.screen.get"

var biScreenReadHandlerNames = []string{BIScreenGet}

type BIScreenQueries interface {
	Get(context.Context, workbench.BIScreenQuery) (workbench.BIScreenResult, error)
}

func BIScreenReadHandlerNames() []string {
	return append([]string(nil), biScreenReadHandlerNames...)
}

func NewBIScreenReadHandlers(queries BIScreenQueries) (map[string]toolruntime.ToolHandler, error) {
	if queries == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		BIScreenGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input biScreenInput
			if err := decodeInput(call.Input, &input); err != nil {
				return nil, err
			}
			return queries.Get(ctx, workbench.BIScreenQuery{
				WorkspaceID:   call.Authorization.WorkspaceID,
				ProjectSetIDs: biOptionalID(input.ProjectSetID), PlanUnitIDs: biOptionalID(input.PlanUnitID),
				RequirementIDs: biOptionalID(input.RequirementID), Period: input.Period,
				Location: time.UTC,
			})
		}),
	}, nil
}

type biScreenInput struct {
	ProjectSetID  uint   `json:"project_set_id"`
	PlanUnitID    uint   `json:"project_new_id"`
	RequirementID uint   `json:"requirement_id"`
	Period        string `json:"period"`
}

func biOptionalID(value uint) []uint {
	if value == 0 {
		return nil
	}
	return []uint{value}
}
