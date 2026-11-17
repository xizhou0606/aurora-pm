package taskgorm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyCoreTaskCreate(ctx context.Context, tx *gorm.DB, facts task.CoreMutationFacts, effects task.CoreMutationEffectSink, command task.CoreMutation) (task.CoreMutationResult, error) {
	values := command.Values
	scope, err := facts.ResolveCoreMutationScope(ctx, command.WorkspaceID, values.RequirementID)
	if err != nil {
		return task.CoreMutationResult{}, err
	}
	if !validCoreScope(scope, command.WorkspaceID, values.RequirementID) {
		return task.CoreMutationResult{}, task.ErrCoreMutationConflict
	}
	if err := validateCoreUserFacts(ctx, facts, command.WorkspaceID, values, command.PrincipalID); err != nil {
		return task.CoreMutationResult{}, err
	}
	kind, status, priority := values.Kind, values.Status, values.Priority
	if kind == "" {
		kind = task.WorkTaskKindTask
	}
	if status == "" {
		status = "draft"
	}
	if priority == "" {
		priority = "normal"
	}
	reporter := values.ReporterUserID
	if reporter == nil {
		reporter = uintPointer(command.PrincipalID)
	}
	row := WorkTask{
		WorkspaceID: scope.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID, ProjectID: scope.PlanUnitID,
		RequirementID: scope.RequirementID, TaskClientID: values.TaskClientID, Kind: kind, Title: values.Title, Description: values.Description,
		Status: status, Priority: priority, AssigneeID: values.AssigneeID, ReporterUserID: reporter,
		ParentID: values.ParentID, StartsAt: values.StartsAt, DueAt: values.DueAt, MutationState: workTaskStateActive,
	}
	if values.EstimatedMinutes != nil {
		row.EstimatedMinutes = *values.EstimatedMinutes
	}
	if err := validateCoreTaskClient(tx, row.WorkspaceID, row.TaskClientID); err != nil {
		return task.CoreMutationResult{}, err
	}
	if err := lockAndValidateCoreParent(tx, &row); err != nil {
		return task.CoreMutationResult{}, err
	}
	key, err := Next(tx, ProjectScope{ID: scope.ProjectSetID, WorkspaceID: scope.WorkspaceID, Code: scope.ProjectCode})
	if err != nil {
		return task.CoreMutationResult{}, err
	}
	row.TaskKey = key
	if row.DueAt == nil {
		row.DueAt, err = inheritedCoreDue(tx, row.WorkspaceID, row.ParentID)
		if err != nil {
			return task.CoreMutationResult{}, err
		}
	}
	applyCoreCompletion(&row, row.Status, time.Now().UTC())
	if err := tx.Create(&row).Error; err != nil {
		return task.CoreMutationResult{}, err
	}
	if err := tx.Create(&WorkTaskActivity{WorkspaceID: row.WorkspaceID, WorkTaskID: row.ID, UserID: command.PrincipalID, Action: "created", Summary: "创建了任务"}).Error; err != nil {
		return task.CoreMutationResult{}, err
	}
	if err := effects.RecordCoreTaskMutation(ctx, coreEffect(command, row, "created", snapshotPayload(row), row.AssigneeID != nil)); err != nil {
		return task.CoreMutationResult{}, err
	}
	snapshot := taskSnapshot(row)
	return task.CoreMutationResult{Task: &snapshot}, nil
}

