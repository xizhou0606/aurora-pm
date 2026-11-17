package requirement

import (
	"context"
	"strings"
)

const (
	DefaultListLimit = 50
	MaximumListLimit = 500
)

type ListFilter struct {
	ProjectSetID uint
	PlanUnitID   uint
	Query        string
	Limit        int
	Offset       int
}

type ListPage struct {
	Items []Snapshot `json:"items"`
	Total int64      `json:"total"`
}

type ListStore interface {
	ListRequirements(context.Context, uint, ListFilter) (ListPage, error)
	ListScopeRequirements(context.Context, uint, []uint) ([]Snapshot, error)
}

type ListService struct{ store ListStore }

func NewListService(store ListStore) (*ListService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ListService{store: store}, nil
}

func (service *ListService) ListRequirements(ctx context.Context, workspaceID uint, filter ListFilter) (ListPage, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if workspaceID == 0 || filter.Offset < 0 || (filter.ProjectSetID == 0 && filter.PlanUnitID == 0 && filter.Query == "") {
		return ListPage{}, ErrReadQueryInvalid
	}
	filter.Limit = normalizeListLimit(filter.Limit)
	page, err := service.store.ListRequirements(ctx, workspaceID, filter)
	if page.Items == nil && err == nil {
		page.Items = []Snapshot{}
	}
	return page, err
}

func (service *ListService) ListScopeRequirements(ctx context.Context, workspaceID uint, projectSetIDs []uint) ([]Snapshot, error) {
	if workspaceID == 0 {
		return nil, ErrReadQueryInvalid
	}
	if len(projectSetIDs) == 0 {
		return []Snapshot{}, nil
	}
	items, err := service.store.ListScopeRequirements(ctx, workspaceID, projectSetIDs)
	if items == nil && err == nil {
		items = []Snapshot{}
	}
	return items, err
}

func normalizeListLimit(value int) int {
	if value < 1 {
		return DefaultListLimit
	}
	if value > MaximumListLimit {
		return MaximumListLimit
	}
	return value
}
