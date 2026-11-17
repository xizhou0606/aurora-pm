package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"encoding/json"
	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	audit "github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type workflowToolSubjectResolver struct {
	projects     *project.ReadService
	requirements *requirement.ReadService
}

func (resolver workflowToolSubjectResolver) ResolveWorkflowSubject(
	ctx context.Context,
	workspaceID uint,
	subjectType string,
	subjectID uint,
) (workflow.SubjectContext, error) {
	switch subjectType {
	case "plan_unit":
		unit, err := resolver.projects.GetPlanUnit(ctx, workspaceID, subjectID)
		if err != nil || unit.WorkspaceID != workspaceID || unit.ProjectSetID == 0 {
			return workflow.SubjectContext{}, workflow.ErrMutationConflict
		}
		return workflow.SubjectContext{
			ProjectSetID: unit.ProjectSetID, PlanUnitID: unit.ID, TeamID: unit.TeamID,
			SubjectID: unit.ID, SubjectType: "plan_unit", SubjectTitle: unit.Title,
		}, nil
	case "requirement":
		item, err := resolver.requirements.GetRequirement(ctx, workspaceID, subjectID)
		if err != nil || item.WorkspaceID != workspaceID || item.ProjectSetID == 0 {
			return workflow.SubjectContext{}, workflow.ErrMutationConflict
		}
		result := workflow.SubjectContext{
			ProjectSetID: item.ProjectSetID, TeamID: item.TeamID,
			SubjectID: item.ID, SubjectType: "requirement", SubjectTitle: item.Title,
		}
		if item.PlanUnitID != nil {
			result.PlanUnitID = *item.PlanUnitID
		}
		return result, nil
	default:
		return workflow.SubjectContext{}, workflow.ErrMutationConflict
	}
}

type workflowToolUserVerifier struct {
	memberships *identity.WorkspaceMembershipService
}

func (verifier workflowToolUserVerifier) VerifyActiveWorkspaceUsers(
	ctx context.Context,
	workspaceID uint,
	userIDs []uint,
) error {
	if verifier.memberships == nil || workspaceID == 0 {
		return workflow.ErrMutationUnavailable
	}
	for _, userID := range userIDs {
		if userID == 0 || verifier.memberships.VerifyWorkspaceMembership(ctx, userID, workspaceID) != nil {
			return workflow.ErrMutationConflict
		}
	}
	return nil
}

type workflowToolReportWriter struct{ reports *quality.TestReportService }

type workflowToolScheduleWriter struct {
	schedules *project.WorkflowScheduleService
}

type workflowToolActionEffects struct {
	effects *audit.MutationEffectService
	tx      *gorm.DB
}

type workflowToolAIActions struct{ actions *ai.WorkflowActionService }

func (service workflowToolAIActions) PrepareWorkflowAIAction(
	ctx context.Context,
	request workflow.AIActionRequest,
) (workflow.AIActionResult, error) {
	if service.actions == nil {
		return workflow.AIActionResult{}, workflow.ErrMutationUnavailable
	}
	result, err := service.actions.PrepareWorkflowAction(ctx, ai.WorkflowActionCommand{
		WorkspaceID: request.WorkspaceID, PrincipalID: request.PrincipalID, RunID: request.RunID,
		Capability: request.Capability, Action: request.Action,
	})
	if err != nil {
		return workflow.AIActionResult{}, workflow.ErrMutationConflict
	}
	return workflow.AIActionResult{RunID: result.RunID, Path: result.Path}, nil
}

type workflowToolDesignWriter struct {
	designs *requirement.WorkflowDesignService
}

func (writer workflowToolDesignWriter) AttachWorkflowDesign(
	ctx context.Context,
	workspaceID, requirementID uint,
	rawURL string,
) error {
	if writer.designs == nil {
		return workflow.ErrMutationUnavailable
	}
	if err := writer.designs.AttachWorkflowDesign(ctx, workspaceID, requirementID, rawURL); err != nil {
		return workflow.ErrMutationConflict
	}
	return nil
}

func (writer workflowToolActionEffects) RecordWorkflowAction(ctx context.Context, effect workflow.ActionEffect) error {
	if writer.effects == nil {
		return workflow.ErrMutationUnavailable
	}
	if err := writer.effects.RecordMutationEffect(ctx, audit.MutationEffect{
		WorkspaceID: effect.WorkspaceID, PrincipalID: effect.PrincipalID, ProjectSetID: effect.ProjectSetID,
		Resource: "workflow_instance", ResourceID: effect.InstanceID, Action: effect.Action,
		Target: effect.NodeID, EventType: "pm.workflow_instance." + effect.Action,
		Payload: map[string]any{
			"node_id": effect.NodeID, "status": effect.Status, "apply_key": effect.ApplyKey,
		},
	}); err != nil {
		return err
	}
	// Requirement evidence changed (action or rollback): ask the delivery
	// consumer to re-evaluate the scheduled plan unit. Best effort facts —
	// an unscheduled requirement simply has no unit to reconcile.
	return writer.recordRequirementReconcile(ctx, effect)
}

