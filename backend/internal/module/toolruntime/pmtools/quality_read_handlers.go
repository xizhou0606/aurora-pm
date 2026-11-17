package pmtools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	QualityBugList         = "quality.bug.list"
	QualityBugGet          = "quality.bug.get"
	QualityBugActivityList = "quality.bug.activity.list"
)

var qualityReadHandlerNames = []string{QualityBugList, QualityBugGet, QualityBugActivityList}

type QualityReadQueries interface {
	ListBugs(context.Context, uint, quality.BugReadFilter) (quality.BugReadPage, error)
	GetBug(context.Context, uint, uint) (quality.BugSnapshot, error)
	ListBugActivities(context.Context, uint, uint, int, int) (quality.BugActivityPage, error)
}

var _ QualityReadQueries = (*quality.BugReadService)(nil)

func QualityReadHandlerNames() []string {
	return append([]string(nil), qualityReadHandlerNames...)
}

func NewQualityReadHandlers(queries QualityReadQueries) (map[string]toolruntime.ToolHandler, error) {
	if queries == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		QualityBugList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			filter, err := qualityBugListFilter(call.Input)
			if err != nil {
				return nil, err
			}
			return queries.ListBugs(ctx, call.Authorization.WorkspaceID, filter)
		}),
		QualityBugGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			return queries.GetBug(ctx, call.Authorization.WorkspaceID, input.ID)
		}),
		QualityBugActivityList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input qualityBugActivityInput
			if err := decodeInput(call.Input, &input); err != nil || input.ID == 0 || input.Offset < 0 {
				return nil, fmt.Errorf("%w: id and valid pagination are required", ErrReadHandlersInvalid)
			}
			return queries.ListBugActivities(ctx, call.Authorization.WorkspaceID, input.ID, input.Limit, input.Offset)
		}),
	}, nil
}

type qualityBugListInput struct {
	ProjectSetID     uint   `json:"project_set_id"`
	PlanUnitID       uint   `json:"project_new_id"`
	RequirementID    uint   `json:"requirement_id"`
	Query            string `json:"q"`
	Status           string `json:"status"`
	Priority         string `json:"priority"`
	SeverityKey      string `json:"severity_key"`
	FoundEnvironment string `json:"found_environment"`
	FoundVersion     string `json:"found_version"`
	BugType          string `json:"bug_type"`
	DefectDomain     string `json:"defect_domain"`
	CurrentOwnerID   uint   `json:"current_owner_user_id"`
	AssigneeID       uint   `json:"assignee_id"`
	ReporterUserID   uint   `json:"reporter_user_id"`
	Limit            int    `json:"limit"`
	Offset           int    `json:"offset"`
}

type qualityBugActivityInput struct {
	ID     uint `json:"id"`
	Limit  int  `json:"limit"`
	Offset int  `json:"offset"`
}

func qualityBugListFilter(raw json.RawMessage) (quality.BugReadFilter, error) {
	var input qualityBugListInput
	if err := decodeInput(raw, &input); err != nil || input.Offset < 0 {
		return quality.BugReadFilter{}, fmt.Errorf("%w: invalid quality bug filter", ErrReadHandlersInvalid)
	}
	return quality.BugReadFilter{
		ProjectSetID: input.ProjectSetID, PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID,
		Query: input.Query, Statuses: []string{input.Status}, Priorities: []string{input.Priority},
		SeverityKeys: []string{input.SeverityKey}, FoundEnvironments: []string{input.FoundEnvironment},
		FoundVersion: input.FoundVersion, BugTypes: []string{input.BugType}, DefectDomains: []string{input.DefectDomain},
		CurrentOwnerID: input.CurrentOwnerID,
		AssigneeID:     input.AssigneeID, ReporterUserID: input.ReporterUserID, Limit: input.Limit, Offset: input.Offset,
	}, nil
}
