package workflowgorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MutationDependencies struct {
	Subjects    workflow.SubjectResolver
	Users       workflow.UserVerifier
	Reports     workflow.TestReportLinkWriter
	Schedule    workflow.ScheduleWriter
	Effects     workflow.ActionEffectWriter
	Assignments workflow.SubjectAssignmentWriter
	Invitations workflow.PreReviewInvitationWriter
	AI          workflow.AIActionService
	Designs     workflow.RequirementDesignWriter
	// IntakeLocks and IntakeEffects back the project requirement-intake lock.
	// They stay optional so older wiring keeps compiling; the intake node
	// completion fails with ErrMutationUnavailable when they are missing.
	IntakeLocks   workflow.RequirementIntakeLockStore
	IntakeEffects workflow.RequirementIntakeEffectWriter
	// PlanRequirements backs requirements_guard evaluation on the mutation
	// path; guarded node actions fail with ErrMutationUnavailable when nil.
	PlanRequirements workflow.PlanRequirementScopeReader
}

type MutationDependencyFactory func(*gorm.DB) (MutationDependencies, error)

type MutationRepository struct {
	db           *gorm.DB
	dependencies MutationDependencyFactory
}

func NewMutationRepository(db *gorm.DB, dependencies MutationDependencyFactory) (*MutationRepository, error) {
	if db == nil || dependencies == nil {
		return nil, workflow.ErrMutationInvalid
	}
	return &MutationRepository{db: db, dependencies: dependencies}, nil
}

func (repository *MutationRepository) ApplyWorkflowMutation(
	ctx context.Context,
	command workflow.Mutation,
) (workflow.MutationResult, error) {
	if repository == nil || repository.db == nil || repository.dependencies == nil {
		return workflow.MutationResult{}, workflow.ErrMutationInvalid
	}
	if replay, found, err := repository.replay(ctx, command); err != nil || found {
		return replay, err
	}
	var result workflow.MutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := ToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey,
			Operation: string(command.Operation), InputDigest: command.InputDigest, ResultJSON: "",
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		dependencies, err := repository.dependencies(tx)
		if err != nil || dependencies.Subjects == nil || dependencies.Users == nil || dependencies.Reports == nil ||
			dependencies.Schedule == nil || dependencies.Effects == nil || dependencies.Assignments == nil ||
			dependencies.Invitations == nil || dependencies.AI == nil || dependencies.Designs == nil {
			return workflow.ErrMutationUnavailable
		}
		state, err := lockWorkflowMutationState(ctx, tx, dependencies, command)
		if err != nil {
			return err
		}
		switch command.Operation {
		case workflow.ActionExecute:
			result, err = applyWorkflowAction(ctx, tx, dependencies, command, state)
		case workflow.NodePlanUpdate:
			result, err = applyWorkflowNodePlans(ctx, tx, dependencies, command, state)
		case workflow.OpinionCreate:
			result, err = applyWorkflowOpinionCreate(ctx, tx, dependencies, command, state)
		case workflow.OpinionUpdate:
			result, err = applyWorkflowOpinionUpdate(ctx, tx, dependencies, command, state)
		case workflow.TestReportLinkCreate:
			result, err = applyWorkflowTestReportLink(ctx, tx, dependencies, command, state)
		default:
			err = workflow.ErrMutationInvalid
		}
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		updated := tx.Model(&ToolMutationApply{}).Where("id = ? AND result_json = ''", ledger.ID).
			Update("result_json", string(encoded))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return workflow.ErrMutationConflict
		}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replay(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return workflow.MutationResult{}, normalizeWorkflowMutationError(err)
}

type workflowMutationState struct {
	instance   WorkflowInstance
	definition workflow.Definition
	subject    workflow.SubjectContext
}

func lockWorkflowMutationState(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
) (workflowMutationState, error) {
	var instance WorkflowInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND workspace_id = ?", command.InstanceID, command.WorkspaceID).First(&instance).Error; err != nil {
		return workflowMutationState{}, normalizeWorkflowMutationError(err)
	}
	if instance.TemplateID == 0 || instance.TemplateVersionID == 0 || instance.SubjectID == 0 ||
		(instance.SubjectType != "plan_unit" && instance.SubjectType != "requirement") {
		return workflowMutationState{}, workflow.ErrMutationConflict
	}
	var version WorkflowTemplateVersion
	if err := tx.Where(
		"id = ? AND workspace_id = ? AND template_id = ?",
		instance.TemplateVersionID, instance.WorkspaceID, instance.TemplateID,
	).First(&version).Error; err != nil {
		return workflowMutationState{}, normalizeWorkflowMutationError(err)
	}
	var template WorkflowTemplate
	if err := tx.Where("id = ? AND workspace_id = ?", instance.TemplateID, instance.WorkspaceID).First(&template).Error; err != nil {
		return workflowMutationState{}, normalizeWorkflowMutationError(err)
	}
	if version.Version == 0 || template.SubjectType != instance.SubjectType || version.DefinitionHash == "" {
		return workflowMutationState{}, workflow.ErrMutationConflict
	}
	definition, err := workflow.ParseDefinition(version.Definition)
	if err != nil {
		return workflowMutationState{}, err
	}
	subject, err := dependencies.Subjects.ResolveWorkflowSubject(ctx, command.WorkspaceID, instance.SubjectType, instance.SubjectID)
	if err != nil || subject.ProjectSetID == 0 || instance.SubjectType == "plan_unit" && subject.PlanUnitID != instance.SubjectID {
		return workflowMutationState{}, workflow.ErrMutationConflict
	}
	return workflowMutationState{instance: instance, definition: definition, subject: subject}, nil
}

func (repository *MutationRepository) replay(
	ctx context.Context,
	command workflow.Mutation,
) (workflow.MutationResult, bool, error) {
	var ledger ToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ?", command.ApplyKey).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflow.MutationResult{}, false, nil
	}
	if err != nil {
		return workflow.MutationResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.Operation != string(command.Operation) ||
		ledger.InputDigest != command.InputDigest || ledger.ResultJSON == "" {
		return workflow.MutationResult{}, true, workflow.ErrMutationConflict
	}
	var result workflow.MutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return workflow.MutationResult{}, true, workflow.ErrMutationConflict
	}
	return result, true, nil
}

func normalizeWorkflowMutationError(err error) error {
	if err == nil || errors.Is(err, workflow.ErrMutationInvalid) || errors.Is(err, workflow.ErrMutationNotFound) ||
		errors.Is(err, workflow.ErrMutationConflict) || errors.Is(err, workflow.ErrMutationUnavailable) ||
		errors.Is(err, workflow.ErrRequirementLockConfirmationRequired) || errors.Is(err, workflow.ErrRequirementsGuardBlocked) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflow.ErrMutationNotFound
	}
	return fmt.Errorf("workflow mutation store: %w", err)
}

var _ workflow.MutationStore = (*MutationRepository)(nil)
