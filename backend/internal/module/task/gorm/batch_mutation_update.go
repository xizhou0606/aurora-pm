package taskgorm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyTaskBatchUpdate(
	ctx context.Context,
	tx *gorm.DB,
	dependencies BatchMutationDependencies,
	command task.BatchMutation,
) (task.BatchMutationResult, error) {
	ids, err := resolveTaskBatchSelection(tx, command.WorkspaceID, command.Selector, true)
	if err != nil {
		return task.BatchMutationResult{}, err
	}
	updates, userIDs, err := batchPatchUpdates(command.Updates)
	if err != nil {
		return task.BatchMutationResult{}, err
	}
	if err := dependencies.Users.VerifyTaskBatchUsers(ctx, command.WorkspaceID, userIDs); err != nil {
		return task.BatchMutationResult{}, task.ErrBatchMutationConflict
	}
	if command.Updates.TaskClientID.Set {
		if err := validateCoreTaskClient(tx, command.WorkspaceID, command.Updates.TaskClientID.Value); err != nil {
			return task.BatchMutationResult{}, err
		}
	}
	rows, err := lockedBatchTasksByID(tx, command.WorkspaceID, ids)
	if err != nil {
		return task.BatchMutationResult{}, err
	}
	beforeByID := make(map[uint]WorkTask, len(rows))
	for _, row := range rows {
		beforeByID[row.ID] = row
	}
	afterByID := make(map[uint]WorkTask, len(rows))
	for _, id := range ids {
		before := beforeByID[id]
		after, rowUpdates, err := prepareBatchTaskUpdate(before, updates)
		if err != nil {
			return task.BatchMutationResult{}, err
		}
		changed := tx.Model(&WorkTask{}).Where(
			"id = ? AND workspace_id = ? AND mutation_state = ? AND updated_at = ?", before.ID, before.WorkspaceID, workTaskStateActive, before.UpdatedAt,
		).Updates(rowUpdates)
		if changed.Error != nil {
			return task.BatchMutationResult{}, changed.Error
		}
		if changed.RowsAffected != 1 {
			return task.BatchMutationResult{}, task.ErrBatchMutationConflict
		}
		if err := tx.Where("id = ? AND workspace_id = ? AND mutation_state = ?", id, command.WorkspaceID, workTaskStateActive).First(&after).Error; err != nil {
			return task.BatchMutationResult{}, err
		}
		afterByID[id] = after
		if err := createBatchUpdateActivities(tx, command.PrincipalID, before, after, rowUpdates); err != nil {
			return task.BatchMutationResult{}, err
		}
		if assigneeChanged(before.AssigneeID, after.AssigneeID) && after.AssigneeID != nil {
			if err := dependencies.Notifications.RecordTaskAssignment(ctx, task.AssignmentNotification{
				WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, TaskID: after.ID,
				AssigneeUserID: *after.AssigneeID, TaskKey: after.TaskKey, Title: after.Title,
			}); err != nil {
				return task.BatchMutationResult{}, err
			}
		}
		if err := dependencies.Effects.RecordTaskBatchEffect(ctx, task.BatchEffect{
			WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: after.ProjectSetID,
			PreviousStatus: before.Status, TaskID: after.ID, Action: "updated", ApplyKey: command.ApplyKey, Payload: rowUpdates,
		}); err != nil {
			return task.BatchMutationResult{}, err
		}
	}
	duePropagated := map[uint]bool{}
	if command.Updates.DueAt.Set && command.Updates.DueAt.Value != nil {
		for _, id := range ids {
			if err := propagateBatchDue(ctx, tx, dependencies, command, afterByID[id], duePropagated); err != nil {
				return task.BatchMutationResult{}, err
			}
		}
	}
	autoCompleted := map[uint]bool{}
	for _, id := range ids {
		if isBatchTerminal(afterByID[id].Status) {
			if err := autoCompleteBatchAncestors(ctx, tx, dependencies, command, afterByID[id].ParentID, autoCompleted); err != nil {
				return task.BatchMutationResult{}, err
			}
		}
	}
	affected := map[uint]bool{}
	for _, id := range ids {
		affected[id] = true
	}
	for id := range duePropagated {
		affected[id] = true
	}
	for id := range autoCompleted {
		affected[id] = true
	}
	return task.BatchMutationResult{
		UpdatedIDs: ids, UpdatedCount: len(ids), AffectedIDs: sortedBatchIDs(affected),
		AutoCompletedAncestorIDs: sortedBatchIDs(autoCompleted), DuePropagatedDescendantIDs: sortedBatchIDs(duePropagated),
		Atomic: true,
	}, nil
}

