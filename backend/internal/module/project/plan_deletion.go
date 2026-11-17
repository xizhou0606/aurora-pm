package project

import (
	"context"
	"errors"
)

type PlanDeletionImpact struct {
	RequirementCount int64
	WorkflowHistory  bool
	AIRunCount       int64
}

func (impact PlanDeletionImpact) Blocked() bool {
	return impact.RequirementCount > 0 || impact.WorkflowHistory || impact.AIRunCount > 0
}

type PlanDeletionInspector interface {
	InspectPlanUnitDeletion(context.Context, uint, uint) (PlanDeletionImpact, error)
}
type PlanWorkflowCleaner interface {
	DeletePlanUnitScaffolding(context.Context, uint, uint) error
}

type PlanDeletionSaga struct {
	WorkspaceID, ProjectSetID, PlanUnitID uint
	ApplyKey, InputDigest, Status         string
	NextStep                              int
	Result                                PlanMutationResult
}
type PlanDeletionStore interface {
	FindPlanDeletion(context.Context, string) (PlanDeletionSaga, bool, error)
	BeginPlanDeletion(context.Context, PlanMutation) (PlanDeletionSaga, error)
	RejectPlanDeletion(context.Context, string) error
	AdvancePlanDeletion(context.Context, string, int) (PlanDeletionSaga, error)
	CompletePlanDeletion(context.Context, string) (PlanMutationResult, error)
}

type PlanDeletionService struct {
	store    PlanDeletionStore
	impact   PlanDeletionInspector
	workflow PlanWorkflowCleaner
}

func NewPlanDeletionService(store PlanDeletionStore, impact PlanDeletionInspector, workflow PlanWorkflowCleaner) (*PlanDeletionService, error) {
	if store == nil || impact == nil || workflow == nil {
		return nil, ErrMutationInvalid
	}
	return &PlanDeletionService{store: store, impact: impact, workflow: workflow}, nil
}
func (service *PlanDeletionService) ApplyPlanUnitDeletion(ctx context.Context, command PlanMutation) (PlanMutationResult, error) {
	if service == nil || command.Operation != PlanDelete || command.WorkspaceID == 0 || command.ID == 0 || command.ApplyKey == "" || len(command.InputDigest) != 64 {
		return PlanMutationResult{}, ErrMutationInvalid
	}
	saga, found, err := service.store.FindPlanDeletion(ctx, command.ApplyKey)
	if err != nil {
		return PlanMutationResult{}, ErrMutationRetryable
	}
	if !found {
		saga, err = service.store.BeginPlanDeletion(ctx, command)
		if err != nil {
			return PlanMutationResult{}, retryableDeletionError(err)
		}
	} else if saga.WorkspaceID != command.WorkspaceID || saga.PlanUnitID != command.ID || saga.InputDigest != command.InputDigest {
		return PlanMutationResult{}, ErrMutationConflict
	}
	if saga.NextStep == 0 {
		impact, err := service.impact.InspectPlanUnitDeletion(ctx, saga.WorkspaceID, saga.PlanUnitID)
		if err != nil {
			return PlanMutationResult{}, ErrMutationRetryable
		}
		if impact.Blocked() {
			_ = service.store.RejectPlanDeletion(ctx, saga.ApplyKey)
			return PlanMutationResult{}, ErrMutationConflict
		}
		saga, err = service.store.AdvancePlanDeletion(ctx, saga.ApplyKey, 0)
		if err != nil {
			return PlanMutationResult{}, ErrMutationRetryable
		}
	}
	if saga.NextStep == 1 {
		if err := service.workflow.DeletePlanUnitScaffolding(ctx, saga.WorkspaceID, saga.PlanUnitID); err != nil {
			return PlanMutationResult{}, ErrMutationRetryable
		}
		saga, err = service.store.AdvancePlanDeletion(ctx, saga.ApplyKey, 1)
		if err != nil {
			return PlanMutationResult{}, ErrMutationRetryable
		}
	}
	result, err := service.store.CompletePlanDeletion(ctx, saga.ApplyKey)
	if err != nil {
		if errors.Is(err, ErrMutationConflict) || errors.Is(err, ErrMutationNotFound) {
			return PlanMutationResult{}, err
		}
		return PlanMutationResult{}, ErrMutationRetryable
	}
	return result, nil
}
