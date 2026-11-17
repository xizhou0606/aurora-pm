package httpapi

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func pmTaskKinds() []string {
	return taskgorm.PMWorkTaskKinds()
}

func pmTaskQuery(db *gorm.DB) *gorm.DB {
	return db.Where("LOWER(TRIM(kind)) IN ?", pmTaskKinds())
}

func (s *Server) taskFromBody(
	project projectgorm.ProjectSet,
	requirement requirementgorm.Requirement,
	body taskBody,
) (taskgorm.WorkTask, error) {
	updates, err := s.taskUpdates(project, requirement.ID, 0, body)
	if err != nil {
		return taskgorm.WorkTask{}, err
	}
	task := taskgorm.WorkTask{
		WorkspaceID: project.WorkspaceID, TeamID: project.TeamID, ProjectSetID: project.ID,
		ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID,
		Kind: taskgorm.WorkTaskKindTask, Title: strings.TrimSpace(*body.Title), Status: "draft", Priority: "normal",
	}
	applyTaskUpdates(&task, updates)
	return task, nil
}

func (s *Server) taskUpdates(
	project projectgorm.ProjectSet, requirementID, currentTaskID uint, body taskBody,
) (map[string]any, error) {
	updates := map[string]any{}
	effectiveKind := taskgorm.WorkTaskKindTask
	var effectiveParent *uint
	var effectiveStartsAt, effectiveDueAt *time.Time
	if currentTaskID > 0 {
		var current taskgorm.WorkTask
		if err := s.db.Where(
			"id = ? AND workspace_id = ? AND project_set_id = ? AND requirement_id = ?",
			currentTaskID, project.WorkspaceID, project.ID, requirementID,
		).First(&current).Error; err != nil {
			return nil, fmt.Errorf("task not found")
		}
		effectiveKind = taskgorm.NormalizePMWorkTaskKind(current.Kind)
		effectiveParent = current.ParentID
		effectiveStartsAt, effectiveDueAt = current.StartsAt, current.DueAt
	}
	for key, value := range map[string]*string{
		"title": body.Title, "description": body.Description,
	} {
		if value != nil {
			updates[key] = strings.TrimSpace(*value)
		}
	}
	if body.Kind != nil {
		kind := taskgorm.NormalizePMWorkTaskKind(*body.Kind)
		if !taskgorm.IsPMWorkTaskKind(kind) {
			return nil, fmt.Errorf("kind must be task or subtask")
		}
		updates["kind"] = kind
		effectiveKind = kind
	}
	if body.Status != nil {
		if !validTaskStatus(*body.Status) {
			return nil, fmt.Errorf("status is invalid")
		}
		updates["status"] = strings.ToLower(strings.TrimSpace(*body.Status))
	}
	if body.Priority != nil {
		if !validTaskPriority(*body.Priority) {
			return nil, fmt.Errorf("priority is invalid")
		}
		updates["priority"] = normalizeTaskPriority(*body.Priority)
	}
	if body.TaskClientID.Set {
		if body.TaskClientID.Value != nil {
			if *body.TaskClientID.Value == 0 {
				return nil, fmt.Errorf("task_client_id must be positive")
			}
			var count int64
			if err := s.db.Model(&taskgorm.TaskClientOption{}).Where("workspace_id = ? AND id = ?", project.WorkspaceID, *body.TaskClientID.Value).Count(&count).Error; err != nil || count != 1 {
				return nil, fmt.Errorf("task_client_id is invalid")
			}
		}
		updates["task_client_id"] = body.TaskClientID.Value
	}
	if body.AssigneeID.Set {
		if body.AssigneeID.Value != nil && *body.AssigneeID.Value == 0 {
			return nil, fmt.Errorf("assignee_id must be positive")
		}
		updates["assignee_id"] = body.AssigneeID.Value
	}
	if body.ReporterUserID.Set {
		if body.ReporterUserID.Value != nil && *body.ReporterUserID.Value == 0 {
			return nil, fmt.Errorf("reporter_user_id must be positive")
		}
		updates["reporter_user_id"] = body.ReporterUserID.Value
	}
	if body.ParentID.Set {
		if body.ParentID.Value != nil {
			if *body.ParentID.Value == 0 || *body.ParentID.Value == currentTaskID {
				return nil, fmt.Errorf("parent_id is invalid")
			}
			var parent taskgorm.WorkTask
			query := pmTaskQuery(s.db).Where(
				"id = ? AND workspace_id = ? AND project_set_id = ? AND requirement_id = ?",
				*body.ParentID.Value, project.WorkspaceID, project.ID, requirementID,
			)
			query = applyWorkspaceHierarchyScope(query, "work_tasks", project.WorkspaceID)
			if query.First(&parent).Error != nil || taskgorm.NormalizePMWorkTaskKind(parent.Kind) != taskgorm.WorkTaskKindTask {
				return nil, fmt.Errorf("parent task does not belong to requirement workspace")
			}
		}
		updates["parent_id"] = body.ParentID.Value
		effectiveParent = body.ParentID.Value
	}
	if body.EstimatedMinutes.Set {
		if body.EstimatedMinutes.Value != nil && *body.EstimatedMinutes.Value < 0 {
			return nil, fmt.Errorf("estimated_minutes cannot be negative")
		}
		updates["estimated_minutes"] = body.EstimatedMinutes.Value
	}
	for key, value := range map[string]optionalValue[string]{
		"starts_at": body.StartsAt, "due_at": body.DueAt,
	} {
		if value.Set {
			parsed, err := parseOptionalTime(value.Value)
			if err != nil {
				return nil, err
			}
			updates[key] = parsed
			if key == "starts_at" {
				effectiveStartsAt = parsed
			} else {
				effectiveDueAt = parsed
			}
		}
	}
	if effectiveKind == taskgorm.WorkTaskKindSubtask && effectiveParent == nil {
		return nil, fmt.Errorf("subtask requires parent_id")
	}
	if effectiveKind == taskgorm.WorkTaskKindTask && effectiveParent != nil {
		return nil, fmt.Errorf("task cannot have parent_id")
	}
	if effectiveStartsAt != nil && effectiveDueAt != nil && effectiveStartsAt.After(*effectiveDueAt) {
		return nil, fmt.Errorf("starts_at cannot be after due_at")
	}
	return updates, nil
}

