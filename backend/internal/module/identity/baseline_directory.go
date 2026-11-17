package identity

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrBaselineDirectoryInvalid     = errors.New("baseline directory query is invalid")
	ErrBaselineDirectoryTeamMissing = errors.New("baseline directory team is unavailable")
)

type AvailableWorkspace struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	IsDefault bool   `json:"is_default"`
}

type DirectoryUser struct {
	Email       string `json:"email,omitempty"`
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type DirectoryUserFilter struct {
	Query    string
	Username string
	IDs      []uint
	Exact    bool
	Limit    int
	Offset   int
}

type DirectoryUserPage struct {
	Items []DirectoryUser `json:"items"`
	Total int64           `json:"total"`
}

type DirectoryTeam struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type DirectoryTeamMember struct {
	DirectoryUser
	Role string `json:"role,omitempty"`
}

type BaselineDirectoryStore interface {
	ListAvailableWorkspaces(context.Context, uint) ([]AvailableWorkspace, error)
	SearchUsers(context.Context, uint, DirectoryUserFilter) (DirectoryUserPage, error)
	ListTeams(context.Context, uint) ([]DirectoryTeam, error)
	ListTeamMembers(context.Context, uint, uint) ([]DirectoryTeamMember, error)
}

type BaselineDirectoryService struct{ store BaselineDirectoryStore }

func NewBaselineDirectoryService(store BaselineDirectoryStore) (*BaselineDirectoryService, error) {
	if store == nil {
		return nil, ErrBaselineDirectoryInvalid
	}
	return &BaselineDirectoryService{store: store}, nil
}

func (service *BaselineDirectoryService) ListAvailableWorkspaces(
	ctx context.Context,
	principalID uint,
) ([]AvailableWorkspace, error) {
	if principalID == 0 {
		return nil, ErrBaselineDirectoryInvalid
	}
	items, err := service.store.ListAvailableWorkspaces(ctx, principalID)
	if items == nil && err == nil {
		items = []AvailableWorkspace{}
	}
	return items, err
}

func (service *BaselineDirectoryService) SearchUsers(
	ctx context.Context,
	workspaceID uint,
	filter DirectoryUserFilter,
) (DirectoryUserPage, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Username = strings.TrimSpace(filter.Username)
	filter.IDs = normalizeDirectoryUserIDs(filter.IDs)
	if workspaceID == 0 || filter.Offset < 0 {
		return DirectoryUserPage{}, ErrBaselineDirectoryInvalid
	}
	if len(filter.IDs) > 100 {
		return DirectoryUserPage{}, ErrBaselineDirectoryInvalid
	}
	if filter.Limit < 1 {
		filter.Limit = 100
	} else if filter.Limit > 500 {
		filter.Limit = 500
	}
	page, err := service.store.SearchUsers(ctx, workspaceID, filter)
	if page.Items == nil && err == nil {
		page.Items = []DirectoryUser{}
	}
	return page, err
}

func normalizeDirectoryUserIDs(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (service *BaselineDirectoryService) ListTeams(
	ctx context.Context,
	workspaceID uint,
) ([]DirectoryTeam, error) {
	if workspaceID == 0 {
		return nil, ErrBaselineDirectoryInvalid
	}
	items, err := service.store.ListTeams(ctx, workspaceID)
	if items == nil && err == nil {
		items = []DirectoryTeam{}
	}
	return items, err
}

func (service *BaselineDirectoryService) ListTeamMembers(
	ctx context.Context,
	workspaceID uint,
	teamID uint,
) ([]DirectoryTeamMember, error) {
	if workspaceID == 0 || teamID == 0 {
		return nil, ErrBaselineDirectoryInvalid
	}
	items, err := service.store.ListTeamMembers(ctx, workspaceID, teamID)
	if items == nil && err == nil {
		items = []DirectoryTeamMember{}
	}
	return items, err
}
