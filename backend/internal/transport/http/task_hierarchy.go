package httpapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func collectTaskDescendants(db *gorm.DB, workspaceID, rootID uint) ([]taskgorm.WorkTask, error) {
	return collectTaskDescendantsForRoots(db, workspaceID, []uint{rootID})
}

func collectTaskDescendantsForRoots(db *gorm.DB, workspaceID uint, rootIDs []uint) ([]taskgorm.WorkTask, error) {
	if workspaceID == 0 || len(rootIDs) == 0 {
		return []taskgorm.WorkTask{}, nil
	}
	seen := make(map[uint]bool, len(rootIDs))
	frontier := make([]uint, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		if rootID == 0 || seen[rootID] {
			continue
		}
		seen[rootID] = true
		frontier = append(frontier, rootID)
	}
	result := make([]taskgorm.WorkTask, 0)
	for len(frontier) > 0 {
		var children []taskgorm.WorkTask
		if err := pmTaskQuery(db).Where(
			"workspace_id = ? AND parent_id IN ?", workspaceID, frontier,
		).Order("id ASC").Find(&children).Error; err != nil {
			return nil, err
		}
		frontier = frontier[:0]
		for _, child := range children {
			if seen[child.ID] {
				continue
			}
			seen[child.ID] = true
			result = append(result, child)
			frontier = append(frontier, child.ID)
			if len(result) > 10000 {
				return nil, fmt.Errorf("task hierarchy is too deep")
			}
		}
	}
	return result, nil
}

func resolveInheritedTaskDueAt(db *gorm.DB, workspaceID uint, parentID *uint) (*time.Time, error) {
	seen := map[uint]bool{}
	for parentID != nil && *parentID > 0 {
		if seen[*parentID] {
			return nil, fmt.Errorf("task hierarchy cycle detected")
		}
		seen[*parentID] = true
		var parent taskgorm.WorkTask
		if err := pmTaskQuery(db).Where(
			"id = ? AND workspace_id = ?", *parentID, workspaceID,
		).First(&parent).Error; err != nil {
			return nil, err
		}
		if parent.DueAt != nil {
			value := parent.DueAt.UTC()
			return &value, nil
		}
		parentID = parent.ParentID
	}
	return nil, nil
}

func (s *Server) inheritTaskDueOnCreate(tx *gorm.DB, task *taskgorm.WorkTask) error {
	if task == nil || task.DueAt != nil || task.ParentID == nil {
		return nil
	}
	due, err := resolveInheritedTaskDueAt(tx, task.WorkspaceID, task.ParentID)
	if err != nil {
		return err
	}
	if due != nil {
		task.DueAt = due
	}
	return nil
}

