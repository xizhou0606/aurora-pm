package audit

import (
	"context"
	"errors"
	"time"
)

var ErrReadQueryInvalid = errors.New("audit read query is invalid")

type EventSnapshot struct {
	ID          uint      `json:"id"`
	WorkspaceID uint      `json:"workspace_id"`
	UserID      uint      `json:"user_id"`
	Username    string    `json:"username,omitempty"`
	Role        string    `json:"role,omitempty"`
	Method      string    `json:"method,omitempty"`
	StatusCode  int       `json:"status_code,omitempty"`
	Resource    string    `json:"resource"`
	ResourceID  uint      `json:"resource_id"`
	ResourceRef string    `json:"resource_ref,omitempty"`
	ProjectID   uint      `json:"project_id,omitempty"`
	Action      string    `json:"action"`
	Target      string    `json:"target,omitempty"`
	Detail      string    `json:"detail,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProjectEventPage struct {
	Items []EventSnapshot `json:"items"`
	Total int64           `json:"total"`
}

type ReadStore interface {
	ListProjectEvents(context.Context, uint, uint, int, int) (ProjectEventPage, error)
}

type ReadService struct{ store ReadStore }

func NewReadService(store ReadStore) (*ReadService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ReadService{store: store}, nil
}

func (service *ReadService) ListProjectEvents(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
	limit int,
	offset int,
) (ProjectEventPage, error) {
	if workspaceID == 0 || projectSetID == 0 || offset < 0 {
		return ProjectEventPage{}, ErrReadQueryInvalid
	}
	if limit < 1 {
		limit = 30
	} else if limit > 500 {
		limit = 500
	}
	page, err := service.store.ListProjectEvents(ctx, workspaceID, projectSetID, limit, offset)
	if page.Items == nil && err == nil {
		page.Items = []EventSnapshot{}
	}
	return page, err
}
