package project

import (
	"context"
	"errors"
	"time"
)

var (
	ErrReadQueryInvalid   = errors.New("project read query is invalid")
	ErrProjectSetNotFound = errors.New("project set not found")
	ErrPlanUnitNotFound   = errors.New("plan unit not found")
)

type ProjectSetSnapshot struct {
	ID             uint       `json:"id"`
	WorkspaceID    uint       `json:"workspace_id"`
	TeamID         uint       `json:"team_id"`
	Name           string     `json:"name"`
	Code           string     `json:"code,omitempty"`
	Description    string     `json:"description,omitempty"`
	OwnerUserID    *uint      `json:"owner_user_id,omitempty"`
	PlannedStartAt *time.Time `json:"starts_at,omitempty"`
	PlannedEndAt   *time.Time `json:"due_at,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type PlanUnitSnapshot struct {
	ID                    uint       `json:"id"`
	WorkspaceID           uint       `json:"workspace_id"`
	TeamID                uint       `json:"team_id"`
	ProjectSetID          uint       `json:"project_new_id"`
	Title                 string     `json:"title"`
	Type                  string     `json:"type"`
	Goal                  string     `json:"goal,omitempty"`
	OwnerUserID           *uint      `json:"owner_user_id,omitempty"`
	PlannedStartAt        *time.Time `json:"starts_at,omitempty"`
	PlannedEndAt          *time.Time `json:"due_at,omitempty"`
	DesignFreezeDueAt     *time.Time `json:"design_freeze_due_at,omitempty"`
	DevelopmentCompleteAt *time.Time `json:"development_complete_due_at,omitempty"`
	TestAcceptanceDueAt   *time.Time `json:"test_acceptance_due_at,omitempty"`
	ActualStartAt         *time.Time `json:"actual_start_at,omitempty"`
	ActualCompletedAt     *time.Time `json:"actual_completed_at,omitempty"`
	RiskLevel             string     `json:"risk_level,omitempty"`
	Status                string     `json:"status"`
	SortOrder             int        `json:"sort_order"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type ReadStore interface {
	GetProjectSet(context.Context, uint, uint) (ProjectSetSnapshot, error)
	GetPlanUnit(context.Context, uint, uint) (PlanUnitSnapshot, error)
}

type ReadService struct{ store ReadStore }

func NewReadService(store ReadStore) (*ReadService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ReadService{store: store}, nil
}

func (service *ReadService) GetProjectSet(ctx context.Context, workspaceID, id uint) (ProjectSetSnapshot, error) {
	if workspaceID == 0 || id == 0 {
		return ProjectSetSnapshot{}, ErrReadQueryInvalid
	}
	return service.store.GetProjectSet(ctx, workspaceID, id)
}

func (service *ReadService) GetPlanUnit(ctx context.Context, workspaceID, id uint) (PlanUnitSnapshot, error) {
	if workspaceID == 0 || id == 0 {
		return PlanUnitSnapshot{}, ErrReadQueryInvalid
	}
	return service.store.GetPlanUnit(ctx, workspaceID, id)
}
