package pmtools

import (
	"context"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	ProjectSetList         = "project.set.list"
	ProjectPlanUnitList    = "project.plan_unit.list"
	ProjectAccessEffective = "project.access.effective"
)

var projectListHandlerNames = []string{ProjectSetList, ProjectPlanUnitList}
var projectAccessHandlerNames = []string{ProjectAccessEffective}

// ProjectAccessUserResolver validates the internal PM principal used by both
// Tool authorization and project data. Returning 0 fails closed.
type ProjectAccessUserResolver interface {
	FactUserID(ctx context.Context, principalID uint) (uint, error)
}

type ProjectListQueries interface {
	ListProjectSets(context.Context, uint, project.ProjectSetListFilter) (project.ProjectSetListPage, error)
	ListAccessibleProjectSets(context.Context, uint, project.ProjectSetAccessListFilter) (project.ProjectSetListPage, error)
	ListPlanUnits(context.Context, uint, uint) ([]project.PlanUnitSnapshot, error)
}

type ProjectAccessQueries interface {
	GetProjectSet(context.Context, uint, uint) (project.ProjectSetSnapshot, error)
}

type ProjectListDependencies struct {
	Projects ProjectListQueries
	// AccessFilterBuilder resolves the caller's project membership filter.
	// When nil the project set list fails closed (no rows).
	Access ProjectAccessUserResolver
}

type ProjectAccessDependencies struct{ Projects ProjectAccessQueries }

func ProjectListHandlerNames() []string {
	return append([]string(nil), projectListHandlerNames...)
}

func ProjectAccessHandlerNames() []string {
	return append([]string(nil), projectAccessHandlerNames...)
}

func NewProjectListHandlers(dependencies ProjectListDependencies) (map[string]toolruntime.ToolHandler, error) {
	if dependencies.Projects == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		ProjectSetList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input projectSetListInput
			if err := decodeInput(call.Input, &input); err != nil {
				return nil, err
			}
			// Project sets are project data: the list is restricted to the
			// caller's accessible projects (owner / participant / admin). The
			// membership filter is resolved from the fact user ID.
			accessFilter, err := resolveProjectAccessFilter(ctx, call, dependencies)
			if err != nil {
				return nil, err
			}
			page, err := dependencies.Projects.ListAccessibleProjectSets(ctx, call.Authorization.WorkspaceID, project.ProjectSetAccessListFilter{
				ProjectSetListFilter: project.ProjectSetListFilter{
					TeamID: input.TeamID, Query: input.Query, Limit: input.Limit, Offset: input.Offset,
				},
				Access: accessFilter,
			})
			if err != nil {
				return nil, err
			}
			return projectSetListOutput{Items: nonNilProjectSets(page.Items), Total: page.Total}, nil
		}),
		ProjectPlanUnitList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input projectPlanUnitListInput
			if err := decodeInput(call.Input, &input); err != nil || input.ProjectSetID == 0 {
				return nil, fmt.Errorf("%w: project_new_id is required", ErrReadHandlersInvalid)
			}
			items, err := dependencies.Projects.ListPlanUnits(ctx, call.Authorization.WorkspaceID, input.ProjectSetID)
			return makeListOutput(items), err
		}),
	}, nil
}

// resolveProjectAccessFilter translates the tool-runtime principal into the
// fact user ID and resolves its project membership filter. A missing resolver
// fails closed with a filter that matches nothing.
func resolveProjectAccessFilter(
	ctx context.Context,
	call toolruntime.ToolCall,
	dependencies ProjectListDependencies,
) (project.AccessFilter, error) {
	if call.Authorization.PrincipalID == 0 {
		return project.AccessFilter{SQL: "1 = 0"}, nil
	}
	if dependencies.Access == nil {
		return project.AccessFilter{SQL: "1 = 0"}, nil
	}
	userID, err := dependencies.Access.FactUserID(ctx, call.Authorization.PrincipalID)
	if err != nil {
		return project.AccessFilter{}, err
	}
	if userID == 0 {
		return project.AccessFilter{SQL: "1 = 0"}, nil
	}
	builder, ok := dependencies.Access.(project.AccessFilterBuilder)
	if !ok {
		return project.AccessFilter{SQL: "1 = 0"}, nil
	}
	return builder.BuildMemberFilter(ctx, "project_sets", "id", call.Authorization.WorkspaceID, userID)
}

func NewProjectAccessHandlers(dependencies ProjectAccessDependencies) (map[string]toolruntime.ToolHandler, error) {
	if dependencies.Projects == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		ProjectAccessEffective: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			if call.Authorization.PrincipalID == 0 {
				return nil, ErrReadHandlersInvalid
			}
			var input projectAccessEffectiveInput
			if err := decodeInput(call.Input, &input); err != nil || input.ProjectSetID == 0 {
				return nil, fmt.Errorf("%w: project_set_id is required", ErrReadHandlersInvalid)
			}
			item, err := dependencies.Projects.GetProjectSet(ctx, call.Authorization.WorkspaceID, input.ProjectSetID)
			if err != nil {
				return nil, err
			}
			ownerMatch := item.OwnerUserID != nil && *item.OwnerUserID == call.Authorization.PrincipalID
			return projectAccessEffectiveOutput{
				Effective: true,
				Actor:     projectAccessActor{UserID: call.Authorization.PrincipalID},
				Workspace: projectAccessWorkspace{ID: call.Authorization.WorkspaceID, Effective: true},
				ProjectSet: projectAccessProjectSet{
					ID: item.ID, Code: item.Code, Name: item.Name, OwnerUserID: item.OwnerUserID, ActorIsOwner: ownerMatch,
				},
				AccessBasis: projectAccessBasis{
					Model: "external_grant_plus_pm_authorization", WorkspaceIsDataContext: true,
					ProjectOwnerIsInformational: true, LegacyProjectPermissionUsed: false,
				},
			}, nil
		}),
	}, nil
}

func nonNilProjectSets(items []project.ProjectSetSnapshot) []project.ProjectSetSnapshot {
	if items == nil {
		return []project.ProjectSetSnapshot{}
	}
	return items
}
