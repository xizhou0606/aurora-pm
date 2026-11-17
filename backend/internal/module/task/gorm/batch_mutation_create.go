package taskgorm

import (
	"context"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type preparedBatchTask struct {
	input    task.BatchTaskInput
	scope    task.BatchRequirementScope
	row      WorkTask
	children []preparedBatchTask
}

func applyTaskBatchCreate(
	ctx context.Context,
	tx *gorm.DB,
	dependencies BatchMutationDependencies,
	command task.BatchMutation,
) (task.BatchMutationResult, error) {
	prepared, err := prepareTaskBatchCreate(ctx, tx, dependencies, command)
	if err != nil {
		return task.BatchMutationResult{}, err
	}
	created, count := make([]task.BatchCreatedTask, 0, len(prepared)), 0
	for index := range prepared {
		result, createdCount, createErr := createPreparedBatchTask(ctx, tx, dependencies, command, &prepared[index], nil)
		if createErr != nil {
			return task.BatchMutationResult{}, createErr
		}
		created, count = append(created, result), count+createdCount
	}
	result := task.BatchMutationResult{Created: created, CreatedCount: count, Atomic: true}
	if command.Operation == task.ChildrenBatchCreate {
		result.ParentID = command.ParentID
	}
	return result, nil
}

func prepareTaskBatchCreate(
	ctx context.Context,
	tx *gorm.DB,
	dependencies BatchMutationDependencies,
	command task.BatchMutation,
) ([]preparedBatchTask, error) {
	inputs := command.Tasks
	var inherited *WorkTask
	if command.Operation == task.ChildrenBatchCreate {
		var parent WorkTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND workspace_id = ? AND mutation_state = ?", command.ParentID, command.WorkspaceID, workTaskStateActive,
		).First(&parent).Error; err != nil {
			return nil, normalizeTaskBatchMutationError(err)
		}
		if NormalizePMWorkTaskKind(parent.Kind) != WorkTaskKindTask {
			return nil, task.ErrBatchMutationConflict
		}
		inherited = &parent
	}
	seen, userIDs := map[string]bool{}, make([]uint, 0, len(inputs)*2)
	prepared := make([]preparedBatchTask, 0, len(inputs))
	for _, input := range inputs {
		requirementID := input.RequirementID
		if inherited != nil {
			requirementID = inherited.RequirementID
		}
		scope, err := dependencies.Scopes.ResolveTaskRequirementScope(ctx, command.WorkspaceID, requirementID)
		if err != nil || !validBatchRequirementScope(scope, command.WorkspaceID, requirementID) {
			return nil, task.ErrBatchMutationConflict
		}
		if inherited != nil && !batchTaskMatchesScope(*inherited, scope) {
			return nil, task.ErrBatchMutationConflict
		}
		item, ids, err := normalizeBatchCreateTask(tx, input, scope, command.PrincipalID, inherited, seen)
		if err != nil {
			return nil, err
		}
		userIDs = append(userIDs, ids...)
		prepared = append(prepared, item)
	}
	if err := dependencies.Users.VerifyTaskBatchUsers(ctx, command.WorkspaceID, uniqueBatchUserIDs(userIDs)); err != nil {
		return nil, task.ErrBatchMutationConflict
	}
	return prepared, nil
}

func normalizeBatchCreateTask(
	tx *gorm.DB,
	input task.BatchTaskInput,
	scope task.BatchRequirementScope,
	principalID uint,
	parent *WorkTask,
	seen map[string]bool,
) (preparedBatchTask, []uint, error) {
	clientID := strings.TrimSpace(input.ClientID)
	if clientID == "" || len(clientID) > 100 || seen[clientID] || !task.ValidBatchText(input.Title, 160) {
		return preparedBatchTask{}, nil, task.ErrBatchMutationInvalid
	}
	seen[clientID] = true
	status, priority := "draft", "normal"
	if input.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*input.Status))
	}
	if input.Priority != nil {
		priority = normalizeBatchPriority(*input.Priority)
	}
	assigneeID := principalID
	if parent != nil && parent.AssigneeID != nil {
		assigneeID = *parent.AssigneeID
	}
	if input.AssigneeID != nil {
		assigneeID = *input.AssigneeID
	}
	reporterID := principalID
	if input.ReporterUserID != nil {
		reporterID = *input.ReporterUserID
	}
	startsAt, dueAt := input.StartsAt, input.DueAt
	taskClientID := input.TaskClientID
	if parent != nil {
		if taskClientID == nil {
			taskClientID = parent.TaskClientID
		}
		if startsAt == nil {
			startsAt = parent.StartsAt
		}
		if dueAt == nil {
			dueAt = parent.DueAt
		}
	}
	if !validBatchStatus(status) || !validBatchPriority(priority) || assigneeID == 0 || reporterID == 0 ||
		input.EstimatedMinutes != nil && *input.EstimatedMinutes < 0 || startsAt != nil && dueAt != nil && startsAt.After(*dueAt) {
		return preparedBatchTask{}, nil, task.ErrBatchMutationInvalid
	}
	description, estimated := "", 0
	if input.Description != nil {
		description = strings.TrimSpace(*input.Description)
	}
	if input.EstimatedMinutes != nil {
		estimated = *input.EstimatedMinutes
	}
	kind := WorkTaskKindTask
	var parentID *uint
	if parent != nil {
		kind, parentID = WorkTaskKindSubtask, uintBatchPointer(parent.ID)
	}
	row := WorkTask{
		WorkspaceID: scope.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID,
		ProjectID: scope.PlanUnitID, RequirementID: scope.RequirementID, TaskClientID: taskClientID, Kind: kind,
		Title: strings.TrimSpace(input.Title), Description: description, Status: status, Priority: priority,
		AssigneeID: uintBatchPointer(assigneeID), ReporterUserID: uintBatchPointer(reporterID), ParentID: parentID,
		EstimatedMinutes: estimated, StartsAt: copyBatchTime(startsAt), DueAt: copyBatchTime(dueAt),
		MutationState: workTaskStateActive,
	}
	if err := validateCoreTaskClient(tx, row.WorkspaceID, row.TaskClientID); err != nil {
		return preparedBatchTask{}, nil, err
	}
	applyBatchLifecycle(&row)
	prepared := preparedBatchTask{input: input, scope: scope, row: row}
	userIDs := []uint{assigneeID, reporterID}
	for _, child := range input.Children {
		child.RequirementID = scope.RequirementID
		childPrepared, childUsers, err := normalizeBatchCreateTask(tx, child, scope, principalID, &row, seen)
		if err != nil {
			return preparedBatchTask{}, nil, err
		}
		prepared.children = append(prepared.children, childPrepared)
		userIDs = append(userIDs, childUsers...)
	}
	return prepared, userIDs, nil
}