func (s *Server) propagateTaskDueToUnsetDescendants(
	tx *gorm.DB, root taskgorm.WorkTask, actorID uint,
) error {
	descendants, err := collectTaskDescendants(tx, root.WorkspaceID, root.ID)
	if err != nil {
		return err
	}
	for _, descendant := range descendants {
		if descendant.DueAt != nil || descendant.ParentID == nil {
			continue
		}
		due, err := resolveInheritedTaskDueAt(tx, root.WorkspaceID, descendant.ParentID)
		if err != nil {
			return err
		}
		if due == nil {
			continue
		}
		if err := tx.Model(&taskgorm.WorkTask{}).Where(
			"id = ? AND workspace_id = ?", descendant.ID, root.WorkspaceID,
		).Update("due_at", due).Error; err != nil {
			return err
		}
		if err := tx.Create(&taskgorm.WorkTaskActivity{
			WorkspaceID: root.WorkspaceID, WorkTaskID: descendant.ID, UserID: actorID,
			Action: "auto_inherit_due", FieldKey: "due_at", NewValue: due.Format(time.RFC3339),
			Summary: "截止日期已继承自父项",
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) listTaskDueAtDescendantConflicts(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, _, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	if !s.requireTaskCreatorAction(c, accesscatalog.ActionWorkItemsTasksUpdate, task) {
		return
	}
	raw := strings.TrimSpace(c.Query("due_at"))
	proposed, err := parseOptionalTime(&raw)
	if err != nil || proposed == nil {
		bad(c, "due_at required")
		return
	}
	descendants, err := collectTaskDescendants(s.db, task.WorkspaceID, task.ID)
	if err != nil {
		dbFail(c)
		return
	}
	items := make([]gin.H, 0)
	for _, descendant := range descendants {
		if descendant.DueAt != nil && descendant.DueAt.After(*proposed) {
			items = append(items, gin.H{
				"id": descendant.ID, "task_key": descendant.TaskKey,
				"title": descendant.Title, "due_at": descendant.DueAt,
			})
		}
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left]["id"].(uint) < items[right]["id"].(uint)
	})
	ok(c, gin.H{"items": items})
}

func (s *Server) autoCompleteTaskAncestors(
	tx *gorm.DB, workspaceID uint, parentID *uint, actorID uint,
) error {
	seen := map[uint]bool{}
	for parentID != nil && *parentID > 0 {
		if seen[*parentID] {
			return fmt.Errorf("task hierarchy cycle detected")
		}
		seen[*parentID] = true
		var parent taskgorm.WorkTask
		if err := pmTaskQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"})).Where(
			"id = ? AND workspace_id = ?", *parentID, workspaceID,
		).First(&parent).Error; err != nil {
			return err
		}
		var children []taskgorm.WorkTask
		if err := pmTaskQuery(tx).Where(
			"workspace_id = ? AND parent_id = ?", workspaceID, parent.ID,
		).Find(&children).Error; err != nil {
			return err
		}
		if len(children) == 0 {
			return nil
		}
		allTerminal := true
		for _, child := range children {
			if !isTerminalTaskStatus(child.Status) {
				allTerminal = false
				break
			}
		}
		if !allTerminal {
			return nil
		}
		if !isTerminalTaskStatus(parent.Status) {
			now := time.Now().UTC()
			if err := tx.Model(&taskgorm.WorkTask{}).Where(
				"id = ? AND workspace_id = ?", parent.ID, workspaceID,
			).Updates(map[string]any{"status": "completed", "completed_at": &now}).Error; err != nil {
				return err
			}
			if err := tx.Create(&taskgorm.WorkTaskActivity{
				WorkspaceID: workspaceID, WorkTaskID: parent.ID, UserID: actorID,
				Action: "auto_complete", FieldKey: "status", OldValue: parent.Status,
				NewValue: "completed", Summary: "所有子项完成，父项已自动完成",
			}).Error; err != nil {
				return err
			}

			previous := parent.Status
			parent.Status = "completed"
			if err := s.recordTaskStatusNotification(tx, parent, previous, actorID); err != nil {
				return err
			}
		}
		parentID = parent.ParentID
	}
	return nil
}

func isTerminalTaskStatus(status string) bool {
	value := normalizeStatus(status)
	for _, terminal := range terminalTaskStatuses() {
		if value == terminal {
			return true
		}
	}
	return false
}

func taskFieldOldValue(task taskgorm.WorkTask, field string) any {
	switch field {
	case "title":
		return task.Title
	case "description":
		return task.Description
	case "status":
		return task.Status
	case "priority":
		return task.Priority
	case "assignee_id":
		return task.AssigneeID
	case "reporter_user_id":
		return task.ReporterUserID
	case "parent_id":
		return task.ParentID
	case "estimated_minutes":
		return task.EstimatedMinutes
	case "starts_at":
		return formatTaskActivityTime(task.StartsAt)
	case "due_at":
		return formatTaskActivityTime(task.DueAt)
	case "requirement_id":
		return task.RequirementID
	default:
		return ""
	}
}

func formatTaskActivityTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func taskUpdateActivityRows(
	before taskgorm.WorkTask, updates map[string]any, actorID uint,
) []taskgorm.WorkTaskActivity {
	fields := []string{
		"title", "description", "status", "priority", "assignee_id", "reporter_user_id", "parent_id",
		"requirement_id", "estimated_minutes", "starts_at", "due_at",
	}
	rows := make([]taskgorm.WorkTaskActivity, 0, len(updates))
	for _, field := range fields {
		value, changed := updates[field]
		if !changed {
			continue
		}
		oldValue := taskActivityValue(taskFieldOldValue(before, field))
		newValue := taskActivityValue(value)
		if typed, ok := value.(*time.Time); ok {
			newValue = formatTaskActivityTime(typed)
		}
		if oldValue == newValue {
			continue
		}
		action := "field_change"
		if field == "status" {
			action = "status_change"
		}
		summary := fmt.Sprintf("%s：%s → %s", taskFieldLabel(field), oldValue, newValue)
		if field == "description" {
			oldValue, newValue, summary = "", "", "说明已修改"
		}
		rows = append(rows, taskgorm.WorkTaskActivity{
			WorkspaceID: before.WorkspaceID, WorkTaskID: before.ID, UserID: actorID,
			Action: action, FieldKey: field, OldValue: oldValue, NewValue: newValue,
			Summary: summary,
		})
	}
	return rows
}

func taskMoveNotAllowed(c *gin.Context) {
	fail(c, http.StatusBadRequest, "subtask scope follows its parent")
}
