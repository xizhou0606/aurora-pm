package project

import (
	"context"
	"errors"
)

var ErrMutationRetryable = errors.New("project mutation can be retried")

type DeletionStep int

const (
	DeletionPreflight DeletionStep = iota
	DeletionWorkflow
	DeletionAI
	DeletionQuality
	DeletionTask
	DeletionRequirement
	DeletionProject
	DeletionDone
)

type DeletionSaga struct {
	WorkspaceID    uint
	ProjectSetID   uint
	ApplyKey       string
	InputDigest    string
	SubjectDigest  string
	PlanUnitIDs    []uint
	RequirementIDs []uint
	NextStep       DeletionStep
	Status         string
	Result         SetMutationResult
}

type DeletionSagaStore interface {
	FindProjectDeletion(context.Context, string) (DeletionSaga, bool, error)
	BeginProjectDeletion(context.Context, SetMutation, DeletionImpact) (DeletionSaga, error)
	RefreshProjectDeletionImpact(context.Context, string, DeletionImpact) (DeletionSaga, error)
	AdvanceProjectDeletion(context.Context, string, DeletionStep) (DeletionSaga, error)
	CompleteProjectDeletion(context.Context, string) (SetMutationResult, error)
}

type DeletionStepRunner interface {
	RunProjectDeletionStep(context.Context, DeletionStep, DeletionSaga) error
}

type DeletionService struct {
	store  DeletionSagaStore
	impact DeletionImpactInspector
	steps  DeletionStepRunner
}

func NewDeletionService(store DeletionSagaStore, impact DeletionImpactInspector, steps DeletionStepRunner) (*DeletionService, error) {
	if store == nil || impact == nil || steps == nil {
		return nil, ErrMutationInvalid
	}
	return &DeletionService{store: store, impact: impact, steps: steps}, nil
}

func (service *DeletionService) ApplyProjectSetDeletion(
	ctx context.Context,
	command SetMutation,
) (SetMutationResult, error) {
	if service == nil || service.store == nil || service.impact == nil || service.steps == nil ||
		command.Operation != SetDelete || command.WorkspaceID == 0 || command.ID == 0 ||
		command.ApplyKey == "" || len(command.InputDigest) != 64 {
		return SetMutationResult{}, ErrMutationInvalid
	}
	saga, found, err := service.store.FindProjectDeletion(ctx, command.ApplyKey)
	if err != nil {
		return SetMutationResult{}, retryableDeletionError(err)
	}
	if found {
		if saga.WorkspaceID != command.WorkspaceID || saga.ProjectSetID != command.ID || saga.InputDigest != command.InputDigest {
			return SetMutationResult{}, ErrMutationConflict
		}
	} else {
		impact, err := service.impact.InspectProjectDeletionImpact(ctx, command.WorkspaceID, command.ID)
		if err != nil {
			return SetMutationResult{}, retryableDeletionError(err)
		}
		saga, err = service.store.BeginProjectDeletion(ctx, command, impact)
		if err != nil {
			return SetMutationResult{}, retryableDeletionError(err)
		}
	}
	for saga.NextStep < DeletionProject {
		step := saga.NextStep
		if step == DeletionPreflight {
			impact, err := service.impact.InspectProjectDeletionImpact(ctx, saga.WorkspaceID, saga.ProjectSetID)
			if err != nil {
				return SetMutationResult{}, retryableDeletionError(err)
			}
			saga, err = service.store.RefreshProjectDeletionImpact(ctx, saga.ApplyKey, impact)
			if err != nil {
				return SetMutationResult{}, retryableDeletionError(err)
			}
		} else {
			if err := service.steps.RunProjectDeletionStep(ctx, step, saga); err != nil {
				return SetMutationResult{}, retryableDeletionError(err)
			}
		}
		saga, err = service.store.AdvanceProjectDeletion(ctx, saga.ApplyKey, step)
		if err != nil {
			return SetMutationResult{}, retryableDeletionError(err)
		}
	}
	result, err := service.store.CompleteProjectDeletion(ctx, saga.ApplyKey)
	if err != nil {
		return SetMutationResult{}, retryableDeletionError(err)
	}
	return result, nil
}

func retryableDeletionError(err error) error {
	if errors.Is(err, ErrMutationInvalid) || errors.Is(err, ErrMutationNotFound) || errors.Is(err, ErrMutationConflict) {
		return err
	}
	return ErrMutationRetryable
}
