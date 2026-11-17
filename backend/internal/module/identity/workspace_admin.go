package identity

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	workspaceNameMaxLength        = 128
	workspaceSlugMaxLength        = 80
	workspaceDescriptionMaxLength = 500
)

var (
	ErrWorkspaceAdminNotFound      = errors.New("workspace not found")
	ErrWorkspaceNameInvalid        = errors.New("workspace name is invalid")
	ErrWorkspaceSlugInvalid        = errors.New("workspace slug is invalid")
	ErrWorkspaceDescriptionInvalid = errors.New("workspace description is invalid")
	ErrWorkspaceSlugConflict       = errors.New("workspace slug already exists")
	ErrWorkspaceDefaultProtected   = errors.New("default workspace is protected")
	ErrDefaultWorkspaceUnavailable = errors.New("exactly one enabled default workspace is required")
	ErrWorkspaceInUse              = errors.New("workspace is still in use")
	ErrWorkspaceMemberNotFound     = errors.New("workspace member not found")
	ErrWorkspaceMemberConflict     = errors.New("workspace member already exists")
	ErrWorkspaceMemberRoleInvalid  = errors.New("workspace member role is invalid")
	ErrWorkspaceMemberUserMissing  = errors.New("workspace member user not found")
	ErrWorkspaceMemberQueryInvalid = errors.New("workspace member query is invalid")
)

type ManagedWorkspace struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	IsDefault   bool      `json:"is_default"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkspaceInput struct {
	Name        string
	Slug        string
	Description string
	Enabled     bool
	IsDefault   bool
}

type WorkspaceUpdate struct {
	Name        *string
	Description *string
	Enabled     *bool
	IsDefault   *bool
}

type ManagedWorkspaceMember struct {
	UserID      uint          `json:"user_id"`
	Username    string        `json:"username"`
	DisplayName string        `json:"display_name"`
	Email       string        `json:"email,omitempty"`
	AvatarURL   string        `json:"avatar_url,omitempty"`
	Status      AccountStatus `json:"status"`
	Role        string        `json:"role"`
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type WorkspaceMemberInput struct {
	UserID  uint
	Role    string
	Enabled bool
}

type WorkspaceMemberUpdate struct {
	Role    *string
	Enabled *bool
}

type WorkspaceMemberFilter struct {
	Query           string
	IncludeDisabled bool
	Page            int
	PageSize        int
}

type ManagedWorkspaceMemberPage struct {
	Items    []ManagedWorkspaceMember `json:"items"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Total    int64                    `json:"total"`
	HasMore  bool                     `json:"has_more"`
}

type WorkspaceAdminStore interface {
	ListManagedWorkspaces(context.Context, uint, bool) ([]ManagedWorkspace, error)
	FindManagedWorkspace(context.Context, uint) (ManagedWorkspace, error)
	FindVisibleManagedWorkspace(context.Context, uint, uint) (ManagedWorkspace, error)
	CreateManagedWorkspace(context.Context, WorkspaceInput) (ManagedWorkspace, error)
	UpdateManagedWorkspace(context.Context, uint, WorkspaceUpdate) (ManagedWorkspace, error)
	DeleteManagedWorkspace(context.Context, uint) error
	ListManagedWorkspaceMembers(context.Context, uint, WorkspaceMemberFilter) (ManagedWorkspaceMemberPage, error)
	CreateManagedWorkspaceMember(context.Context, uint, WorkspaceMemberInput) (ManagedWorkspaceMember, error)
	UpdateManagedWorkspaceMember(context.Context, uint, uint, WorkspaceMemberUpdate) (ManagedWorkspaceMember, error)
	DeleteManagedWorkspaceMember(context.Context, uint, uint) error
}

type WorkspaceAdminService struct{ store WorkspaceAdminStore }

func NewWorkspaceAdminService(store WorkspaceAdminStore) *WorkspaceAdminService {
	return &WorkspaceAdminService{store: store}
}

func (service *WorkspaceAdminService) List(ctx context.Context, principalID uint, includeDisabled bool) ([]ManagedWorkspace, error) {
	if principalID == 0 {
		return nil, ErrAccessUserNotFound
	}
	return service.store.ListManagedWorkspaces(ctx, principalID, includeDisabled)
}

func (service *WorkspaceAdminService) Workspace(ctx context.Context, principalID, workspaceID uint) (ManagedWorkspace, error) {
	if principalID == 0 || workspaceID == 0 {
		return ManagedWorkspace{}, ErrWorkspaceAdminNotFound
	}
	return service.store.FindVisibleManagedWorkspace(ctx, principalID, workspaceID)
}

func (service *WorkspaceAdminService) Create(ctx context.Context, input WorkspaceInput) (ManagedWorkspace, error) {
	normalized, err := normalizeWorkspaceInput(input)
	if err != nil {
		return ManagedWorkspace{}, err
	}
	return service.store.CreateManagedWorkspace(ctx, normalized)
}

