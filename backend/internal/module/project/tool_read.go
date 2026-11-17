package project

import (
	"context"
	"time"
)

type DeliveryLabelSnapshot struct {
	ID           uint      `json:"id"`
	WorkspaceID  uint      `json:"workspace_id"`
	ProjectSetID uint      `json:"project_set_id"`
	PlanUnitID   uint      `json:"plan_unit_id"`
	ClientKey    string    `json:"client_key,omitempty"`
	Name         string    `json:"name"`
	SystemKey    string    `json:"system_key,omitempty"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ScopeProjectSet struct {
	ID     uint   `json:"id"`
	TeamID uint   `json:"team_id"`
	Name   string `json:"name"`
	Code   string `json:"code,omitempty"`
}

type ScopeOptions struct {
	ProjectSets []ScopeProjectSet  `json:"project_sets"`
	PlanUnits   []PlanUnitSnapshot `json:"projects"`
}

type ToolReadStore interface {
	ListDeliveryLabels(context.Context, uint, uint) ([]DeliveryLabelSnapshot, error)
	// ListScopeOptions returns the scope options for the caller's accessible
	// projects. The AccessFilter is resolved by ToolReadService from the
	// project access formula; an empty filter means no additional restriction
	// (the admin fast path).
	ListScopeOptions(context.Context, uint, AccessFilter) (ScopeOptions, error)
}

// ToolReadService layers project data access over the raw read store. A nil
// access builder fails closed: scope options are project data, so without an
// access formula the service must not enumerate projects.
type ToolReadService struct {
	store  ToolReadStore
	access AccessFilterBuilder
}

func NewToolReadService(store ToolReadStore, access AccessFilterBuilder) (*ToolReadService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ToolReadService{store: store, access: access}, nil
}

func (service *ToolReadService) ListDeliveryLabels(ctx context.Context, workspaceID, planUnitID uint) ([]DeliveryLabelSnapshot, error) {
	if workspaceID == 0 || planUnitID == 0 {
		return nil, ErrReadQueryInvalid
	}
	items, err := service.store.ListDeliveryLabels(ctx, workspaceID, planUnitID)
	if items == nil && err == nil {
		items = []DeliveryLabelSnapshot{}
	}
	return items, err
}

// ListScopeOptions enumerates the project sets and plan units the caller may
// select as scope filters. It applies the same project data-access formula as
// every other project-scoped read: admin fast path, otherwise owner or
// participant membership.
func (service *ToolReadService) ListScopeOptions(ctx context.Context, workspaceID, userID uint) (ScopeOptions, error) {
	if workspaceID == 0 || userID == 0 {
		return ScopeOptions{}, ErrParticipantInvalid
	}
	if service.access == nil {
		// Fail closed: no access formula means no project enumeration.
		return ScopeOptions{ProjectSets: []ScopeProjectSet{}, PlanUnits: []PlanUnitSnapshot{}}, nil
	}
	filter, err := service.access.BuildMemberFilter(ctx, "project_sets", "id", workspaceID, userID)
	if err != nil {
		return ScopeOptions{}, err
	}
	result, err := service.store.ListScopeOptions(ctx, workspaceID, filter)
	if err == nil {
		if result.ProjectSets == nil {
			result.ProjectSets = []ScopeProjectSet{}
		}
		if result.PlanUnits == nil {
			result.PlanUnits = []PlanUnitSnapshot{}
		}
	}
	return result, err
}
