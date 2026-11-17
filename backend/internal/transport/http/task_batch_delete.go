package httpapi

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func (s *Server) batchDeleteTasks(c *gin.Context) {
	var body struct {
		IDs     []uint `json:"ids"`
		Atomic  bool   `json:"atomic"`
		Cascade bool   `json:"cascade"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	ids := uniqueTaskIDs(body.IDs)
	if len(ids) == 0 {
		bad(c, "ids required")
		return
	}
	if len(ids) > 500 {
		bad(c, "too many ids")
		return
	}
	workspace, _ := currentWorkspace(c)
	// Resolve distinct project IDs and gate every project before proceeding.
	var tasks []taskgorm.WorkTask
	if err := s.db.Model(&taskgorm.WorkTask{}).Select("id", "workspace_id", "project_set_id").
		Where("workspace_id = ? AND id IN ?", workspace.ID, ids).
		Find(&tasks).Error; err != nil {
		dbFail(c)
		return
	}
	seen := map[uint]bool{}
	for _, task := range tasks {
		pid := task.ProjectSetID
		if !seen[pid] {
			if !s.requireProjectAccess(c, workspace.ID, pid) {
				return
			}
			seen[pid] = true
		}
	}
	creatorAllowed, accessValid := s.taskCreatorBatchAccess(
		c, accesscatalog.ActionWorkItemsTasksDelete, tasks,
	)
	if !accessValid {
		return
	}
	requestedIDs := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		requestedIDs[id] = struct{}{}
	}
	if body.Cascade {
		descendants, err := collectTaskDescendantsForRoots(s.db, workspace.ID, ids)
		if err != nil {
			dbFail(c)
			return
		}
		descendantIDs := make([]uint, 0, len(descendants))
		for _, descendant := range descendants {
			descendantIDs = append(descendantIDs, descendant.ID)
		}
		ids = uniqueTaskIDs(append(ids, descendantIDs...))
	}
	if body.Atomic {
		for _, task := range tasks {
			if !creatorAllowed[task.ID] {
				failCode(c, http.StatusForbidden, apiErrorAccessInterfaceForbidden, "service interface access is forbidden")
				return
			}
		}
		s.atomicBatchDeleteTasks(c, workspace.ID, ids)
		return
	}
	ordered := s.taskDeleteOrder(workspace.ID, ids)
	deleted, failures := 0, make([]gin.H, 0)
	for _, id := range ordered {
		var task taskgorm.WorkTask
		query := pmTaskQuery(s.db).Where("id = ? AND workspace_id = ?", id, workspace.ID)
		query = applyWorkspaceHierarchyScope(query, "work_tasks", workspace.ID)
		if query.First(&task).Error != nil {
			failures = append(failures, gin.H{"id": id, "error": "not found"})
			continue
		}
		_, explicitlyRequested := requestedIDs[task.ID]
		if explicitlyRequested && !creatorAllowed[task.ID] {
			failures = append(failures, gin.H{"id": id, "error": "forbidden"})
			continue
		}
		var project projectgorm.ProjectSet
		if s.db.Where("id = ? AND workspace_id = ?", task.ProjectSetID, workspace.ID).
			First(&project).Error != nil {
			failures = append(failures, gin.H{"id": id, "error": "not found"})
			continue
		}
		var childCount int64
		if s.db.Model(&taskgorm.WorkTask{}).Where(
			"workspace_id = ? AND parent_id = ?", workspace.ID, task.ID,
		).Count(&childCount).Error != nil {
			failures = append(failures, gin.H{"id": id, "error": "delete failed"})
			continue
		}
		if childCount > 0 {
			failures = append(failures, gin.H{"id": id, "error": "task has child tasks"})
			continue
		}
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := deleteWorkTaskDependents(tx, workspace.ID, []uint{task.ID}); err != nil {
				return err
			}
			if err := s.recordMutation(
				tx, c, project.ID, "work_task", task.ID, "deleted", task.Title, task,
			); err != nil {
				return err
			}
			result := tx.Where("id = ? AND workspace_id = ?", task.ID, workspace.ID).
				Delete(&taskgorm.WorkTask{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			return nil
		})
		if err != nil {
			failures = append(failures, gin.H{"id": id, "error": "delete failed"})
			continue
		}
		deleted++
	}
	ok(c, gin.H{"deleted": deleted, "failed": len(failures), "failures": failures})
}

func (s *Server) atomicBatchDeleteTasks(c *gin.Context, workspaceID uint, ids []uint) {
	var tasks []taskgorm.WorkTask
	query := pmTaskQuery(s.db).Where("work_tasks.id IN ? AND work_tasks.workspace_id = ?", ids, workspaceID)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspaceID)
	if err := query.Find(&tasks).Error; err != nil {
		dbFail(c)
		return
	}
	if len(tasks) != len(ids) {
		failCode(c, http.StatusNotFound, apiErrorTaskBatchNotFound, "task not found")
		return
	}
	selected := make(map[uint]struct{}, len(ids))
	projects := make(map[uint]projectgorm.ProjectSet)
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	for _, task := range tasks {
		if _, exists := projects[task.ProjectSetID]; exists {
			continue
		}
		var project projectgorm.ProjectSet
		if err := s.db.Where("id = ? AND workspace_id = ?", task.ProjectSetID, workspaceID).
			First(&project).Error; err != nil {
			failCode(c, http.StatusNotFound, apiErrorTaskBatchNotFound, "task not found")
			return
		}
		projects[project.ID] = project
	}
	var children []taskgorm.WorkTask
	childQuery := pmTaskQuery(s.db).Where("workspace_id = ? AND parent_id IN ?", workspaceID, ids)
	if err := childQuery.Find(&children).Error; err != nil {
		dbFail(c)
		return
	}
	missingDescendants := make([]uint, 0)
	for _, child := range children {
		if _, exists := selected[child.ID]; !exists {
			missingDescendants = append(missingDescendants, child.ID)
		}
	}
	if len(missingDescendants) > 0 {
		sort.Slice(missingDescendants, func(i, j int) bool { return missingDescendants[i] < missingDescendants[j] })
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"code": apiErrorTaskBatchDescendantsRequired, "error": "task descendants must be included",
			"missing_descendant_ids": missingDescendants,
		})
		return
	}
	byID := make(map[uint]taskgorm.WorkTask, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	ordered := s.taskDeleteOrder(workspaceID, ids)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteWorkTaskDependents(tx, workspaceID, ordered); err != nil {
			return err
		}
		for _, id := range ordered {
			task := byID[id]
			project := projects[task.ProjectSetID]
			if err := s.recordMutation(
				tx, c, project.ID, "work_task", task.ID, "deleted", task.Title, task,
			); err != nil {
				return err
			}
			result := tx.Where("id = ? AND workspace_id = ?", task.ID, workspaceID).
				Delete(&taskgorm.WorkTask{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"deleted": len(ordered), "failed": 0, "failures": []gin.H{}, "atomic": true,
		"deleted_ids": ordered, "affected_ids": ids, "affected_count": len(ordered),
	})
}

func uniqueTaskIDs(values []uint) []uint {
	result := make([]uint, 0, len(values))
	seen := map[uint]struct{}{}
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *Server) taskDeleteOrder(workspaceID uint, ids []uint) []uint {
	ordered := append([]uint(nil), ids...)
	var tasks []taskgorm.WorkTask
	pmTaskQuery(s.db).Select("id", "parent_id").Where(
		"workspace_id = ? AND id IN ?", workspaceID, ids,
	).Find(&tasks)
	parents := make(map[uint]*uint, len(tasks))
	for index := range tasks {
		parents[tasks[index].ID] = tasks[index].ParentID
	}
	depth := func(id uint) int {
		seen, value := map[uint]struct{}{}, 0
		for {
			parent, exists := parents[id]
			if !exists || parent == nil || *parent == 0 {
				return value
			}
			if _, loop := seen[*parent]; loop {
				return value
			}
			seen[*parent] = struct{}{}
			value++
			id = *parent
		}
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		return depth(ordered[left]) > depth(ordered[right])
	})
	return ordered
}
