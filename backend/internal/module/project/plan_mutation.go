package project

import (
	"context"
	"strings"
	"time"
)

type PlanMutationOperation string

const (
	PlanCreate PlanMutationOperation = "plan_unit.create"
	PlanUpdate PlanMutationOperation = "plan_unit.update"
	PlanDelete PlanMutationOperation = "plan_unit.delete"
)

type PlanMutation struct {
	Operation                            PlanMutationOperation
	WorkspaceID, ActorUserID             uint
	ApplyKey, InputDigest                string
	ID, ProjectSetID, WorkflowTemplateID uint
	Title                                string
	TitleSet                             bool
	Type                                 string
	TypeSet                              bool
	Goal                                 string
	GoalSet                              bool
	OwnerUserID                          *uint
	OwnerSet                             bool
	PlannedStartAt                       *time.Time
	PlannedStartSet                      bool
	PlannedEndAt                         *time.Time
	PlannedEndSet                        bool
	RiskLevel                            string
	RiskLevelSet                         bool
}

type PlanMutationResult struct {
	ID             uint       `json:"id"`
	WorkspaceID    uint       `json:"workspace_id"`
	TeamID         uint       `json:"team_id"`
	ProjectSetID   uint       `json:"project_set_id"`
	Title          string     `json:"title"`
	Type           string     `json:"type"`
	Goal           string     `json:"goal"`
	OwnerUserID    *uint      `json:"owner_user_id"`
	PlannedStartAt *time.Time `json:"planned_start_at"`
	PlannedEndAt   *time.Time `json:"planned_end_at"`
	RiskLevel      string     `json:"risk_level"`
	Status         string     `json:"status"`
	Deleted        bool       `json:"deleted,omitempty"`
}

type PlanMutationStore interface {
	ApplyPlanUnit(context.Context, PlanMutation) (PlanMutationResult, error)
}

type PlanScheduleGuard interface {
	ValidatePlanUnitSchedule(context.Context, uint, uint, *time.Time, *time.Time) error
}

type PlanMutationService struct {
	store      PlanMutationStore
	references ProjectReferenceValidator
}

func NewPlanMutationService(store PlanMutationStore, references ProjectReferenceValidator) (*PlanMutationService, error) {
	if store == nil || references == nil {
		return nil, ErrMutationInvalid
	}
	return &PlanMutationService{store: store, references: references}, nil
}

func (service *PlanMutationService) ApplyPlanUnit(ctx context.Context, command PlanMutation) (PlanMutationResult, error) {
	command.Title, command.Type, command.Goal, command.RiskLevel = strings.TrimSpace(command.Title), strings.TrimSpace(command.Type), strings.TrimSpace(command.Goal), strings.TrimSpace(command.RiskLevel)
	if service == nil || service.store == nil || service.references == nil || command.WorkspaceID == 0 || command.ActorUserID == 0 || command.ApplyKey == "" || len(command.InputDigest) != 64 {
		return PlanMutationResult{}, ErrMutationInvalid
	}
	users := []uint{}
	switch command.Operation {
	case PlanUpdate:
		if command.ID == 0 || command.ProjectSetID != 0 || command.WorkflowTemplateID != 0 || command.TitleSet && (command.Title == "" || len(command.Title) > 160) ||
			command.TypeSet && command.Type != "project" && command.Type != "version" {
			return PlanMutationResult{}, ErrMutationInvalid
		}
		if command.PlannedStartAt != nil && command.PlannedEndAt != nil && command.PlannedStartAt.After(*command.PlannedEndAt) {
			return PlanMutationResult{}, ErrMutationInvalid
		}
		if !command.TitleSet && !command.TypeSet && !command.GoalSet && !command.OwnerSet && !command.RiskLevelSet && !command.PlannedStartSet && !command.PlannedEndSet {
			return PlanMutationResult{}, ErrMutationInvalid
		}
		if command.OwnerSet && command.OwnerUserID != nil {
			users = append(users, *command.OwnerUserID)
		}
	default:
		return PlanMutationResult{}, ErrMutationInvalid
	}
	if len(users) > 0 {
		valid, err := service.references.ProjectReferencesValid(ctx, command.WorkspaceID, 0, users)
		if err != nil {
			return PlanMutationResult{}, err
		}
		if !valid {
			return PlanMutationResult{}, ErrProjectReferenceInvalid
		}
	}
	return service.store.ApplyPlanUnit(ctx, command)
}