func batchPatchUpdates(patch task.BatchTaskPatch) (map[string]any, []uint, error) {
	updates, users := map[string]any{}, make([]uint, 0, 2)
	if patch.TaskClientID.Set {
		if patch.TaskClientID.Value != nil && *patch.TaskClientID.Value == 0 {
			return nil, nil, task.ErrBatchMutationInvalid
		}
		updates["task_client_id"] = patch.TaskClientID.Value
	}
	for field, value := range map[string]task.Optional[string]{
		"title": patch.Title, "description": patch.Description, "status": patch.Status, "priority": patch.Priority,
	} {
		if !value.Set {
			continue
		}
		if value.Value == nil {
			return nil, nil, task.ErrBatchMutationInvalid
		}
		normalized := strings.TrimSpace(*value.Value)
		if field == "title" && !task.ValidBatchText(normalized, 160) || field == "status" && !validBatchStatus(normalized) ||
			field == "priority" && !validBatchPriority(normalizeBatchPriority(normalized)) {
			return nil, nil, task.ErrBatchMutationInvalid
		}
		if field == "status" {
			normalized = strings.ToLower(normalized)
		}
		if field == "priority" {
			normalized = normalizeBatchPriority(normalized)
		}
		updates[field] = normalized
	}
	for field, value := range map[string]task.Optional[uint]{"assignee_id": patch.AssigneeID, "reporter_user_id": patch.ReporterUserID} {
		if !value.Set {
			continue
		}
		if value.Value != nil && *value.Value == 0 {
			return nil, nil, task.ErrBatchMutationInvalid
		}
		updates[field] = value.Value
		if value.Value != nil {
			users = append(users, *value.Value)
		}
	}
	if patch.EstimatedMinutes.Set {
		if patch.EstimatedMinutes.Value != nil && *patch.EstimatedMinutes.Value < 0 {
			return nil, nil, task.ErrBatchMutationInvalid
		}
		updates["estimated_minutes"] = patch.EstimatedMinutes.Value
	}
	for field, value := range map[string]task.Optional[time.Time]{"starts_at": patch.StartsAt, "due_at": patch.DueAt} {
		if value.Set {
			updates[field] = copyBatchTime(value.Value)
		}
	}
	if len(updates) == 0 {
		return nil, nil, task.ErrBatchMutationInvalid
	}
	return updates, uniqueBatchUserIDs(users), nil
}

func prepareBatchTaskUpdate(before WorkTask, updates map[string]any) (WorkTask, map[string]any, error) {
	after, rowUpdates := before, make(map[string]any, len(updates)+1)
	for key, value := range updates {
		rowUpdates[key] = value
		switch key {
		case "task_client_id":
			after.TaskClientID, _ = value.(*uint)
		case "title":
			after.Title = value.(string)
		case "description":
			after.Description = value.(string)
		case "status":
			after.Status = value.(string)
		case "priority":
			after.Priority = value.(string)
		case "assignee_id":
			after.AssigneeID, _ = value.(*uint)
		case "reporter_user_id":
			after.ReporterUserID, _ = value.(*uint)
		case "estimated_minutes":
			after.EstimatedMinutes = 0
			if pointer, _ := value.(*int); pointer != nil {
				after.EstimatedMinutes = *pointer
			}
		case "starts_at":
			after.StartsAt, _ = value.(*time.Time)
		case "due_at":
			after.DueAt, _ = value.(*time.Time)
		}
	}
	if after.StartsAt != nil && after.DueAt != nil && after.StartsAt.After(*after.DueAt) {
		return WorkTask{}, nil, task.ErrBatchMutationInvalid
	}
	applyBatchLifecycle(&after)
	rowUpdates["completed_at"] = after.CompletedAt
	return after, rowUpdates, nil
}