func createPreparedBatchTask(
	ctx context.Context,
	tx *gorm.DB,
	dependencies BatchMutationDependencies,
	command task.BatchMutation,
	prepared *preparedBatchTask,
	parentID *uint,
) (task.BatchCreatedTask, int, error) {
	if parentID != nil {
		prepared.row.ParentID = parentID
		prepared.row.Kind = WorkTaskKindSubtask
	}
	key, err := Next(tx, ProjectScope{ID: prepared.scope.ProjectSetID, WorkspaceID: prepared.scope.WorkspaceID, Code: prepared.scope.ProjectCode})
	if err != nil {
		return task.BatchCreatedTask{}, 0, err
	}
	prepared.row.TaskKey = key
	if err := tx.Create(&prepared.row).Error; err != nil {
		return task.BatchCreatedTask{}, 0, err
	}
	if err := tx.Create(&WorkTaskActivity{
		WorkspaceID: command.WorkspaceID, WorkTaskID: prepared.row.ID, UserID: command.PrincipalID,
		Action: "created", Summary: "创建了任务",
	}).Error; err != nil {
		return task.BatchCreatedTask{}, 0, err
	}
	if err := dependencies.Notifications.RecordTaskAssignment(ctx, task.AssignmentNotification{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, TaskID: prepared.row.ID,
		AssigneeUserID: *prepared.row.AssigneeID, TaskKey: prepared.row.TaskKey, Title: prepared.row.Title,
	}); err != nil {
		return task.BatchCreatedTask{}, 0, err
	}
	if err := dependencies.Effects.RecordTaskBatchEffect(ctx, task.BatchEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: prepared.row.ProjectSetID,
		TaskID: prepared.row.ID, Action: "created", ApplyKey: command.ApplyKey,
		Payload: map[string]any{"task_key": prepared.row.TaskKey, "title": prepared.row.Title},
	}); err != nil {
		return task.BatchCreatedTask{}, 0, err
	}
	result, count := task.BatchCreatedTask{ClientID: strings.TrimSpace(prepared.input.ClientID), ID: prepared.row.ID, TaskKey: prepared.row.TaskKey}, 1
	for index := range prepared.children {
		child, childCount, err := createPreparedBatchTask(ctx, tx, dependencies, command, &prepared.children[index], &prepared.row.ID)
		if err != nil {
			return task.BatchCreatedTask{}, 0, err
		}
		result.Children, count = append(result.Children, child), count+childCount
	}
	return result, count, nil
}

func validBatchRequirementScope(scope task.BatchRequirementScope, workspaceID, requirementID uint) bool {
	return scope.WorkspaceID == workspaceID && scope.RequirementID == requirementID && scope.TeamID > 0 &&
		scope.ProjectSetID > 0 && scope.PlanUnitID > 0
}

func batchTaskMatchesScope(row WorkTask, scope task.BatchRequirementScope) bool {
	return row.WorkspaceID == scope.WorkspaceID && row.TeamID == scope.TeamID && row.ProjectSetID == scope.ProjectSetID &&
		row.ProjectID == scope.PlanUnitID && row.RequirementID == scope.RequirementID
}

func uintBatchPointer(value uint) *uint { return &value }

func copyBatchTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func uniqueBatchUserIDs(values []uint) []uint {
	seen, result := map[uint]bool{}, make([]uint, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
