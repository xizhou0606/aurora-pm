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

func validCoreScope(scope task.CoreMutationScope, workspaceID, requirementID uint) bool {
	return scope.WorkspaceID == workspaceID && scope.RequirementID == requirementID && scope.TeamID > 0 &&
		scope.ProjectSetID > 0 && scope.PlanUnitID > 0 && scope.RequirementVersion > 0
}

func coreTaskMatchesScope(row WorkTask, scope task.CoreMutationScope) bool {
	return validCoreScope(scope, row.WorkspaceID, row.RequirementID) && row.TeamID == scope.TeamID &&
		row.ProjectSetID == scope.ProjectSetID && row.ProjectID == scope.PlanUnitID
}

func validateCoreUserFacts(ctx context.Context, facts task.CoreMutationFacts, workspaceID uint, values task.CoreMutationValues, principalID uint) error {
	ids := coreUserIDs(values.AssigneeID, values.ReporterUserID, uintPointer(principalID))
	return facts.ValidateActiveTaskUsers(ctx, workspaceID, ids)
}

func coreUserIDs(values ...*uint) []uint {
	result, seen := make([]uint, 0, len(values)), map[uint]struct{}{}
	for _, value := range values {
		if value == nil || *value == 0 {
			continue
		}
		if _, exists := seen[*value]; exists {
			continue
		}
		seen[*value] = struct{}{}
		result = append(result, *value)
	}
	return result
}

func lockAndValidateCoreParent(tx *gorm.DB, row *WorkTask) error {
	return validateCoreTaskShape(tx, *row)
}

func validateCoreTaskShape(tx *gorm.DB, row WorkTask) error {
	kind := task.NormalizePMWorkTaskKind(row.Kind)
	if kind == task.WorkTaskKindTask {
		if row.ParentID != nil {
			return task.ErrCoreMutationInvalid
		}
		return nil
	}
	if kind != task.WorkTaskKindSubtask || row.ParentID == nil || *row.ParentID == row.ID {
		return task.ErrCoreMutationInvalid
	}
	var parent WorkTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND id = ? AND mutation_state = ?", row.WorkspaceID, *row.ParentID, workTaskStateActive,
	).First(&parent).Error
	if err != nil {
		return normalizeCoreMutationError(err)
	}
	if parent.TeamID != row.TeamID || parent.ProjectSetID != row.ProjectSetID || parent.ProjectID != row.ProjectID ||
		parent.RequirementID != row.RequirementID || task.NormalizePMWorkTaskKind(parent.Kind) != task.WorkTaskKindTask {
		return task.ErrCoreMutationConflict
	}
	return nil
}

func inheritedCoreDue(tx *gorm.DB, workspaceID uint, parentID *uint) (*time.Time, error) {
	seen := map[uint]struct{}{}
	for parentID != nil && *parentID > 0 {
		if _, exists := seen[*parentID]; exists {
			return nil, task.ErrCoreMutationConflict
		}
		seen[*parentID] = struct{}{}
		var parent WorkTask
		if err := tx.Where("workspace_id = ? AND id = ? AND mutation_state = ?", workspaceID, *parentID, workTaskStateActive).First(&parent).Error; err != nil {
			return nil, normalizeCoreMutationError(err)
		}
		if parent.DueAt != nil {
			value := parent.DueAt.UTC()
			return &value, nil
		}
		parentID = parent.ParentID
	}
	return nil, nil
}

func applyCoreValues(row *WorkTask, values task.CoreMutationValues) {
	fields := values.Fields
	if fields["requirement_id"] {
		row.RequirementID = values.RequirementID
	}
	if fields["task_client_id"] {
		row.TaskClientID = values.TaskClientID
	}
	if fields["kind"] {
		row.Kind = values.Kind
	}
	if fields["title"] {
		row.Title = values.Title
	}
	if fields["description"] {
		row.Description = values.Description
	}
	if fields["status"] {
		row.Status = values.Status
	}
	if fields["priority"] {
		row.Priority = values.Priority
	}
	if fields["assignee_id"] {
		row.AssigneeID = values.AssigneeID
	}
	if fields["reporter_user_id"] {
		row.ReporterUserID = values.ReporterUserID
	}
	if fields["parent_id"] {
		row.ParentID = values.ParentID
	}
	if fields["estimated_minutes"] {
		row.EstimatedMinutes = 0
		if values.EstimatedMinutes != nil {
			row.EstimatedMinutes = *values.EstimatedMinutes
		}
	}
	if fields["starts_at"] {
		row.StartsAt = values.StartsAt
	}
	if fields["due_at"] {
		row.DueAt = values.DueAt
	}
}

func applyCoreCompletion(row *WorkTask, status string, now time.Time) {
	if coreTerminal(status) {
		row.CompletedAt = &now
	} else {
		row.CompletedAt = nil
	}
}

func coreTerminal(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "cancelled", "canceled":
		return true
	}
	return false
}