func applyCoreTaskUpdate(ctx context.Context, tx *gorm.DB, facts task.CoreMutationFacts, effects task.CoreMutationEffectSink, command task.CoreMutation) (task.CoreMutationResult, error) {
	row, err := lockCoreTask(tx, command.WorkspaceID, command.ID)
	if err != nil {
		return task.CoreMutationResult{}, err
	}
	if !sameCoreUpdatedAt(row.UpdatedAt, command.ExpectedUpdatedAt) {
		return task.CoreMutationResult{}, task.ErrCoreMutationConflict
	}
	currentScope, err := facts.ResolveCoreMutationScope(ctx, command.WorkspaceID, row.RequirementID)
	if err != nil || !coreTaskMatchesScope(row, currentScope) {
		if err != nil {
			return task.CoreMutationResult{}, err
		}
		return task.CoreMutationResult{}, task.ErrCoreMutationConflict
	}
	values := command.Values
	if err := validateCoreUserFacts(ctx, facts, command.WorkspaceID, values, command.PrincipalID); err != nil {
		return task.CoreMutationResult{}, err
	}
	scopeChanged := values.Fields["requirement_id"] && values.RequirementID != row.RequirementID
	var descendants []WorkTask
	if scopeChanged {
		if task.NormalizePMWorkTaskKind(row.Kind) == task.WorkTaskKindSubtask {
			return task.CoreMutationResult{}, task.ErrCoreMutationConflict
		}
		descendants, err = collectCoreDescendants(tx, command.WorkspaceID, []uint{row.ID})
		if err != nil {
			return task.CoreMutationResult{}, err
		}
		target, resolveErr := facts.ResolveCoreMutationScope(ctx, command.WorkspaceID, values.RequirementID)
		if resolveErr != nil || !validCoreScope(target, command.WorkspaceID, values.RequirementID) {
			if resolveErr != nil {
				return task.CoreMutationResult{}, resolveErr
			}
			return task.CoreMutationResult{}, task.ErrCoreMutationConflict
		}
		row.TeamID, row.ProjectSetID, row.ProjectID, row.RequirementID = target.TeamID, target.ProjectSetID, target.PlanUnitID, target.RequirementID
	}
	before := row
	applyCoreValues(&row, values)
	if err := validateCoreTaskClient(tx, row.WorkspaceID, row.TaskClientID); err != nil {
		return task.CoreMutationResult{}, err
	}
	if err := validateCoreTaskShape(tx, row); err != nil {
		return task.CoreMutationResult{}, err
	}
	applyCoreCompletion(&row, row.Status, time.Now().UTC())
	updates := coreTaskUpdates(before, row, values)
	if len(updates) > 0 {
		updated := tx.Model(&WorkTask{}).Where("id = ? AND workspace_id = ? AND mutation_state = ? AND updated_at = ?", row.ID, row.WorkspaceID, workTaskStateActive, command.ExpectedUpdatedAt).Updates(updates)
		if updated.Error != nil {
			return task.CoreMutationResult{}, updated.Error
		}
		if updated.RowsAffected != 1 {
			return task.CoreMutationResult{}, task.ErrCoreMutationConflict
		}
	}
	if scopeChanged && len(descendants) > 0 {
		if err := moveCoreDescendants(tx, row, descendants); err != nil {
			return task.CoreMutationResult{}, err
		}
	}
	if err := tx.Where("id = ? AND workspace_id = ?", row.ID, row.WorkspaceID).First(&row).Error; err != nil {
		return task.CoreMutationResult{}, err
	}
	if err := recordCoreUpdateActivities(tx, before, row, values.Fields, command.PrincipalID); err != nil {
		return task.CoreMutationResult{}, err
	}
	if values.Fields["due_at"] || values.Fields["parent_id"] {
		if err := propagateCoreDue(tx, row, command.PrincipalID); err != nil {
			return task.CoreMutationResult{}, err
		}
	}
	if coreTerminal(row.Status) {
		if err := autoCompleteCoreAncestors(ctx, tx, row.WorkspaceID, row.ParentID, command, effects); err != nil {
			return task.CoreMutationResult{}, err
		}
	}
	notifyAssignee := values.Fields["assignee_id"] && !sameOptionalCoreUint(before.AssigneeID, row.AssigneeID) && row.AssigneeID != nil
	effect := coreEffect(command, row, "updated", snapshotPayload(row), notifyAssignee)
	effect.PreviousStatus = before.Status
	if err := effects.RecordCoreTaskMutation(ctx, effect); err != nil {
		return task.CoreMutationResult{}, err
	}
	snapshot := taskSnapshot(row)
	return task.CoreMutationResult{Task: &snapshot}, nil
}

