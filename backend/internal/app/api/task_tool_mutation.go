package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

func newTaskToolMutationFacts(tx *gorm.DB) (taskToolMutationFacts, error) {
	if tx == nil {
		return taskToolMutationFacts{}, task.ErrCoreMutationInvalid
	}
	scopes, err := requirement.NewMutationScopeService(requirementgorm.NewMutationScopeRepository(tx))
	if err != nil {
		return taskToolMutationFacts{}, err
	}
	plans, err := project.NewPlanAssignmentService(projectgorm.NewPlanAssignmentRepository(tx))
	if err != nil {
		return taskToolMutationFacts{}, err
	}
	users, err := identity.NewProjectReferenceService(identitygorm.NewProjectReferenceRepository(tx))
	if err != nil {
		return taskToolMutationFacts{}, err
	}
	return taskToolMutationFacts{scopes: scopes, plans: plans, users: users}, nil
}

func newTaskToolMutationEffects(tx *gorm.DB, notificationConfig config.Notification) (taskToolMutationEffects, error) {
	effects, err := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(tx))
	if err != nil {
		return taskToolMutationEffects{}, err
	}
	tasks, err := task.NewReadService(taskgorm.NewReadRepository(tx))
	if err != nil {
		return taskToolMutationEffects{}, err
	}
	return taskToolMutationEffects{
		effects: effects, assignments: taskToolAssignmentWriter{tasks: tasks, config: notificationConfig, tx: tx},
	}, nil
}

func newTaskToolDependentCleaner(tx *gorm.DB) (taskToolDependentCleaner, error) {
	cleaner, err := quality.NewTaskReferenceCleanupService(qualitygorm.NewTaskReferenceCleanupRepository(tx))
	if err != nil {
		return taskToolDependentCleaner{}, err
	}
	return taskToolDependentCleaner{quality: cleaner}, nil
}

func newTaskBatchMutationDependencies(tx *gorm.DB, notificationConfig config.Notification) (taskgorm.BatchMutationDependencies, error) {
	facts, err := newTaskToolMutationFacts(tx)
	if err != nil {
		return taskgorm.BatchMutationDependencies{}, err
	}
	effects, err := newTaskToolMutationEffects(tx, notificationConfig)
	if err != nil {
		return taskgorm.BatchMutationDependencies{}, err
	}
	tasks, err := task.NewReadService(taskgorm.NewReadRepository(tx))
	if err != nil {
		return taskgorm.BatchMutationDependencies{}, err
	}
	return taskgorm.BatchMutationDependencies{
		Scopes: facts, Users: facts, Effects: effects,
		Notifications: taskToolAssignmentWriter{tasks: tasks, config: notificationConfig, tx: tx},
	}, nil
}

// taskToolMutationFacts adapts PM-owned hierarchy and identity services to the
// task mutation ports. It is constructed with repositories bound to the same
// transaction as the task mutation ledger.
type taskToolMutationFacts struct {
	scopes *requirement.MutationScopeService
	plans  *project.PlanAssignmentService
	users  *identity.ProjectReferenceService
}

func (facts taskToolMutationFacts) ResolveCoreMutationScope(
	ctx context.Context,
	workspaceID, requirementID uint,
) (task.CoreMutationScope, error) {
	scope, code, err := facts.resolveRequirementScope(ctx, workspaceID, requirementID)
	if err != nil {
		return task.CoreMutationScope{}, err
	}
	return task.CoreMutationScope{
		WorkspaceID: scope.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID,
		ProjectCode: code, PlanUnitID: scope.PlanUnitID, RequirementID: scope.RequirementID,
		RequirementVersion: scope.Version,
	}, nil
}

func (facts taskToolMutationFacts) ResolveTaskRequirementScope(
	ctx context.Context,
	workspaceID, requirementID uint,
) (task.BatchRequirementScope, error) {
	scope, code, err := facts.resolveRequirementScope(ctx, workspaceID, requirementID)
	if err != nil {
		return task.BatchRequirementScope{}, err
	}
	return task.BatchRequirementScope{
		WorkspaceID: scope.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID,
		PlanUnitID: scope.PlanUnitID, RequirementID: scope.RequirementID, ProjectCode: code,
	}, nil
}