func lockedBatchTasksByID(tx *gorm.DB, workspaceID uint, ids []uint) ([]WorkTask, error) {
	var rows []WorkTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND id IN ? AND mutation_state = ? AND LOWER(TRIM(kind)) IN ?", workspaceID, ids, workTaskStateActive, PMWorkTaskKinds(),
	).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, task.ErrBatchMutationNotFound
	}
	return rows, nil
}

func propagateBatchDue(
	ctx context.Context,
	tx *gorm.DB,
	dependencies BatchMutationDependencies,
	command task.BatchMutation,
	root WorkTask,
	affected map[uint]bool,
) error {
	frontier := []WorkTask{root}
	for len(frontier) > 0 {
		parent := frontier[0]
		frontier = frontier[1:]
		var children []WorkTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND parent_id = ? AND mutation_state = ? AND LOWER(TRIM(kind)) IN ?", command.WorkspaceID, parent.ID, workTaskStateActive, PMWorkTaskKinds(),
		).Find(&children).Error; err != nil {
			return err
		}
		for index := range children {
			child := &children[index]
			if child.DueAt == nil && parent.DueAt != nil {
				changed := tx.Model(&WorkTask{}).Where("id = ? AND workspace_id = ? AND mutation_state = ? AND due_at IS NULL", child.ID, command.WorkspaceID, workTaskStateActive).
					Update("due_at", parent.DueAt)
				if changed.Error != nil || changed.RowsAffected != 1 {
					return task.ErrBatchMutationConflict
				}
				child.DueAt = copyBatchTime(parent.DueAt)
				affected[child.ID] = true
				if err := tx.Create(&WorkTaskActivity{
					WorkspaceID: command.WorkspaceID, WorkTaskID: child.ID, UserID: command.PrincipalID,
					Action: "auto_inherit_due", FieldKey: "due_at", NewValue: child.DueAt.Format(time.RFC3339),
					Summary: "截止日期已继承自父项",
				}).Error; err != nil {
					return err
				}
			}
			frontier = append(frontier, *child)
		}
	}
	return nil
}

