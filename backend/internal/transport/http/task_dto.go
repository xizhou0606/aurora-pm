package httpapi

import (
	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func taskDTO(task taskgorm.WorkTask) gin.H {
	return gin.H{
		"id": task.ID, "workspace_id": task.WorkspaceID, "team_id": task.TeamID,
		"project_set_id": task.ProjectSetID, "project_id": task.ProjectID,
		"requirement_id": task.RequirementID, "kind": task.Kind, "task_key": task.TaskKey,
		"task_client_id": task.TaskClientID,
		"title":          task.Title, "description": task.Description, "status": task.Status,
		"priority": task.Priority, "assignee_id": task.AssigneeID,
		"reporter_user_id":  task.ReporterUserID,
		"parent_id":         task.ParentID,
		"estimated_minutes": task.EstimatedMinutes,
		"starts_at":         task.StartsAt, "due_at": task.DueAt,
		"allowed_status_transitions": standardAllowedTaskStatuses(),
		"completed_at":               task.CompletedAt, "created_at": task.CreatedAt, "updated_at": task.UpdatedAt,
	}
}

func (s *Server) taskByID(c *gin.Context, id uint) (taskgorm.WorkTask, projectgorm.ProjectSet, bool) {
	workspace, _ := currentWorkspace(c)
	var task taskgorm.WorkTask
	query := pmTaskQuery(s.db).Where("id = ? AND workspace_id = ?", id, workspace.ID)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspace.ID)
	if query.
		First(&task).Error != nil {
		fail(c, 404, "task not found")
		return taskgorm.WorkTask{}, projectgorm.ProjectSet{}, false
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", task.ProjectSetID, workspace.ID).First(&project).Error != nil {
		fail(c, 404, "project set not found")
		return taskgorm.WorkTask{}, projectgorm.ProjectSet{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return taskgorm.WorkTask{}, projectgorm.ProjectSet{}, false
	}
	return task, project, true
}
