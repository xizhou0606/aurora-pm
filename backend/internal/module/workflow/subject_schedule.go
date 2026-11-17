package workflow

import (
	"context"
	"errors"
	"time"
)

var ErrSubjectScheduleConflict = errors.New("workflow subject schedule conflicts with boundary")

type SubjectScheduleStore interface {
	ValidateSubjectSchedules(context.Context, uint, uint, []uint, *time.Time, *time.Time) error
	RequirementScheduleBounds(context.Context, uint, uint) (*time.Time, *time.Time, bool, error)
}

type SubjectScheduleService struct{ store SubjectScheduleStore }

func NewSubjectScheduleService(store SubjectScheduleStore) (*SubjectScheduleService, error) {
	if store == nil {
		return nil, ErrSubjectScheduleConflict
	}
	return &SubjectScheduleService{store: store}, nil
}

func (service *SubjectScheduleService) ValidateSubjectSchedules(ctx context.Context, workspaceID, planUnitID uint, requirementIDs []uint, start, end *time.Time) error {
	if service == nil || service.store == nil || workspaceID == 0 || planUnitID == 0 || start != nil && end != nil && start.After(*end) {
		return ErrSubjectScheduleConflict
	}
	return service.store.ValidateSubjectSchedules(ctx, workspaceID, planUnitID, requirementIDs, start, end)
}

func (service *SubjectScheduleService) RequirementScheduleBounds(ctx context.Context, workspaceID, requirementID uint) (*time.Time, *time.Time, bool, error) {
	if service == nil || service.store == nil || workspaceID == 0 || requirementID == 0 {
		return nil, nil, false, ErrSubjectScheduleConflict
	}
	return service.store.RequirementScheduleBounds(ctx, workspaceID, requirementID)
}
