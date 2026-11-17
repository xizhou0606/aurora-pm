package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

const workspaceTaskRootExpression = "CASE WHEN parent_id IS NULL OR parent_id = 0 THEN id ELSE parent_id END"

type workspaceTaskRoot struct {
	RootID uint `gorm:"column:root_id"`
}

// listWorkspaceTaskFilteredTree paginates distinct root tasks, then returns
// matching rows plus the hierarchy context required to operate on them. Search
// results include the complete descendant tree so deleting a matched parent
// cannot lose the explicit cascade signal in the list presentation.
func (s *Server) listWorkspaceTaskFilteredTree(
	c *gin.Context,
	workspaceID uint,
	scope workspaceScope,
	matchedQuery *gorm.DB,
	rowOrder string,
	rootOrder string,
) {
	var matchedTotal int64
	if matchedQuery.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).Count(&matchedTotal).Error != nil {
		dbFail(c)
		return
	}

	rootSetQuery := matchedQuery.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
		Select(workspaceTaskRootExpression + " AS root_id").
		Group(workspaceTaskRootExpression)
	var rootTotal int64
	if s.db.Table("(?) AS matched_roots", rootSetQuery).Count(&rootTotal).Error != nil {
		dbFail(c)
		return
	}

	limit, offset := pagination(c, 50)
	var roots []workspaceTaskRoot
	if matchedQuery.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
		Select(workspaceTaskRootExpression+" AS root_id").
		Group(workspaceTaskRootExpression).
		Order(rootOrder).
		Limit(limit).
		Offset(offset).
		Scan(&roots).Error != nil {
		dbFail(c)
		return
	}
	if len(roots) == 0 {
		ok(c, gin.H{
			"items":        []gin.H{},
			"total":        rootTotal,
			"match_total":  matchedTotal,
			"root_total":   rootTotal,
			"page_unit":    "root_task",
			"match_policy": "filtered_tree",
		})
		return
	}

	rootIDs := make([]uint, 0, len(roots))
	for _, root := range roots {
		rootIDs = append(rootIDs, root.RootID)
	}
	var matched []taskgorm.WorkTask
	if matchedQuery.Session(&gorm.Session{}).
		Where(workspaceTaskRootExpression+" IN ?", rootIDs).
		Order(rowOrder).
		Find(&matched).Error != nil {
		dbFail(c)
		return
	}
	displayTasks := matched
	if strings.TrimSpace(c.Query("q")) != "" {
		descendants, err := collectTaskDescendantsForRoots(s.db, workspaceID, rootIDs)
		if err != nil {
			dbFail(c)
			return
		}
		seen := make(map[uint]struct{}, len(matched)+len(descendants))
		for _, task := range matched {
			seen[task.ID] = struct{}{}
		}
		for _, task := range descendants {
			if _, exists := seen[task.ID]; exists {
				continue
			}
			seen[task.ID] = struct{}{}
			displayTasks = append(displayTasks, task)
		}
	}

	rootQuery := applyWorkspaceScope(
		pmTaskQuery(s.db).Where("workspace_id = ? AND id IN ?", workspaceID, rootIDs),
		scope,
	)
	rootQuery = applyWorkspaceHierarchyScope(rootQuery, "work_tasks", workspaceID)
	var rootRows []taskgorm.WorkTask
	if rootQuery.Find(&rootRows).Error != nil {
		dbFail(c)
		return
	}

	matchedIDs := make(map[uint]struct{}, len(matched))
	for _, task := range matched {
		matchedIDs[task.ID] = struct{}{}
	}
	rootByID := make(map[uint]taskgorm.WorkTask, len(rootRows))
	for _, task := range rootRows {
		rootByID[task.ID] = task
	}
	parentByID := make(map[uint]uint, len(rootRows)+len(displayTasks))
	for _, task := range append(rootRows, displayTasks...) {
		if task.ParentID != nil && *task.ParentID > 0 {
			parentByID[task.ID] = *task.ParentID
		}
	}
	rootIDFor := func(task taskgorm.WorkTask) uint {
		current := task.ID
		seen := map[uint]struct{}{}
		for {
			if _, exists := rootByID[current]; exists {
				return current
			}
			if _, exists := seen[current]; exists {
				return task.ID
			}
			seen[current] = struct{}{}
			parentID, exists := parentByID[current]
			if !exists || parentID == 0 {
				return current
			}
			current = parentID
		}
	}
	displayByRoot := make(map[uint][]taskgorm.WorkTask, len(rootIDs))
	for _, task := range displayTasks {
		rootID := rootIDFor(task)
		displayByRoot[rootID] = append(displayByRoot[rootID], task)
	}

	ordered := make([]taskgorm.WorkTask, 0, len(displayTasks)+len(rootRows))
	seen := make(map[uint]struct{}, len(displayTasks)+len(rootRows))
	appendTask := func(task taskgorm.WorkTask) {
		if _, exists := seen[task.ID]; exists {
			return
		}
		seen[task.ID] = struct{}{}
		ordered = append(ordered, task)
	}
	for _, rootID := range rootIDs {
		if root, exists := rootByID[rootID]; exists {
			appendTask(root)
		}
		for _, task := range displayByRoot[rootID] {
			appendTask(task)
		}
	}

	context := s.loadWorkspaceContext(
		workspaceID,
		workspaceContextReferences{}.withTasks(ordered),
	)
	ok(c, gin.H{
		"items":        s.workspaceTaskRows(c, workspaceID, context, ordered, matchedIDs),
		"total":        rootTotal,
		"match_total":  matchedTotal,
		"root_total":   rootTotal,
		"page_unit":    "root_task",
		"match_policy": "filtered_tree",
	})
}
