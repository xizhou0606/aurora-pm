package task

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrTodoReadInvalid = errors.New("task todo read query is invalid")

type TodoFilter struct {
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
	Query         string
	Limit         int
	Offset        int
	Now           time.Time
}

type TodoItem struct {
	ListItem
	MentionAt *time.Time `json:"mention_at,omitempty"`
}

type TodoPage struct {
	Items []TodoItem `json:"items"`
	Total int64      `json:"total"`
}

type TodoReadStore interface {
	ListPrincipalTodos(context.Context, uint, uint, TodoFilter) (TodoPage, error)
	ListPrincipalMentions(context.Context, uint, uint, string, TodoFilter) (TodoPage, error)
}

type TodoReadService struct{ store TodoReadStore }

func NewTodoReadService(store TodoReadStore) (*TodoReadService, error) {
	if store == nil {
		return nil, ErrTodoReadInvalid
	}
	return &TodoReadService{store: store}, nil
}

func (service *TodoReadService) ListPrincipalTodos(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	filter TodoFilter,
) (TodoPage, error) {
	if err := prepareTodoFilter(workspaceID, principalID, &filter); err != nil {
		return TodoPage{}, err
	}
	return service.store.ListPrincipalTodos(ctx, workspaceID, principalID, filter)
}

func (service *TodoReadService) ListPrincipalMentions(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	username string,
	filter TodoFilter,
) (TodoPage, error) {
	if err := prepareTodoFilter(workspaceID, principalID, &filter); err != nil {
		return TodoPage{}, err
	}
	return service.store.ListPrincipalMentions(ctx, workspaceID, principalID, strings.TrimSpace(username), filter)
}

func prepareTodoFilter(workspaceID, principalID uint, filter *TodoFilter) error {
	if workspaceID == 0 || principalID == 0 || filter == nil || filter.Offset < 0 {
		return ErrTodoReadInvalid
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Limit < 1 {
		filter.Limit = 20
	} else if filter.Limit > 500 {
		filter.Limit = 500
	}
	if filter.Now.IsZero() {
		filter.Now = time.Now().UTC()
	} else {
		filter.Now = filter.Now.UTC()
	}
	return nil
}