func (facts taskToolMutationFacts) resolveRequirementScope(
	ctx context.Context,
	workspaceID, requirementID uint,
) (requirement.MutationScope, string, error) {
	if facts.scopes == nil || facts.plans == nil {
		return requirement.MutationScope{}, "", task.ErrCoreMutationInvalid
	}
	scope, err := facts.scopes.ResolveMutationScope(ctx, workspaceID, requirementID)
	if err != nil {
		if errors.Is(err, requirement.ErrMutationScopeInvalid) || errors.Is(err, requirement.ErrMutationScopeNotFound) {
			return requirement.MutationScope{}, "", task.ErrCoreMutationConflict
		}
		return requirement.MutationScope{}, "", err
	}
	assignment, err := facts.plans.ResolvePlanAssignment(ctx, workspaceID, scope.PlanUnitID)
	if err != nil {
		if errors.Is(err, project.ErrPlanAssignmentInvalid) {
			return requirement.MutationScope{}, "", task.ErrCoreMutationConflict
		}
		return requirement.MutationScope{}, "", err
	}
	if assignment.WorkspaceID != workspaceID || assignment.PlanUnitID != scope.PlanUnitID ||
		assignment.ProjectSetID != scope.ProjectSetID || assignment.TeamID != scope.TeamID || strings.TrimSpace(assignment.ProjectCode) == "" {
		return requirement.MutationScope{}, "", task.ErrCoreMutationConflict
	}
	return scope, assignment.ProjectCode, nil
}

func (facts taskToolMutationFacts) ValidateActiveTaskUsers(ctx context.Context, workspaceID uint, userIDs []uint) error {
	return facts.verifyUsers(ctx, workspaceID, userIDs, task.ErrCoreMutationConflict)
}

func (facts taskToolMutationFacts) VerifyTaskBatchUsers(ctx context.Context, workspaceID uint, userIDs []uint) error {
	return facts.verifyUsers(ctx, workspaceID, userIDs, task.ErrBatchMutationConflict)
}

func (facts taskToolMutationFacts) verifyUsers(ctx context.Context, workspaceID uint, userIDs []uint, conflict error) error {
	if facts.users == nil {
		return conflict
	}
	if err := facts.users.ValidateProjectReferences(ctx, workspaceID, 0, userIDs); err != nil {
		if errors.Is(err, identity.ErrProjectReferencesInvalid) {
			return conflict
		}
		return err
	}
	return nil
}

type taskToolMutationEffects struct {
	effects     *audit.MutationEffectService
	assignments taskToolAssignmentWriter
}

type taskToolDependentCleaner struct {
	quality *quality.TaskReferenceCleanupService
}

func (cleaner taskToolDependentCleaner) ClearTaskReferences(ctx context.Context, facts task.TaskDependentReferenceFacts) error {
	if cleaner.quality == nil {
		return task.ErrTaskDeletionInvalid
	}
	if err := cleaner.quality.ClearTaskReferences(ctx, quality.TaskReferenceCleanupFacts{
		WorkspaceID: facts.WorkspaceID, TaskIDs: facts.TaskIDs, TestExecutionIDs: facts.TestExecutionIDs,
	}); err != nil {
		if errors.Is(err, quality.ErrTaskReferenceCleanupInvalid) || errors.Is(err, quality.ErrTaskReferenceCleanupConflict) {
			return task.ErrTaskDeletionConflict
		}
		return err
	}
	return nil
}

func (writer taskToolMutationEffects) RecordCoreTaskMutation(ctx context.Context, effect task.CoreMutationEffect) error {
	if err := writer.record(ctx, effect.WorkspaceID, effect.PrincipalID, effect.ProjectSetID, effect.TaskID, effect.Action, effect.Target, effect.Payload); err != nil {
		return err
	}
	if err := writer.assignments.recordStatusChange(ctx, effect.WorkspaceID, effect.PrincipalID, effect.TaskID, effect.PreviousStatus); err != nil {
		return err
	}
	if !effect.NotifyAssignee || effect.AssigneeID == nil {
		return nil
	}
	return writer.assignments.RecordTaskAssignment(ctx, task.AssignmentNotification{
		WorkspaceID: effect.WorkspaceID, PrincipalID: effect.PrincipalID, TaskID: effect.TaskID,
		AssigneeUserID: *effect.AssigneeID, TaskKey: effect.TaskKey, Title: effect.Target,
	})
}

