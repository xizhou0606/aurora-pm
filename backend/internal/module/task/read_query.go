package task

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrReadQueryInvalid = errors.New("task read query is invalid")
	ErrTaskNotFound     = errors.New("task not found")
	ErrTaskAmbiguous    = errors.New("task reference is ambiguous")
)

type Lookup struct {
	ID            uint
	Reference     string
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
}

type Reference struct {
	ID      uint   `json:"id"`
	TaskKey string `json:"task_key"`
	Title   string `json:"title"`
}

type Snapshot struct {
	ID               uint        `json:"id"`
	WorkspaceID      uint        `json:"workspace_id"`
	TeamID           uint        `json:"team_id"`
	ProjectSetID     uint        `json:"project_set_id"`
	PlanUnitID       uint        `json:"project_new_id"`
	RequirementID    uint        `json:"requirement_id"`
	TaskClientID     *uint       `json:"task_client_id,omitempty"`
	Kind             string      `json:"kind"`
	TaskKey          string      `json:"task_key"`
	Title            string      `json:"title"`
	Description      string      `json:"description,omitempty"`
	Status           string      `json:"status"`
	Priority         string      `json:"priority"`
	AssigneeID       *uint       `json:"assignee_id,omitempty"`
	ReporterUserID   *uint       `json:"reporter_user_id,omitempty"`
	ParentID         *uint       `json:"parent_id,omitempty"`
	EstimatedMinutes int         `json:"estimated_minutes"`
	StartsAt         *time.Time  `json:"starts_at,omitempty"`
	DueAt            *time.Time  `json:"due_at,omitempty"`
	CompletedAt      *time.Time  `json:"completed_at,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	Parent           *Reference  `json:"parent,omitempty"`
	Children         []Reference `json:"children"`
}

type ReadStore interface {
	GetTask(context.Context, uint, Lookup) (Snapshot, error)
}

type ReadService struct{ store ReadStore }

func NewReadService(store ReadStore) (*ReadService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ReadService{store: store}, nil
}

func (service *ReadService) GetTask(ctx context.Context, workspaceID uint, lookup Lookup) (Snapshot, error) {
	lookup.Reference = strings.TrimSpace(lookup.Reference)
	if workspaceID == 0 || (lookup.ID == 0 && lookup.Reference == "") {
		return Snapshot{}, ErrReadQueryInvalid
	}
	return service.store.GetTask(ctx, workspaceID, lookup)
}
