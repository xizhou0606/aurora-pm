package pmtools

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	UserMe              = toolruntime.PMCurrentUserToolName
	UserSearch          = "user.search"
	TeamMemberList      = "team.member.list"
	WorkspaceMemberList = "workspace.member.list"
)

var accessReadHandlerNames = []string{UserMe, UserSearch, TeamMemberList, WorkspaceMemberList}

type AccessReadQueries interface {
	CurrentWorkspaceUser(context.Context, uint, uint) (identity.ToolUser, error)
	ListWorkspaceMembers(context.Context, uint) (identity.ToolUserPage, error)
	SearchWorkspaceUsers(context.Context, uint, identity.ToolUserFilter) (identity.ToolUserPage, error)
	ListTeamMembers(context.Context, uint, uint) (identity.ToolTeamMemberPage, error)
}

func AccessReadHandlerNames() []string { return append([]string(nil), accessReadHandlerNames...) }

func NewAccessReadHandlers(queries AccessReadQueries) (map[string]toolruntime.ToolHandler, error) {
	if queries == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		UserMe: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			return queries.CurrentWorkspaceUser(ctx, call.Authorization.WorkspaceID, call.Authorization.PrincipalID)
		}),
		UserSearch: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input userSearchInput
			if err := decodeInput(call.Input, &input); err != nil || input.Offset < 0 {
				return nil, projectReadInputError("invalid user search")
			}
			return queries.SearchWorkspaceUsers(ctx, call.Authorization.WorkspaceID, identity.ToolUserFilter{
				ID: input.ID, Query: input.Query, Username: input.Username, Exact: input.Exact,
				Limit: input.Limit, Offset: input.Offset,
			})
		}),
		TeamMemberList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			return queries.ListTeamMembers(ctx, call.Authorization.WorkspaceID, input.ID)
		}),
		WorkspaceMemberList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			if input.ID != call.Authorization.WorkspaceID {
				return nil, projectReadInputError("workspace id does not match the authorized workspace")
			}
			return queries.ListWorkspaceMembers(ctx, call.Authorization.WorkspaceID)
		}),
	}, nil
}

type userSearchInput struct {
	ID       uint   `json:"id"`
	Query    string `json:"q"`
	Username string `json:"username"`
	Exact    bool   `json:"exact"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}
