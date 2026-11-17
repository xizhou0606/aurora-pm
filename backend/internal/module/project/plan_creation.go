package project

import (
	"context"
	"errors"
	"strings"
)

type PlanCreationSaga struct {
	WorkspaceID, ProjectSetID, PlanUnitID        uint
	WorkflowTemplateID, WorkflowInstanceID       uint
	ApplyKey, InputDigest, SubjectStatus, Status string
	NextStep                                     int
	Result                                       PlanMutationResult
}

type PlanCreationStore interface {
	FindPlanCreation(context.Context, string) (PlanCreationSaga, bool, error)
	BeginPlanCreation(context.Context, PlanMutation) (PlanCreationSaga, error)
	RecordPlanCreationWorkflow(context.Context, string, uint, string) (PlanCreationSaga, error)
	CompletePlanCreation(context.Context, string) (PlanMutationResult, error)
	RejectPlanCreation(context.Context, string) error
}

type PlanWorkflowStartCommand struct {
	WorkspaceID, ActorUserID         uint
	ProjectSetID, TeamID, PlanUnitID uint
	WorkflowTemplateID               uint
	ApplyKey, InputDigest            string
	Subject                          PlanMutationResult
}

type PlanWorkflowStartResult struct {
	InstanceID    uint
	SubjectStatus string
}

type PlanWorkflowStarter interface {
	StartPlanUnitWorkflow(context.Context, PlanWorkflowStartCommand) (PlanWorkflowStartResult, error)
}

type PlanCreationService struct {
	store      PlanCreationStore
	references ProjectReferenceValidator
	workflow   PlanWorkflowStarter
}

func NewPlanCreationService(store PlanCreationStore, references ProjectReferenceValidator, workflow PlanWorkflowStarter) (*PlanCreationService, error) {
	if store == nil || references == nil || workflow == nil {
		return nil, ErrMutationInvalid
	}
	return &PlanCreationService{store: store, references: references, workflow: workflow}, nil
}

func (service *PlanCreationService) ApplyPlanUnitCreation(ctx context.Context, command PlanMutation) (PlanMutationResult, error) {
	command.Title, command.Type, command.Goal, command.RiskLevel = strings.TrimSpace(command.Title), strings.TrimSpace(command.Type), strings.TrimSpace(command.Goal), strings.TrimSpace(command.RiskLevel)
	if service == nil || command.Operation != PlanCreate || command.WorkspaceID == 0 || command.ActorUserID == 0 ||
		command.ProjectSetID == 0 || command.WorkflowTemplateID == 0 || command.ID != 0 || command.Title == "" || len(command.Title) > 160 ||
		command.ApplyKey == "" || len(command.InputDigest) != 64 || command.PlannedStartAt != nil && command.PlannedEndAt != nil && command.PlannedStartAt.After(*command.PlannedEndAt) {
		return PlanMutationResult{}, ErrMutationInvalid
	}
	if command.Type == "" {
		command.Type = "version"
	}
	if command.Type != "version" && command.Type != "project" {
		return PlanMutationResult{}, ErrMutationInvalid
	}
	if command.OwnerUserID != nil {
		valid, err := service.references.ProjectReferencesValid(ctx, command.WorkspaceID, 0, []uint{*command.OwnerUserID})
		if err != nil {
			return PlanMutationResult{}, err
		}
		if !valid {
			return PlanMutationResult{}, ErrProjectReferenceInvalid
		}
	}
	saga, found, err := service.store.FindPlanCreation(ctx, command.ApplyKey)
	if err != nil {
		return PlanMutationResult{}, ErrMutationRetryable
	}
	if !found {
		saga, err = service.store.BeginPlanCreation(ctx, command)
		if err != nil {
			return PlanMutationResult{}, retryableDeletionError(err)
		}
	} else if saga.WorkspaceID != command.WorkspaceID || saga.ProjectSetID != command.ProjectSetID ||
		saga.WorkflowTemplateID != command.WorkflowTemplateID || saga.InputDigest != command.InputDigest {
		return PlanMutationResult{}, ErrMutationConflict
	}
	if saga.Status == "succeeded" {
		return saga.Result, nil
	}
	if saga.Status != "in_progress" {
		return PlanMutationResult{}, ErrMutationConflict
	}
	if saga.NextStep == 0 {
		started, startErr := service.workflow.StartPlanUnitWorkflow(ctx, PlanWorkflowStartCommand{
			WorkspaceID: saga.WorkspaceID, ActorUserID: command.ActorUserID, ProjectSetID: saga.ProjectSetID,
			TeamID: saga.Result.TeamID, PlanUnitID: saga.PlanUnitID, WorkflowTemplateID: saga.WorkflowTemplateID,
			ApplyKey: saga.ApplyKey, InputDigest: saga.InputDigest, Subject: saga.Result,
		})
		if startErr != nil {
			if errors.Is(startErr, ErrMutationConflict) || errors.Is(startErr, ErrMutationNotFound) || errors.Is(startErr, ErrMutationInvalid) {
				_ = service.store.RejectPlanCreation(ctx, saga.ApplyKey)
				return PlanMutationResult{}, ErrMutationConflict
			}
			return PlanMutationResult{}, ErrMutationRetryable
		}
		if started.InstanceID == 0 || strings.TrimSpace(started.SubjectStatus) == "" {
			return PlanMutationResult{}, ErrMutationRetryable
		}
		saga, err = service.store.RecordPlanCreationWorkflow(ctx, saga.ApplyKey, started.InstanceID, started.SubjectStatus)
		if err != nil {
			return PlanMutationResult{}, ErrMutationRetryable
		}
	}
	result, err := service.store.CompletePlanCreation(ctx, saga.ApplyKey)
	if err != nil {
		if errors.Is(err, ErrMutationConflict) || errors.Is(err, ErrMutationNotFound) {
			return PlanMutationResult{}, err
		}
		return PlanMutationResult{}, ErrMutationRetryable
	}
	return result, nil
}
