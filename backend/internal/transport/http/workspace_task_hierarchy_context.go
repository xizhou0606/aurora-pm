package httpapi

import (
	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type workspaceTaskHierarchyContext struct {
	tasksByID     map[uint]taskgorm.WorkTask
	childrenCount map[uint]int64
	creatorIDs    map[uint]uint
}

type workspaceTaskChildCount struct {
	ParentID uint  `gorm:"column:parent_id"`
	Count    int64 `gorm:"column:count"`
}

func (s *Server) loadWorkspaceTaskHierarchyContext(
	workspaceID uint,
	tasks []taskgorm.WorkTask,
) workspaceTaskHierarchyContext {
	result := workspaceTaskHierarchyContext{
		tasksByID: make(map[uint]taskgorm.WorkTask), childrenCount: make(map[uint]int64),
		creatorIDs: make(map[uint]uint),
	}
	if workspaceID == 0 || len(tasks) == 0 {
		return result
	}
	requestedIDs := make([]uint, 0, len(tasks))
	frontier := make([]uint, 0, len(tasks))
	seenParents := make(map[uint]struct{})
	for _, task := range tasks {
		result.tasksByID[task.ID] = task
		requestedIDs = append(requestedIDs, task.ID)
		if task.ParentID != nil && *task.ParentID > 0 {
			frontier = append(frontier, *task.ParentID)
		}
	}
	result.creatorIDs = s.taskCreatorIDs(workspaceID, requestedIDs)
	for len(frontier) > 0 && len(seenParents) <= 10000 {
		ids := make([]uint, 0, len(frontier))
		for _, id := range frontier {
			if id == 0 {
				continue
			}
			if _, exists := seenParents[id]; exists {
				continue
			}
			seenParents[id] = struct{}{}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			break
		}
		var parents []taskgorm.WorkTask
		query := pmTaskQuery(s.db).Where("workspace_id = ? AND id IN ?", workspaceID, ids)
		query = applyWorkspaceHierarchyScope(query, "work_tasks", workspaceID)
		if query.Find(&parents).Error != nil {
			break
		}
		frontier = frontier[:0]
		for _, parent := range parents {
			result.tasksByID[parent.ID] = parent
			if parent.ParentID != nil && *parent.ParentID > 0 {
				frontier = append(frontier, *parent.ParentID)
			}
		}
	}
	var counts []workspaceTaskChildCount
	query := pmTaskQuery(s.db).Model(&taskgorm.WorkTask{}).
		Select("parent_id, COUNT(*) AS count").
		Where("workspace_id = ? AND parent_id IN ?", workspaceID, requestedIDs).
		Group("parent_id")
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspaceID)
	if query.Scan(&counts).Error == nil {
		for _, count := range counts {
			result.childrenCount[count.ParentID] = count.Count
		}
	}
	return result
}

func (ctx workspaceTaskHierarchyContext) enrich(row gin.H, task taskgorm.WorkTask) gin.H {
	ancestors := make([]taskgorm.WorkTask, 0)
	seen := map[uint]struct{}{task.ID: {}}
	parentID := task.ParentID
	for parentID != nil && *parentID > 0 {
		if _, loop := seen[*parentID]; loop {
			break
		}
		seen[*parentID] = struct{}{}
		parent, found := ctx.tasksByID[*parentID]
		if !found {
			break
		}
		ancestors = append(ancestors, parent)
		parentID = parent.ParentID
	}
	ancestorRows := make([]gin.H, 0, len(ancestors))
	for index := len(ancestors) - 1; index >= 0; index-- {
		ancestorRows = append(ancestorRows, taskHierarchySummary(ancestors[index]))
	}
	var parent any
	if task.ParentID != nil && *task.ParentID > 0 {
		if value, found := ctx.tasksByID[*task.ParentID]; found {
			parent = taskHierarchySummary(value)
		}
	}
	row["hierarchy"] = gin.H{
		"parent": parent, "ancestors": ancestorRows, "depth": len(ancestorRows),
		"children_count": ctx.childrenCount[task.ID],
	}
	return row
}

func taskHierarchySummary(task taskgorm.WorkTask) gin.H {
	return gin.H{
		"id": task.ID, "task_key": task.TaskKey, "title": task.Title, "kind": task.Kind,
	}
}

func (s *Server) enrichWorkspaceTaskContract(
	row gin.H,
	task taskgorm.WorkTask,
	context workspaceContext,
	hierarchy workspaceTaskHierarchyContext,
) gin.H {
	row = context.enrichTask(row, task)
	row = context.enrichTaskCreator(row, hierarchy.creatorIDs[task.ID])
	return hierarchy.enrich(row, task)
}