// recordRequirementReconcile writes the pm.delivery.reconcile_requested event
// when the affected workflow belongs to a requirement subject.
func (writer workflowToolActionEffects) recordRequirementReconcile(ctx context.Context, effect workflow.ActionEffect) error {
	if writer.tx == nil {
		return nil
	}
	var instance workflowgorm.WorkflowInstance
	if err := writer.tx.WithContext(ctx).Select("subject_type", "subject_id").
		Where("id = ? AND workspace_id = ?", effect.InstanceID, effect.WorkspaceID).
		First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if instance.SubjectType != "requirement" {
		return nil
	}
	var row requirementgorm.Requirement
	if err := writer.tx.WithContext(ctx).Select("version_task_new_id").
		Where("id = ? AND workspace_id = ?", instance.SubjectID, effect.WorkspaceID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if row.PlanUnitID == 0 {
		return nil
	}
	return writer.RecordDeliveryReconcile(ctx, effect.WorkspaceID, row.PlanUnitID, "requirement_node_action:"+effect.Action)
}

// RecordDeliveryReconcile writes the dedicated delivery outbox event on the
// adapter's transaction, matching the HTTP path's semantics.
func (writer workflowToolActionEffects) RecordDeliveryReconcile(ctx context.Context, workspaceID, planUnitID uint, reason string) error {
	if writer.tx == nil || workspaceID == 0 || planUnitID == 0 {
		return nil
	}
	payload, err := json.Marshal(map[string]any{"plan_unit_id": planUnitID, "reason": reason})
	if err != nil {
		return err
	}
	return writer.tx.WithContext(ctx).Create(&auditgorm.OutboxEvent{
		WorkspaceID: workspaceID, AggregateType: "plan_unit", AggregateID: planUnitID,
		AggregateRef: fmt.Sprintf("plan_unit:%d", planUnitID),
		EventType:    workflow.DeliveryReconcileEventType, Payload: string(payload),
		OccurredAt: time.Now().UTC(),
	}).Error
}

// workflowToolIntakeLocks bridges the workflow mutation transaction to the
// project-module plan-unit lock row. Expected plan-unit conditions map to
// mutation conflicts so callers never see them as retryable failures.
type workflowToolIntakeLocks struct{ tx *gorm.DB }

func (store workflowToolIntakeLocks) RequirementIntakeLocked(
	ctx context.Context,
	workspaceID, planUnitID uint,
) (bool, error) {
	if workspaceID == 0 || planUnitID == 0 {
		return false, workflow.ErrMutationConflict
	}
	var unit projectgorm.PlanUnit
	if err := store.tx.WithContext(ctx).Select("requirements_locked_at").
		Where("workspace_id = ? AND id = ?", workspaceID, planUnitID).First(&unit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, workflow.ErrMutationConflict
		}
		return false, err
	}
	return unit.RequirementsLockedAt != nil, nil
}

func (store workflowToolIntakeLocks) MarkRequirementIntakeLocked(
	ctx context.Context,
	workspaceID, planUnitID, lockedByUserID uint,
	lockedAt time.Time,
) (bool, error) {
	locked, err := projectgorm.MarkPlanUnitRequirementsLockedTx(store.tx.WithContext(ctx), workspaceID, planUnitID, lockedByUserID, lockedAt)
	if errors.Is(err, projectgorm.ErrPlanUnitMutationInProgress) {
		return false, workflow.ErrMutationConflict
	}
	return locked, err
}

type workflowToolIntakeEffects struct{ effects *audit.MutationEffectService }

// workflowToolPlanRequirements reads the current requirement scope of a plan
// unit on the workflow mutation transaction.
type workflowToolPlanRequirements struct{ tx *gorm.DB }

