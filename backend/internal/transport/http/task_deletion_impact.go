package httpapi

import (
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func (s *Server) getTaskDeletionImpact(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	root, _, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	if !s.requireTaskCreatorAction(c, accesscatalog.ActionWorkItemsTasksDelete, root) {
		return
	}
	descendants, err := collectTaskDescendants(s.db, root.WorkspaceID, root.ID)
	if err != nil {
		dbFail(c)
		return
	}
	tasks := make([]taskgorm.WorkTask, 0, len(descendants)+1)
	tasks = append(tasks, root)
	tasks = append(tasks, descendants...)
	affectedIDs := make([]uint, 0, len(tasks))
	descendantRows := make([]gin.H, 0, len(descendants))
	for index, task := range tasks {
		affectedIDs = append(affectedIDs, task.ID)
		if index > 0 {
			descendantRows = append(descendantRows, taskDeletionSummary(task))
		}
	}
	related, err := s.taskDeletionRelatedCounts(root.WorkspaceID, affectedIDs)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"root": taskDeletionSummary(root), "descendants": descendantRows,
		"affected_ids": affectedIDs, "affected_count": len(affectedIDs),
		"related": related, "cascade_required": len(descendants) > 0,
	})
}

func taskDeletionSummary(task taskgorm.WorkTask) gin.H {
	return gin.H{
		"id": task.ID, "workspace_id": task.WorkspaceID, "team_id": task.TeamID,
		"project_set_id": task.ProjectSetID, "project_id": task.ProjectID,
		"requirement_id": task.RequirementID, "task_key": task.TaskKey, "title": task.Title,
		"kind": task.Kind, "parent_id": task.ParentID,
	}
}

func (s *Server) taskDeletionRelatedCounts(
	workspaceID uint,
	taskIDs []uint,
) (gin.H, error) {
	counts := gin.H{}
	for key, target := range map[string]any{
		"comments": &taskgorm.WorkTaskComment{}, "attachments": &taskgorm.WorkTaskAttachment{},
		"activities": &taskgorm.WorkTaskActivity{}, "watchers": &taskgorm.WorkTaskWatcher{},
	} {
		var count int64
		if err := s.db.Model(target).Where(
			"workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs,
		).Count(&count).Error; err != nil {
			return nil, err
		}
		counts[key] = count
	}
	var qualityReferences int64
	if err := s.db.Model(&qualitygorm.QualityBug{}).Where(
		"workspace_id = ? AND (source_execution_id IN ? OR source_test_id IN ?)",
		workspaceID, taskIDs, taskIDs,
	).Count(&qualityReferences).Error; err != nil {
		return nil, err
	}
	counts["quality_source_references"] = qualityReferences
	return counts, nil
}