func coreTaskUpdates(before, after WorkTask, values task.CoreMutationValues) map[string]any {
	updates := map[string]any{}
	fields := values.Fields
	if fields["requirement_id"] {
		updates["team_id"], updates["project_set_id"], updates["project_id"], updates["requirement_id"] = after.TeamID, after.ProjectSetID, after.ProjectID, after.RequirementID
	}
	if fields["task_client_id"] {
		updates["task_client_id"] = after.TaskClientID
	}
	if fields["kind"] {
		updates["kind"] = after.Kind
	}
	if fields["title"] {
		updates["title"] = after.Title
	}
	if fields["description"] {
		updates["description"] = after.Description
	}
	if fields["status"] {
		updates["status"], updates["completed_at"] = after.Status, after.CompletedAt
	}
	if fields["priority"] {
		updates["priority"] = after.Priority
	}
	if fields["assignee_id"] {
		updates["assignee_id"] = after.AssigneeID
	}
	if fields["reporter_user_id"] {
		updates["reporter_user_id"] = after.ReporterUserID
	}
	if fields["parent_id"] {
		updates["parent_id"] = after.ParentID
	}
	if fields["estimated_minutes"] {
		updates["estimated_minutes"] = after.EstimatedMinutes
	}
	if fields["starts_at"] {
		updates["starts_at"] = after.StartsAt
	}
	if fields["due_at"] {
		updates["due_at"] = after.DueAt
	}
	_ = before
	return updates
}

func recordCoreUpdateActivities(tx *gorm.DB, before, after WorkTask, fields map[string]bool, principalID uint) error {
	ordered := []string{"title", "description", "status", "priority", "task_client_id", "assignee_id", "reporter_user_id", "parent_id", "requirement_id", "estimated_minutes", "starts_at", "due_at"}
	for _, field := range ordered {
		if !fields[field] {
			continue
		}
		oldValue, newValue := coreFieldValue(before, field), coreFieldValue(after, field)
		if oldValue == newValue {
			continue
		}
		action, summary := "field_change", fmt.Sprintf("%s：%s → %s", field, oldValue, newValue)
		if field == "status" {
			action = "status_change"
		}
		if field == "description" {
			oldValue, newValue, summary = "", "", "说明已修改"
		}
		if err := tx.Create(&WorkTaskActivity{WorkspaceID: after.WorkspaceID, WorkTaskID: after.ID, UserID: principalID, Action: action, FieldKey: field, OldValue: oldValue, NewValue: newValue, Summary: summary}).Error; err != nil {
			return err
		}
	}
	return nil
}

func coreFieldValue(row WorkTask, field string) string {
	var value any
	switch field {
	case "title":
		value = row.Title
	case "description":
		value = row.Description
	case "status":
		value = row.Status
	case "priority":
		value = row.Priority
	case "task_client_id":
		value = row.TaskClientID
	case "assignee_id":
		value = row.AssigneeID
	case "reporter_user_id":
		value = row.ReporterUserID
	case "parent_id":
		value = row.ParentID
	case "requirement_id":
		value = row.RequirementID
	case "estimated_minutes":
		value = row.EstimatedMinutes
	case "starts_at":
		value = row.StartsAt
	case "due_at":
		value = row.DueAt
	}
	switch typed := value.(type) {
	case *uint:
		if typed == nil {
			return ""
		}
		return fmt.Sprint(*typed)
	case *time.Time:
		if typed == nil {
			return ""
		}
		return typed.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprint(typed)
	}
}

func autoCompleteCoreAncestors(ctx context.Context, tx *gorm.DB, workspaceID uint, parentID *uint, command task.CoreMutation, effects task.CoreMutationEffectSink) error {
	principalID := command.PrincipalID
	seen := map[uint]struct{}{}
	for parentID != nil && *parentID > 0 {
		if _, exists := seen[*parentID]; exists {
			return task.ErrCoreMutationConflict
		}
		seen[*parentID] = struct{}{}
		parent, err := lockCoreTask(tx, workspaceID, *parentID)
		if err != nil {
			return err
		}
		var children []WorkTask
		if err := tx.Where("workspace_id = ? AND parent_id = ? AND mutation_state = ?", workspaceID, parent.ID, workTaskStateActive).Find(&children).Error; err != nil {
			return err
		}
		if len(children) == 0 {
			return nil
		}
		for _, child := range children {
			if !coreTerminal(child.Status) {
				return nil
			}
		}
		if !coreTerminal(parent.Status) {
			now := time.Now().UTC()
			if err := tx.Model(&WorkTask{}).Where("id = ? AND workspace_id = ? AND mutation_state = ?", parent.ID, workspaceID, workTaskStateActive).Updates(map[string]any{"status": "completed", "completed_at": &now}).Error; err != nil {
				return err
			}
			if err := tx.Create(&WorkTaskActivity{WorkspaceID: workspaceID, WorkTaskID: parent.ID, UserID: principalID, Action: "auto_complete", FieldKey: "status", OldValue: parent.Status, NewValue: "completed", Summary: "所有子项完成，父项已自动完成"}).Error; err != nil {
				return err
			}
			previous := parent.Status
			parent.Status = "completed"
			effect := coreEffect(command, parent, "auto_completed", map[string]any{"status": "completed"}, false)
			effect.PreviousStatus = previous
			if err := effects.RecordCoreTaskMutation(ctx, effect); err != nil {
				return err
			}
		}
		parentID = parent.ParentID
	}
	return nil
}

