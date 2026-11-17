package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementcover "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

type requirementUserValidator struct {
	references *identity.ProjectReferenceService
}

func (value requirementUserValidator) ValidateRequirementUsers(ctx context.Context, workspaceID uint, ids []uint) (bool, error) {
	return value.references.ProjectReferencesValid(ctx, workspaceID, 0, ids)
}

type requirementPlanScopes struct {
	plans *project.PlanAssignmentService
}

func (value requirementPlanScopes) ResolveRequirementPlan(ctx context.Context, workspaceID, planID uint) (requirement.PlanScope, error) {
	scope, err := value.plans.ResolvePlanAssignment(ctx, workspaceID, planID)
	if err != nil {
		return requirement.PlanScope{}, requirement.ErrMutationConflict
	}
	return requirement.PlanScope{WorkspaceID: scope.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID, PlanUnitID: scope.PlanUnitID, Type: scope.Type, RequirementsLockedAt: scope.RequirementsLockedAt, PlannedStartAt: scope.PlannedStartAt, PlannedEndAt: scope.PlannedEndAt}, nil
}

type requirementWorkflowLifecycle struct {
	starts    *workflow.SubjectStartService
	cleanup   *workflow.ProjectDeletionService
	schedules *workflow.SubjectScheduleService
	projects  *project.ReadService
	confirm   *project.WorkflowScheduleService
}

func (value requirementWorkflowLifecycle) StartRequirementWorkflow(ctx context.Context, command requirement.Mutation, subject requirement.MutationResult) (uint, string, error) {
	raw, _ := json.Marshal(subject)
	result, err := value.starts.StartSubject(ctx, workflow.SubjectStartCommand{WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ApplyKey: command.ApplyKey, InputDigest: command.InputDigest, TemplateID: command.WorkflowTemplateID, SubjectType: "requirement", SubjectID: subject.ID, SubjectVersion: subject.Version, ProjectSetID: subject.ProjectSetID, TeamID: subject.TeamID, SubjectTitle: subject.Title, SubjectSnapshot: raw})
	return result.InstanceID, result.SubjectStatus, err
}
func (value requirementWorkflowLifecycle) ValidateRequirementSchedule(ctx context.Context, workspaceID, requirementID, planID uint, confirm bool) error {
	plan, err := value.projects.GetPlanUnit(ctx, workspaceID, planID)
	if err != nil {
		return err
	}
	start, end, hasSuggestion, err := value.schedules.RequirementScheduleBounds(ctx, workspaceID, requirementID)
	if err != nil {
		return err
	}
	needsConfirmation := plan.PlannedStartAt == nil && start != nil || plan.PlannedEndAt == nil && end != nil
	if confirm && !needsConfirmation || needsConfirmation && !confirm {
		return workflow.ErrSubjectScheduleConflict
	}
	if needsConfirmation {
		if value.confirm == nil || !hasSuggestion {
			return workflow.ErrSubjectScheduleConflict
		}
		if err := value.confirm.ConfirmWorkflowSchedule(ctx, project.WorkflowScheduleCommand{
			WorkspaceID: workspaceID, ProjectSetID: plan.ProjectSetID, PlanUnitID: plan.ID,
			SuggestedStartAt: start, SuggestedEndAt: end,
		}); err != nil {
			return err
		}
		plan, err = value.projects.GetPlanUnit(ctx, workspaceID, planID)
		if err != nil {
			return err
		}
	}
	return value.schedules.ValidateSubjectSchedules(ctx, workspaceID, planID, []uint{requirementID}, plan.PlannedStartAt, plan.PlannedEndAt)
}
func (value requirementWorkflowLifecycle) RequirementHasWorkflowHistory(ctx context.Context, workspaceID, id uint) (bool, error) {
	return value.cleanup.SubjectHasExecutionHistory(ctx, workspaceID, "requirement", id)
}
func (value requirementWorkflowLifecycle) DeleteRequirementWorkflow(ctx context.Context, workspaceID, id uint) error {
	return value.cleanup.DeleteSubjectScaffolding(ctx, workspaceID, "requirement", id)
}

type requirementDependents struct {
	tasks   *task.RequirementLifecycleService
	guard   *task.CascadeDeletionGuardService
	quality *quality.RequirementLifecycleService
	ai      *ai.ProjectDeletionService
}

func (value requirementDependents) ValidateRequirementDeletion(ctx context.Context, workspaceID, id uint) error {
	if value.guard == nil {
		return requirement.ErrMutationConflict
	}
	if err := value.guard.ValidateCascadeDeletion(ctx, task.CascadeDeletionScope{WorkspaceID: workspaceID, RequirementID: id}); err != nil {
		if errors.Is(err, task.ErrCascadeDeletionInvalid) || errors.Is(err, task.ErrCascadeDeletionConflict) {
			return requirement.ErrMutationConflict
		}
		return err
	}
	return nil
}

