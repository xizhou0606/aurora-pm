package workflow

import (
	"context"
	"errors"
)

var ErrProjectDeletionInvalid = errors.New("workflow project deletion is invalid")

type ProjectDeletionStore interface {
	DeleteProjectSubjects(context.Context, uint, ProjectSubjects) error
	PlanUnitHasExecutionHistory(context.Context, uint, uint) (bool, error)
	DeletePlanUnitScaffolding(context.Context, uint, uint) error
	SubjectHasExecutionHistory(context.Context, uint, string, uint) (bool, error)
	DeleteSubjectScaffolding(context.Context, uint, string, uint) error
}

func (service *ProjectDeletionService) PlanUnitHasExecutionHistory(ctx context.Context, workspaceID, planUnitID uint) (bool, error) {
	if service == nil || service.store == nil || workspaceID == 0 || planUnitID == 0 {
		return false, ErrProjectDeletionInvalid
	}
	return service.store.PlanUnitHasExecutionHistory(ctx, workspaceID, planUnitID)
}
func (service *ProjectDeletionService) DeletePlanUnitScaffolding(ctx context.Context, workspaceID, planUnitID uint) error {
	if service == nil || service.store == nil || workspaceID == 0 || planUnitID == 0 {
		return ErrProjectDeletionInvalid
	}
	return service.store.DeletePlanUnitScaffolding(ctx, workspaceID, planUnitID)
}
func (service *ProjectDeletionService) SubjectHasExecutionHistory(ctx context.Context, workspaceID uint, subjectType string, subjectID uint) (bool, error) {
	if service == nil || service.store == nil || workspaceID == 0 || subjectID == 0 {
		return false, ErrProjectDeletionInvalid
	}
	return service.store.SubjectHasExecutionHistory(ctx, workspaceID, subjectType, subjectID)
}
func (service *ProjectDeletionService) DeleteSubjectScaffolding(ctx context.Context, workspaceID uint, subjectType string, subjectID uint) error {
	if service == nil || service.store == nil || workspaceID == 0 || subjectID == 0 {
		return ErrProjectDeletionInvalid
	}
	return service.store.DeleteSubjectScaffolding(ctx, workspaceID, subjectType, subjectID)
}

type ProjectDeletionService struct{ store ProjectDeletionStore }

func NewProjectDeletionService(store ProjectDeletionStore) (*ProjectDeletionService, error) {
	if store == nil {
		return nil, ErrProjectDeletionInvalid
	}
	return &ProjectDeletionService{store: store}, nil
}

func (service *ProjectDeletionService) DeleteProjectSubjects(ctx context.Context, workspaceID uint, subjects ProjectSubjects) error {
	if service == nil || service.store == nil || workspaceID == 0 {
		return ErrProjectDeletionInvalid
	}
	return service.store.DeleteProjectSubjects(ctx, workspaceID, subjects)
}