func applyCoreTaskClone(ctx context.Context, tx *gorm.DB, facts task.CoreMutationFacts, effects task.CoreMutationEffectSink, command task.CoreMutation) (task.CoreMutationResult, error) {
	source, err := lockCoreTask(tx, command.WorkspaceID, command.ID)
	if err != nil {
		return task.CoreMutationResult{}, err
	}
	if !sameCoreUpdatedAt(source.UpdatedAt, command.ExpectedUpdatedAt) {
		return task.CoreMutationResult{}, task.ErrCoreMutationConflict
	}
	sourceScope, err := facts.ResolveCoreMutationScope(ctx, command.WorkspaceID, source.RequirementID)
	if err != nil {
		return task.CoreMutationResult{}, err
	}
	if !coreTaskMatchesScope(source, sourceScope) {
		return task.CoreMutationResult{}, task.ErrCoreMutationConflict
	}
	targetRequirement := source.RequirementID
	if command.TargetRequirementID > 0 {
		targetRequirement = command.TargetRequirementID
	}
	if targetRequirement != source.RequirementID && task.NormalizePMWorkTaskKind(source.Kind) == task.WorkTaskKindSubtask {
		return task.CoreMutationResult{}, task.ErrCoreMutationConflict
	}
	scope, err := facts.ResolveCoreMutationScope(ctx, command.WorkspaceID, targetRequirement)
	if err != nil || !validCoreScope(scope, command.WorkspaceID, targetRequirement) {
		if err != nil {
			return task.CoreMutationResult{}, err
		}
		return task.CoreMutationResult{}, task.ErrCoreMutationConflict
	}
	if err := facts.ValidateActiveTaskUsers(ctx, command.WorkspaceID, coreUserIDs(source.AssigneeID, source.ReporterUserID)); err != nil {
		return task.CoreMutationResult{}, err
	}
	suffix := command.TitleSuffix
	if !command.TitleSuffixSet {
		suffix = "（副本）"
	}
	title := strings.TrimSpace(source.Title) + suffix
	if strings.TrimSpace(title) == "" || len([]rune(title)) > 220 {
		return task.CoreMutationResult{}, task.ErrCoreMutationInvalid
	}
	clone := WorkTask{WorkspaceID: scope.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID, ProjectID: scope.PlanUnitID, RequirementID: scope.RequirementID,
		TaskClientID: source.TaskClientID, Kind: source.Kind, Title: title, Description: source.Description, Status: "draft", Priority: source.Priority, AssigneeID: source.AssigneeID,
		ReporterUserID: source.ReporterUserID, EstimatedMinutes: source.EstimatedMinutes, StartsAt: source.StartsAt, DueAt: source.DueAt, MutationState: workTaskStateActive}
	if targetRequirement == source.RequirementID {
		clone.ParentID = source.ParentID
	}
	if err := lockAndValidateCoreParent(tx, &clone); err != nil {
		return task.CoreMutationResult{}, err
	}
	clone.TaskKey, err = Next(tx, ProjectScope{ID: scope.ProjectSetID, WorkspaceID: scope.WorkspaceID, Code: scope.ProjectCode})
	if err != nil {
		return task.CoreMutationResult{}, err
	}
	if clone.DueAt == nil {
		clone.DueAt, err = inheritedCoreDue(tx, clone.WorkspaceID, clone.ParentID)
		if err != nil {
			return task.CoreMutationResult{}, err
		}
	}
	if err := tx.Create(&clone).Error; err != nil {
		return task.CoreMutationResult{}, err
	}
	if err := tx.Create(&WorkTaskActivity{WorkspaceID: clone.WorkspaceID, WorkTaskID: clone.ID, UserID: command.PrincipalID, Action: "cloned", Detail: fmt.Sprintf("source_task_id=%d", source.ID), Summary: fmt.Sprintf("克隆自任务 %s", source.TaskKey)}).Error; err != nil {
		return task.CoreMutationResult{}, err
	}
	if err := effects.RecordCoreTaskMutation(ctx, coreEffect(command, clone, "created", map[string]any{"task": snapshotPayload(clone), "cloned_from_task_id": source.ID}, clone.AssigneeID != nil)); err != nil {
		return task.CoreMutationResult{}, err
	}
	snapshot, reference := taskSnapshot(clone), task.Reference{ID: source.ID, TaskKey: source.TaskKey, Title: source.Title}
	return task.CoreMutationResult{Task: &snapshot, ClonedFrom: &reference}, nil
}

func lockCoreTask(tx *gorm.DB, workspaceID, id uint) (WorkTask, error) {
	var row WorkTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND mutation_state = ?", workspaceID, id, workTaskStateActive).First(&row).Error; err != nil {
		return WorkTask{}, normalizeCoreMutationError(err)
	}
	return row, nil
}

func normalizeCoreMutationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrCoreMutationNotFound
	}
	return err
}