func autoCompleteBatchAncestors(
	ctx context.Context,
	tx *gorm.DB,
	dependencies BatchMutationDependencies,
	command task.BatchMutation,
	parentID *uint,
	affected map[uint]bool,
) error {
	seen := map[uint]bool{}
	for parentID != nil && *parentID > 0 {
		if seen[*parentID] {
			return task.ErrBatchMutationConflict
		}
		seen[*parentID] = true
		var parent WorkTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND workspace_id = ? AND mutation_state = ?", *parentID, command.WorkspaceID, workTaskStateActive,
		).First(&parent).Error; err != nil {
			return err
		}
		var children []WorkTask
		if err := tx.Where("workspace_id = ? AND parent_id = ? AND mutation_state = ?", command.WorkspaceID, parent.ID, workTaskStateActive).Find(&children).Error; err != nil {
			return err
		}
		allTerminal := len(children) > 0
		for _, child := range children {
			allTerminal = allTerminal && isBatchTerminal(child.Status)
		}
		if !allTerminal {
			return nil
		}
		if !isBatchTerminal(parent.Status) {
			now := time.Now().UTC()
			changed := tx.Model(&WorkTask{}).Where(
				"id = ? AND workspace_id = ? AND mutation_state = ? AND updated_at = ?", parent.ID, command.WorkspaceID, workTaskStateActive, parent.UpdatedAt,
			).Updates(map[string]any{"status": "completed", "completed_at": &now})
			if changed.Error != nil || changed.RowsAffected != 1 {
				return task.ErrBatchMutationConflict
			}
			affected[parent.ID] = true
			if err := tx.Create(&WorkTaskActivity{
				WorkspaceID: command.WorkspaceID, WorkTaskID: parent.ID, UserID: command.PrincipalID,
				Action: "auto_complete", FieldKey: "status", OldValue: parent.Status, NewValue: "completed",
				Summary: "所有子项完成，父项已自动完成",
			}).Error; err != nil {
				return err
			}
			if err := dependencies.Effects.RecordTaskBatchEffect(ctx, task.BatchEffect{
				WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: parent.ProjectSetID,
				PreviousStatus: parent.Status, TaskID: parent.ID, Action: "auto_completed", ApplyKey: command.ApplyKey,
				Payload: map[string]any{"status": "completed"},
			}); err != nil {
				return err
			}
		}
		parentID = parent.ParentID
	}
	return nil
}

func createBatchUpdateActivities(tx *gorm.DB, principalID uint, before, after WorkTask, updates map[string]any) error {
	fields := []string{"title", "description", "status", "priority", "assignee_id", "reporter_user_id", "estimated_minutes", "starts_at", "due_at"}
	rows := make([]WorkTaskActivity, 0, len(fields))
	for _, field := range fields {
		if _, changed := updates[field]; !changed || batchActivityValue(before, field) == batchActivityValue(after, field) {
			continue
		}
		action, summary := "field_change", fmt.Sprintf("%s 已修改", field)
		if field == "status" {
			action = "status_change"
		}
		rows = append(rows, WorkTaskActivity{
			WorkspaceID: before.WorkspaceID, WorkTaskID: before.ID, UserID: principalID,
			Action: action, FieldKey: field, OldValue: batchActivityValue(before, field), NewValue: batchActivityValue(after, field), Summary: summary,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func batchActivityValue(row WorkTask, field string) string {
	switch field {
	case "title":
		return row.Title
	case "description":
		return row.Description
	case "status":
		return row.Status
	case "priority":
		return row.Priority
	case "assignee_id":
		if row.AssigneeID != nil {
			return fmt.Sprint(*row.AssigneeID)
		}
	case "reporter_user_id":
		if row.ReporterUserID != nil {
			return fmt.Sprint(*row.ReporterUserID)
		}
	case "estimated_minutes":
		return fmt.Sprint(row.EstimatedMinutes)
	case "starts_at":
		if row.StartsAt != nil {
			return row.StartsAt.UTC().Format(time.RFC3339)
		}
	case "due_at":
		if row.DueAt != nil {
			return row.DueAt.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func validBatchStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft", "in_dev", "pending_test", "completed", "cancelled":
		return true
	default:
		return false
	}
}

func normalizeBatchPriority(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "medium" {
		return "normal"
	}
	return value
}

func validBatchPriority(value string) bool {
	switch normalizeBatchPriority(value) {
	case "low", "normal", "high", "urgent":
		return true
	default:
		return false
	}
}

func isBatchTerminal(status string) bool { return status == "completed" || status == "cancelled" }

func applyBatchLifecycle(row *WorkTask) {
	if isBatchTerminal(row.Status) {
		now := time.Now().UTC()
		row.CompletedAt = &now
	} else {
		row.CompletedAt = nil
	}
}

func assigneeChanged(left, right *uint) bool {
	return left == nil && right != nil || left != nil && right == nil || left != nil && right != nil && *left != *right
}

func sortedBatchIDs(values map[uint]bool) []uint {
	result := make([]uint, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