func (value requirementDependents) MoveRequirementDependents(ctx context.Context, workspaceID, id uint, scope requirement.PlanScope) (int64, int64, error) {
	a, e := value.tasks.MoveRequirementTasks(ctx, workspaceID, id, scope.TeamID, scope.ProjectSetID, scope.PlanUnitID)
	if e != nil {
		return 0, 0, e
	}
	b, e := value.quality.MoveRequirementBugs(ctx, workspaceID, id, scope.ProjectSetID, scope.PlanUnitID)
	return a, b, e
}
func (value requirementDependents) DeleteRequirementDependents(ctx context.Context, workspaceID, id uint) (int64, int64, error) {
	a, e := value.tasks.DeleteRequirementTasks(ctx, workspaceID, id)
	if e != nil {
		if errors.Is(e, task.ErrCascadeDeletionInvalid) || errors.Is(e, task.ErrCascadeDeletionConflict) {
			return 0, 0, requirement.ErrMutationConflict
		}
		return 0, 0, e
	}
	b, e := value.quality.DeleteRequirementBugs(ctx, workspaceID, id)
	if e != nil {
		return 0, 0, e
	}
	e = value.ai.DeleteProjectSubjects(ctx, workspaceID, ai.ProjectSubjects{RequirementIDs: []uint{id}})
	return a, b, e
}

type requirementEffects struct {
	effects *audit.MutationEffectService
	tx      *gorm.DB
}

func (value requirementEffects) RecordRequirementMutation(ctx context.Context, e requirement.MutationEffect) error {
	if err := value.effects.RecordMutationEffect(ctx, audit.MutationEffect{WorkspaceID: e.WorkspaceID, PrincipalID: e.PrincipalID, ProjectSetID: e.ProjectSetID, Resource: "requirement", ResourceID: e.RequirementID, Action: e.Action, EventType: "pm.requirement." + e.Action, Payload: map[string]any{"apply_key": e.ApplyKey}}); err != nil {
		return err
	}
	// The current requirement set of the affected plan units changed: write
	// the delivery reconcile events on the same transaction.
	for _, planUnitID := range e.AffectedPlanUnitIDs {
		if planUnitID == 0 {
			continue
		}
		if err := value.RecordDeliveryReconcile(ctx, e.WorkspaceID, planUnitID, "requirement_"+e.Action); err != nil {
			return err
		}
	}
	return nil
}

// RecordDeliveryReconcile implements workflow.DeliveryReconcileWriter over
// the adapter's transaction.
func (value requirementEffects) RecordDeliveryReconcile(ctx context.Context, workspaceID, planUnitID uint, reason string) error {
	if value.tx == nil || workspaceID == 0 || planUnitID == 0 {
		return nil
	}
	payload, err := json.Marshal(map[string]any{"plan_unit_id": planUnitID, "reason": reason})
	if err != nil {
		return err
	}
	return value.tx.WithContext(ctx).Create(&auditgorm.OutboxEvent{
		WorkspaceID: workspaceID, AggregateType: "plan_unit", AggregateID: planUnitID,
		AggregateRef: fmt.Sprintf("plan_unit:%d", planUnitID),
		EventType:    workflow.DeliveryReconcileEventType, Payload: string(payload),
		OccurredAt: time.Now().UTC(),
	}).Error
}

// requirementScopeRevisions bumps the plan unit delivery scope revision via
// the project module store, inside the requirement mutation transaction.
type requirementScopeRevisions struct{ tx *gorm.DB }

func (value requirementScopeRevisions) BumpDeliveryScopeRevision(ctx context.Context, workspaceID, planUnitID uint) error {
	if value.tx == nil {
		return nil
	}
	return projectgorm.BumpPlanUnitDeliveryScopeRevisionTx(value.tx.WithContext(ctx), workspaceID, planUnitID)
}

type requirementCoverLifecycle struct {
	tx     *gorm.DB
	config config.Config
}

func (value requirementCoverLifecycle) EnqueueRequirementCover(_ context.Context, subject requirement.MutationResult) error {
	if value.tx == nil {
		return requirement.ErrMutationConflict
	}
	return requirementcover.EnqueueTx(value.tx, value.config, requirementgorm.Requirement{
		ID: subject.ID, WorkspaceID: subject.WorkspaceID, ProductDocURL: subject.ProductDocURL,
	})
}

var _ requirement.UserValidator = requirementUserValidator{}
var _ requirement.PlanScopeResolver = requirementPlanScopes{}
var _ requirement.WorkflowLifecycle = requirementWorkflowLifecycle{}
var _ requirement.DependentLifecycle = requirementDependents{}
var _ requirement.EffectWriter = requirementEffects{}
var _ requirement.CoverLifecycle = requirementCoverLifecycle{}