func moveCoreDescendants(tx *gorm.DB, root WorkTask, descendants []WorkTask) error {
	ids := make([]uint, 0, len(descendants))
	for _, descendant := range descendants {
		if descendant.WorkspaceID != root.WorkspaceID || descendant.MutationState != workTaskStateActive {
			return task.ErrCoreMutationConflict
		}
		ids = append(ids, descendant.ID)
	}
	updated := tx.Model(&WorkTask{}).Where("workspace_id = ? AND id IN ? AND mutation_state = ?", root.WorkspaceID, ids, workTaskStateActive).
		Updates(map[string]any{"team_id": root.TeamID, "project_set_id": root.ProjectSetID, "project_id": root.ProjectID, "requirement_id": root.RequirementID})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != int64(len(ids)) {
		return task.ErrCoreMutationConflict
	}
	return nil
}

func propagateCoreDue(tx *gorm.DB, root WorkTask, principalID uint) error {
	descendants, err := collectCoreDescendants(tx, root.WorkspaceID, []uint{root.ID})
	if err != nil {
		return err
	}
	for _, descendant := range descendants {
		if descendant.DueAt != nil || descendant.ParentID == nil {
			continue
		}
		due, err := inheritedCoreDue(tx, root.WorkspaceID, descendant.ParentID)
		if err != nil {
			return err
		}
		if due == nil {
			continue
		}
		updated := tx.Model(&WorkTask{}).Where("workspace_id = ? AND id = ? AND mutation_state = ? AND due_at IS NULL", root.WorkspaceID, descendant.ID, workTaskStateActive).Update("due_at", due)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 1 {
			if err := tx.Create(&WorkTaskActivity{WorkspaceID: root.WorkspaceID, WorkTaskID: descendant.ID, UserID: principalID, Action: "auto_inherit_due", FieldKey: "due_at", NewValue: due.UTC().Format(time.RFC3339), Summary: "截止日期已继承自父项"}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func collectCoreDescendants(tx *gorm.DB, workspaceID uint, roots []uint) ([]WorkTask, error) {
	seen, frontier, result := map[uint]struct{}{}, append([]uint(nil), roots...), []WorkTask{}
	for _, id := range roots {
		seen[id] = struct{}{}
	}
	for len(frontier) > 0 {
		var rows []WorkTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND parent_id IN ? AND mutation_state = ?", workspaceID, frontier, workTaskStateActive).Order("id").Find(&rows).Error; err != nil {
			return nil, err
		}
		frontier = nil
		for _, row := range rows {
			if _, exists := seen[row.ID]; exists {
				continue
			}
			seen[row.ID] = struct{}{}
			result = append(result, row)
			frontier = append(frontier, row.ID)
			if len(result) > 10000 {
				return nil, task.ErrCoreMutationConflict
			}
		}
	}
	return result, nil
}

func coreEffect(command task.CoreMutation, row WorkTask, action string, payload map[string]any, notifyAssignee bool) task.CoreMutationEffect {
	return task.CoreMutationEffect{
		WorkspaceID: row.WorkspaceID, PrincipalID: command.PrincipalID, TeamID: row.TeamID,
		ProjectSetID: row.ProjectSetID, PlanUnitID: row.ProjectID, TaskID: row.ID, TaskKey: row.TaskKey,
		Action: action, Target: row.Title, Payload: payload, AssigneeID: row.AssigneeID, NotifyAssignee: notifyAssignee,
	}
}

func sameOptionalCoreUint(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func snapshotPayload(row WorkTask) map[string]any {
	raw := taskSnapshot(row)
	return map[string]any{"id": raw.ID, "workspace_id": raw.WorkspaceID, "team_id": raw.TeamID, "project_set_id": raw.ProjectSetID, "project_new_id": raw.PlanUnitID, "requirement_id": raw.RequirementID, "task_key": raw.TaskKey, "title": raw.Title, "status": raw.Status}
}

func sameCoreUpdatedAt(left, right time.Time) bool { return left.UTC().Equal(right.UTC()) }
func uintPointer(value uint) *uint                 { return &value }

func sortedCoreIDs(values []uint) []uint {
	result := append([]uint(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func validateCoreTaskClient(tx *gorm.DB, workspaceID uint, id *uint) error {
	if id == nil {
		return nil
	}
	var count int64
	if err := tx.Model(&TaskClientOption{}).Where("workspace_id = ? AND id = ?", workspaceID, *id).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return task.ErrCoreMutationInvalid
	}
	return nil
}
