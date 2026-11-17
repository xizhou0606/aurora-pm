package identity

import (
	"context"
	"errors"
	"strings"
)

var ErrToolDirectoryQueryInvalid = errors.New("identity tool directory query is invalid")
var ErrToolCurrentUserUnavailable = errors.New("current workspace user is unavailable")

type ToolUserFilter struct {
	ID       uint
	Query    string
	Username string
	Exact    bool
	Limit    int
	Offset   int
}

type ToolUser struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type ToolUserPage struct {
	Items []ToolUser `json:"items"`
	Total int64      `json:"total"`
}

type ToolTeamMember struct {
	ToolUser
	Role string `json:"role"`
}

type ToolTeamMemberPage struct {
	Items []ToolTeamMember `json:"items"`
	Total int64            `json:"total"`
}

type ToolDirectoryStore interface {
	FindActiveUser(context.Context, uint) (ToolUser, error)
	ListWorkspaceMembers(context.Context, uint) (ToolUserPage, error)
	SearchWorkspaceUsers(context.Context, uint, ToolUserFilter) (ToolUserPage, error)
	ListTeamMembers(context.Context, uint, uint) (ToolTeamMemberPage, error)
}

type ToolDirectoryService struct{ store ToolDirectoryStore }

func NewToolDirectoryService(store ToolDirectoryStore) (*ToolDirectoryService, error) {
	if store == nil {
		return nil, ErrToolDirectoryQueryInvalid
	}
	return &ToolDirectoryService{store: store}, nil
}

func (service *ToolDirectoryService) SearchWorkspaceUsers(
	ctx context.Context,
	workspaceID uint,
	filter ToolUserFilter,
) (ToolUserPage, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Username = strings.TrimSpace(filter.Username)
	if workspaceID == 0 || filter.Offset < 0 {
		return ToolUserPage{}, ErrToolDirectoryQueryInvalid
	}
	if filter.Limit < 1 {
		filter.Limit = 50
	} else if filter.Limit > 200 {
		filter.Limit = 200
	}
	page, err := service.store.SearchWorkspaceUsers(ctx, workspaceID, filter)
	if page.Items == nil && err == nil {
		page.Items = []ToolUser{}
	}
	return page, err
}

func (service *ToolDirectoryService) CurrentWorkspaceUser(
	ctx context.Context,
	workspaceID uint,
	userID uint,
) (ToolUser, error) {
	if workspaceID == 0 || userID == 0 {
		return ToolUser{}, ErrToolDirectoryQueryInvalid
	}
	user, err := service.store.FindActiveUser(ctx, userID)
	if err != nil {
		return ToolUser{}, err
	}
	if user.ID != userID || strings.TrimSpace(user.Username) == "" || strings.TrimSpace(user.DisplayName) == "" {
		return ToolUser{}, ErrToolCurrentUserUnavailable
	}
	return user, nil
}

func (service *ToolDirectoryService) ListWorkspaceMembers(
	ctx context.Context,
	workspaceID uint,
) (ToolUserPage, error) {
	if workspaceID == 0 {
		return ToolUserPage{}, ErrToolDirectoryQueryInvalid
	}
	page, err := service.store.ListWorkspaceMembers(ctx, workspaceID)
	if page.Items == nil && err == nil {
		page.Items = []ToolUser{}
	}
	return page, err
}

func (service *ToolDirectoryService) ListTeamMembers(
	ctx context.Context,
	workspaceID uint,
	teamID uint,
) (ToolTeamMemberPage, error) {
	if workspaceID == 0 || teamID == 0 {
		return ToolTeamMemberPage{}, ErrToolDirectoryQueryInvalid
	}
	page, err := service.store.ListTeamMembers(ctx, workspaceID, teamID)
	if page.Items == nil && err == nil {
		page.Items = []ToolTeamMember{}
	}
	return page, err
}