func (service *WorkspaceAdminService) Update(ctx context.Context, principalID, workspaceID uint, input WorkspaceUpdate) (ManagedWorkspace, error) {
	if _, err := service.Workspace(ctx, principalID, workspaceID); err != nil {
		return ManagedWorkspace{}, err
	}
	normalized, err := normalizeWorkspaceUpdate(input)
	if err != nil {
		return ManagedWorkspace{}, err
	}
	return service.store.UpdateManagedWorkspace(ctx, workspaceID, normalized)
}

func (service *WorkspaceAdminService) Delete(ctx context.Context, principalID, workspaceID uint) error {
	if _, err := service.Workspace(ctx, principalID, workspaceID); err != nil {
		return err
	}
	return service.store.DeleteManagedWorkspace(ctx, workspaceID)
}

func (service *WorkspaceAdminService) ListMembers(ctx context.Context, principalID, workspaceID uint, filter WorkspaceMemberFilter) (ManagedWorkspaceMemberPage, error) {
	if _, err := service.Workspace(ctx, principalID, workspaceID); err != nil {
		return ManagedWorkspaceMemberPage{}, err
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if len([]rune(filter.Query)) > 200 {
		return ManagedWorkspaceMemberPage{}, ErrWorkspaceMemberQueryInvalid
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return service.store.ListManagedWorkspaceMembers(ctx, workspaceID, filter)
}

func (service *WorkspaceAdminService) AddMember(ctx context.Context, principalID, workspaceID uint, input WorkspaceMemberInput) (ManagedWorkspaceMember, error) {
	if _, err := service.Workspace(ctx, principalID, workspaceID); err != nil {
		return ManagedWorkspaceMember{}, err
	}
	if input.UserID == 0 {
		return ManagedWorkspaceMember{}, ErrWorkspaceMemberUserMissing
	}
	role, err := normalizeWorkspaceMemberRole(input.Role)
	if err != nil {
		return ManagedWorkspaceMember{}, err
	}
	input.Role = role
	return service.store.CreateManagedWorkspaceMember(ctx, workspaceID, input)
}

func (service *WorkspaceAdminService) UpdateMember(ctx context.Context, principalID, workspaceID, userID uint, input WorkspaceMemberUpdate) (ManagedWorkspaceMember, error) {
	if userID == 0 {
		return ManagedWorkspaceMember{}, ErrWorkspaceMemberNotFound
	}
	if _, err := service.Workspace(ctx, principalID, workspaceID); err != nil {
		return ManagedWorkspaceMember{}, err
	}
	if input.Role != nil {
		role, err := normalizeWorkspaceMemberRole(*input.Role)
		if err != nil {
			return ManagedWorkspaceMember{}, err
		}
		input.Role = &role
	}
	return service.store.UpdateManagedWorkspaceMember(ctx, workspaceID, userID, input)
}

func (service *WorkspaceAdminService) DeleteMember(ctx context.Context, principalID, workspaceID, userID uint) error {
	if userID == 0 {
		return ErrWorkspaceMemberNotFound
	}
	if _, err := service.Workspace(ctx, principalID, workspaceID); err != nil {
		return err
	}
	return service.store.DeleteManagedWorkspaceMember(ctx, workspaceID, userID)
}

func normalizeWorkspaceInput(input WorkspaceInput) (WorkspaceInput, error) {
	name, err := normalizeWorkspaceName(input.Name)
	if err != nil {
		return WorkspaceInput{}, err
	}
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	if slug == "" || len(slug) > workspaceSlugMaxLength || !accessCodePattern.MatchString(slug) {
		return WorkspaceInput{}, ErrWorkspaceSlugInvalid
	}
	description, err := normalizeWorkspaceDescription(input.Description)
	if err != nil {
		return WorkspaceInput{}, err
	}
	return WorkspaceInput{Name: name, Slug: slug, Description: description, Enabled: input.Enabled, IsDefault: input.IsDefault}, nil
}

func normalizeWorkspaceUpdate(input WorkspaceUpdate) (WorkspaceUpdate, error) {
	if input.Name != nil {
		value, err := normalizeWorkspaceName(*input.Name)
		if err != nil {
			return WorkspaceUpdate{}, err
		}
		input.Name = &value
	}
	if input.Description != nil {
		value, err := normalizeWorkspaceDescription(*input.Description)
		if err != nil {
			return WorkspaceUpdate{}, err
		}
		input.Description = &value
	}
	return input, nil
}

func normalizeWorkspaceName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len([]rune(value)) > workspaceNameMaxLength {
		return "", ErrWorkspaceNameInvalid
	}
	return value, nil
}

func normalizeWorkspaceDescription(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len([]rune(value)) > workspaceDescriptionMaxLength {
		return "", ErrWorkspaceDescriptionInvalid
	}
	return value, nil
}

func normalizeWorkspaceMemberRole(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value != "owner" && value != "member" {
		return "", ErrWorkspaceMemberRoleInvalid
	}
	return value, nil
}