func applyTaskUpdates(task *taskgorm.WorkTask, updates map[string]any) {
	if value, ok := updates["title"].(string); ok {
		task.Title = value
	}
	if value, ok := updates["kind"].(string); ok && value != "" {
		task.Kind = value
	}
	if value, ok := updates["description"].(string); ok {
		task.Description = value
	}
	if value, ok := updates["status"].(string); ok {
		task.Status = value
	}
	if value, ok := updates["priority"].(string); ok {
		task.Priority = value
	}
	if value, exists := updates["task_client_id"]; exists {
		task.TaskClientID, _ = value.(*uint)
	}
	if value, exists := updates["assignee_id"]; exists {
		task.AssigneeID, _ = value.(*uint)
	}
	if value, exists := updates["reporter_user_id"]; exists {
		task.ReporterUserID, _ = value.(*uint)
	}
	if value, exists := updates["parent_id"]; exists {
		task.ParentID, _ = value.(*uint)
	}
	if value, exists := updates["estimated_minutes"]; exists {
		task.EstimatedMinutes = 0
		if typed, ok := value.(*int); ok && typed != nil {
			task.EstimatedMinutes = *typed
		}
	}
	if value, ok := updates["starts_at"]; ok {
		task.StartsAt, _ = value.(*time.Time)
	}
	if value, ok := updates["due_at"]; ok {
		task.DueAt, _ = value.(*time.Time)
	}
}

func taskBodyUserFactIDs(body taskBody) []uint {
	ids := make([]uint, 0, 2)
	for _, value := range []optionalValue[uint]{body.AssigneeID, body.ReporterUserID} {
		if value.Set && value.Value != nil && *value.Value > 0 {
			ids = append(ids, *value.Value)
		}
	}
	return ids
}

func applyTaskLifecycle(task *taskgorm.WorkTask, updates map[string]any) {
	status, changed := updates["status"].(string)
	if !changed {
		return
	}
	for _, terminal := range terminalTaskStatuses() {
		if normalizeStatus(status) == terminal {
			now := time.Now().UTC()
			updates["completed_at"] = &now
			task.CompletedAt = &now
			return
		}
	}
	updates["completed_at"] = nil
	task.CompletedAt = nil
}

func nextTaskKey(tx *gorm.DB, project projectgorm.ProjectSet) (string, error) {
	return taskgorm.Next(tx, taskgorm.ProjectScope{
		ID: project.ID, WorkspaceID: project.WorkspaceID, Code: project.Code,
	})
}
