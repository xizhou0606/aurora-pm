package quality

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrBugTodoInvalid = errors.New("quality bug todo query is invalid")

type BugTodoFilter struct {
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
	Query         string
	Limit         int
	Offset        int
	Now           time.Time
}

type BugTodoStore interface {
	ListPrincipalBugTodos(context.Context, uint, uint, BugTodoFilter) (BugReadPage, error)
}

type BugTodoService struct{ store BugTodoStore }

func NewBugTodoService(store BugTodoStore) (*BugTodoService, error) {
	if store == nil {
		return nil, ErrBugTodoInvalid
	}
	return &BugTodoService{store: store}, nil
}

func (service *BugTodoService) ListPrincipalBugTodos(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	filter BugTodoFilter,
) (BugReadPage, error) {
	if workspaceID == 0 || principalID == 0 || filter.Offset < 0 {
		return BugReadPage{}, ErrBugTodoInvalid
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
	page, err := service.store.ListPrincipalBugTodos(ctx, workspaceID, principalID, filter)
	if page.Items == nil && err == nil {
		page.Items = []BugSnapshot{}
	}
	return page, err
}