func (reader workflowToolPlanRequirements) PlanUnitRequirementRefs(
	ctx context.Context,
	workspaceID, planUnitID uint,
) ([]workflow.PlanRequirementRef, error) {
	raw, err := requirementgorm.PlanUnitRequirementWorkflowRefs(reader.tx.WithContext(ctx), workspaceID, planUnitID)
	if err != nil {
		return nil, err
	}
	refs := make([]workflow.PlanRequirementRef, 0, len(raw))
	for _, ref := range raw {
		item := workflow.PlanRequirementRef{RequirementID: ref.RequirementID, Status: ref.Status}
		if ref.WorkflowInstanceID != nil {
			item.WorkflowInstanceID = *ref.WorkflowInstanceID
		}
		if ref.TemplateVersionID != nil {
			item.TemplateVersionID = *ref.TemplateVersionID
		}
		refs = append(refs, item)
	}
	return refs, nil
}

func (writer workflowToolIntakeEffects) RecordRequirementIntakeLocked(
	ctx context.Context,
	effect workflow.RequirementIntakeEffect,
) error {
	if writer.effects == nil {
		return workflow.ErrMutationUnavailable
	}
	return writer.effects.RecordMutationEffect(ctx, audit.MutationEffect{
		WorkspaceID: effect.WorkspaceID, PrincipalID: effect.PrincipalID, ProjectSetID: effect.ProjectSetID,
		Resource: "plan_unit", ResourceID: effect.PlanUnitID,
		ResourceRef: fmt.Sprintf("plan_unit:%d", effect.PlanUnitID),
		Action:      "requirements_locked", Target: effect.NodeName,
		EventType: "pm.plan_unit.requirements_locked",
		Payload: map[string]any{
			"plan_unit_id": effect.PlanUnitID, "workflow_instance_id": effect.InstanceID,
			"node_id": effect.NodeID, "locked_at": effect.LockedAt,
			"policy": "new_requirements_only",
		},
	})
}

func (writer workflowToolScheduleWriter) ConfirmWorkflowSchedule(
	ctx context.Context,
	workspaceID uint,
	subject workflow.SubjectContext,
	startAt, endAt *time.Time,
) error {
	if writer.schedules == nil {
		return workflow.ErrMutationUnavailable
	}
	return writer.schedules.ConfirmWorkflowSchedule(ctx, project.WorkflowScheduleCommand{
		WorkspaceID: workspaceID, ProjectSetID: subject.ProjectSetID, PlanUnitID: subject.PlanUnitID,
		SuggestedStartAt: startAt, SuggestedEndAt: endAt,
	})
}

func (writer workflowToolReportWriter) CreateWorkflowTestReportLink(
	_ context.Context,
	workspaceID uint,
	subject workflow.SubjectContext,
	principalID uint,
	applyKey string,
	name string,
	rawURL string,
) (workflow.TestReportLink, error) {
	if writer.reports == nil || workspaceID == 0 || subject.ProjectSetID == 0 || subject.PlanUnitID == 0 ||
		principalID == 0 || strings.TrimSpace(applyKey) == "" {
		return workflow.TestReportLink{}, workflow.ErrMutationInvalid
	}
	report, err := writer.reports.Create(quality.TestReportInput{
		WorkspaceID: workspaceID, PlanUnitID: subject.PlanUnitID, ProjectSetID: subject.ProjectSetID,
		Kind: quality.TestReportKindLink, Name: name, ContentType: "text/uri-list",
		ObjectKey: fmt.Sprintf("link/workflow-tool/%d/%d/%s", workspaceID, subject.PlanUnitID, applyKey),
		URL:       rawURL, UploaderUserID: principalID,
	})
	if err != nil {
		return workflow.TestReportLink{}, err
	}
	return workflow.TestReportLink{
		ID: report.ID, WorkspaceID: report.WorkspaceID, PlanUnitID: report.PlanUnitID,
		ProjectSetID: report.ProjectSetID, Kind: report.Kind, Name: report.Name, URL: report.URL,
		UploaderUserID: report.UploaderUserID, CreatedAt: report.CreatedAt,
	}, nil
}

var _ workflow.SubjectResolver = workflowToolSubjectResolver{}
var _ workflow.UserVerifier = workflowToolUserVerifier{}
var _ workflow.TestReportLinkWriter = workflowToolReportWriter{}
var _ workflow.ScheduleWriter = workflowToolScheduleWriter{}
var _ workflow.ActionEffectWriter = workflowToolActionEffects{}
var _ workflow.AIActionService = workflowToolAIActions{}
var _ workflow.RequirementDesignWriter = workflowToolDesignWriter{}
var _ workflow.RequirementIntakeLockStore = workflowToolIntakeLocks{}
var _ workflow.RequirementIntakeEffectWriter = workflowToolIntakeEffects{}
var _ workflow.PlanRequirementScopeReader = workflowToolPlanRequirements{}
