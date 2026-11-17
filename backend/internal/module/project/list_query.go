package project

import (
	"context"
	"strings"
)

const (
	DefaultProjectSetListLimit = 20
	MaximumProjectSetListLimit = 500
)

type ProjectSetListFilter struct {
	TeamID uint
	Query  string
	Limit  int
	Offset int
}

// ProjectSetAccessListFilter extends ProjectSetListFilter with the caller's
// resolved project access filter. AccessFilter is produced by
// AccessService.BuildMemberFilter; an empty SQL is the admin fast path (no
// restriction). A nil filter fails closed: no project is enumerable.
type ProjectSetAccessListFilter struct {
	ProjectSetListFilter
	Access AccessFilter
}

type ProjectSetListPage struct {
	Items []ProjectSetSnapshot
	Total int64
}

type ListStore interface {
	ListProjectSets(context.Context, uint, ProjectSetListFilter) (ProjectSetListPage, error)
	ListPlanUnits(context.Context, uint, uint) ([]PlanUnitSnapshot, error)
}

// ProjectSetAccessListStore lists project sets under a caller-resolved access
// filter. The GORM adapter owns the SQL; the domain resolves the filter.
type ProjectSetAccessListStore interface {
	ListAccessibleProjectSets(context.Context, uint, ProjectSetAccessListFilter) (ProjectSetListPage, error)
}

type ListService struct {
	store             ListStore
	accessListStore   ProjectSetAccessListStore
	accessListEnabled bool
}

func NewListService(store ListStore) (*ListService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	service := &ListService{store: store}
	if accessStore, ok := store.(ProjectSetAccessListStore); ok {
		service.accessListStore = accessStore
		service.accessListEnabled = true
	}
	return service, nil
}

// ListAccessibleProjectSets lists project sets restricted to the caller's
// accessible projects. The access filter must already be resolved by the
// caller (AccessService.BuildMemberFilter); when the composed store does not
// support access-filtered listing the call fails closed.
func (service *ListService) ListAccessibleProjectSets(
	ctx context.Context,
	workspaceID uint,
	filter ProjectSetAccessListFilter,
) (ProjectSetListPage, error) {
	if workspaceID == 0 || filter.Offset < 0 {
		return ProjectSetListPage{}, ErrReadQueryInvalid
	}
	if !service.accessListEnabled || service.accessListStore == nil {
		// Fail closed: without an access-aware store the service must not
		// enumerate projects the caller may not access.
		return ProjectSetListPage{Items: []ProjectSetSnapshot{}, Total: 0}, nil
	}
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Limit = normalizeProjectSetListLimit(filter.Limit)
	return service.accessListStore.ListAccessibleProjectSets(ctx, workspaceID, filter)
}

func (service *ListService) ListProjectSets(
	ctx context.Context,
	workspaceID uint,
	filter ProjectSetListFilter,
) (ProjectSetListPage, error) {
	if workspaceID == 0 || filter.Offset < 0 {
		return ProjectSetListPage{}, ErrReadQueryInvalid
	}
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Limit = normalizeProjectSetListLimit(filter.Limit)
	return service.store.ListProjectSets(ctx, workspaceID, filter)
}

func (service *ListService) ListPlanUnits(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
) ([]PlanUnitSnapshot, error) {
	if workspaceID == 0 || projectSetID == 0 {
		return nil, ErrReadQueryInvalid
	}
	items, err := service.store.ListPlanUnits(ctx, workspaceID, projectSetID)
	if items == nil && err == nil {
		items = []PlanUnitSnapshot{}
	}
	return items, err
}

func normalizeProjectSetListLimit(limit int) int {
	if limit < 1 {
		return DefaultProjectSetListLimit
	}
	if limit > MaximumProjectSetListLimit {
		return MaximumProjectSetListLimit
	}
	return limit
}