func (writer taskToolMutationEffects) RecordTaskBatchEffect(ctx context.Context, effect task.BatchEffect) error {
	if err := writer.assignments.recordStatusChange(ctx, effect.WorkspaceID, effect.PrincipalID, effect.TaskID, effect.PreviousStatus); err != nil {
		return err
	}
	payload := effect.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payload["apply_key"] = effect.ApplyKey
	return writer.record(ctx, effect.WorkspaceID, effect.PrincipalID, effect.ProjectSetID, effect.TaskID, effect.Action, "", payload)
}

func (writer taskToolMutationEffects) record(
	ctx context.Context,
	workspaceID, principalID, projectSetID, taskID uint,
	action, target string,
	payload map[string]any,
) error {
	if writer.effects == nil {
		return task.ErrCoreMutationInvalid
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return writer.effects.RecordMutationEffect(ctx, audit.MutationEffect{
		WorkspaceID: workspaceID, PrincipalID: principalID, ProjectSetID: projectSetID,
		Resource: "work_task", ResourceID: taskID, Action: action, Target: target,
		EventType: "pm.work_task." + action, Payload: payload,
	})
}

type taskToolAssignmentWriter struct {
	tasks  *task.ReadService
	config config.Notification
	tx     *gorm.DB
}

func (writer taskToolAssignmentWriter) RecordTaskAssignment(ctx context.Context, assignment task.AssignmentNotification) error {
	if writer.tx == nil || writer.tasks == nil {
		return task.ErrBatchMutationUnavailable
	}
	snapshot, err := writer.tasks.GetTask(ctx, assignment.WorkspaceID, task.Lookup{ID: assignment.TaskID})
	if err != nil || snapshot.WorkspaceID != assignment.WorkspaceID || snapshot.ID != assignment.TaskID ||
		snapshot.TeamID == 0 || snapshot.PlanUnitID == 0 || assignment.AssigneeUserID == 0 {
		if err != nil && !errors.Is(err, task.ErrTaskNotFound) && !errors.Is(err, task.ErrReadQueryInvalid) {
			return err
		}
		return task.ErrBatchMutationConflict
	}
	title := strings.TrimSpace(assignment.Title)
	if title == "" {
		title = strings.TrimSpace(snapshot.Title)
	}
	taskKey := strings.TrimSpace(assignment.TaskKey)
	if taskKey == "" {
		taskKey = strings.TrimSpace(snapshot.TaskKey)
	}
	users, err := toolNotificationUsers(ctx, writer.tx, assignment.WorkspaceID, assignment.PrincipalID, assignment.AssigneeUserID)
	if err != nil {
		return err
	}
	url := notification.WorkItemURL(writer.config.PublicBaseURL, assignment.TaskID)
	vars := map[string]string{
		"operator": toolNotificationUserLabel(users, assignment.PrincipalID),
		"title":    title, "taskKey": taskKey,
		"kind": task.WorkTaskKindLabel(snapshot.Kind), "priority": task.WorkTaskPriorityLabel(snapshot.Priority),
		"status": task.WorkTaskStatusLabel(snapshot.Status), "statusKey": snapshot.Status,
		"assignee": toolNotificationUserLabel(users, assignment.AssigneeUserID), "url": url,
	}
	return notification.Enqueue(writer.tx.WithContext(ctx), "work_task_notification", assignment.TaskID, notification.Intent{
		ResourceType: "work_task", ResourceID: fmt.Sprint(assignment.TaskID),
		WorkspaceID: assignment.WorkspaceID, Scene: notification.SceneWorkTaskAssigned,
		TeamID: snapshot.TeamID, ProjectID: snapshot.PlanUnitID, ActorUserID: assignment.PrincipalID,
		RecipientUserIDs: []uint{assignment.AssigneeUserID}, Vars: vars, SkipExternal: !writer.config.Enabled,
		FallbackPlainText: notification.PlainText(notification.SceneWorkTaskAssigned, vars),
	})
}

var _ task.CoreMutationFacts = taskToolMutationFacts{}
var _ task.BatchScopeResolver = taskToolMutationFacts{}
var _ task.BatchUserVerifier = taskToolMutationFacts{}
var _ task.TaskDependentCleaner = taskToolDependentCleaner{}
var _ task.CoreMutationEffectSink = taskToolMutationEffects{}
var _ task.BatchEffectWriter = taskToolMutationEffects{}
var _ task.BatchNotificationWriter = taskToolAssignmentWriter{}
