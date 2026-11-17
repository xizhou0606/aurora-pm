package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func (s *Server) listWorkspaceTasks(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	scope, err := s.parseWorkspaceScope(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	query := applyWorkspaceScope(
		pmTaskQuery(s.db).Where("workspace_id = ?", workspace.ID), scope,
	)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspace.ID)
	query = applyWorkspaceTaskFilters(c, query)
	query, err = applyWorkspaceTaskContractFilters(c, query)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if !s.applyProjectDataFilter(c, query, "work_tasks", "project_set_id") {
		return
	}
	order, treeOrder, err := workspaceTaskOrder(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if hierarchy := strings.ToLower(strings.TrimSpace(c.Query("hierarchy"))); hierarchy != "" {
		if hierarchy != "filtered_tree" {
			bad(c, "hierarchy is invalid")
			return
		}
		s.listWorkspaceTaskFilteredTree(c, workspace.ID, scope, query, order, treeOrder)
		return
	}
	var total int64
	if query.Model(&taskgorm.WorkTask{}).Count(&total).Error != nil {
		dbFail(c)
		return
	}
	limit, offset := pagination(c, 50)
	var tasks []taskgorm.WorkTask
	if query.Order(order).Limit(limit).Offset(offset).Find(&tasks).Error != nil {
		dbFail(c)
		return
	}
	context := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withTasks(tasks),
	)
	rows := s.workspaceTaskRows(c, workspace.ID, context, tasks, nil)
	ok(c, gin.H{"items": rows, "total": total})
}

func workspaceTaskOrder(c *gin.Context) (string, string, error) {
	order := "id DESC"
	treeOrder := "MAX(id) DESC"
	switch strings.ToLower(strings.TrimSpace(c.Query("sort"))) {
	case "created_at_desc":
		order = "created_at DESC, id DESC"
		treeOrder = "MAX(created_at) DESC, MAX(id) DESC"
	case "created_at_asc":
		order = "created_at ASC, id ASC"
		treeOrder = "MIN(created_at) ASC, MIN(id) ASC"
	case "updated_at_asc":
		order = "updated_at ASC, id ASC"
		treeOrder = "MIN(updated_at) ASC, MIN(id) ASC"
	case "":
	case "updated_at_desc":
		order = "updated_at DESC, id DESC"
		treeOrder = "MAX(updated_at) DESC, MAX(id) DESC"
	default:
		return "", "", fmt.Errorf("sort is invalid")
	}
	switch strings.ToLower(strings.TrimSpace(c.Query("group_by"))) {
	case "":
	case "requirement_id":
		order = "requirement_id ASC, " + order
		treeOrder = "MIN(requirement_id) ASC, " + treeOrder
	case "assignee_id":
		order = "assignee_id ASC, " + order
		treeOrder = "MIN(assignee_id) ASC, " + treeOrder
	case "priority":
		order = "priority ASC, " + order
		treeOrder = "MIN(priority) ASC, " + treeOrder
	default:
		return "", "", fmt.Errorf("group_by is invalid")
	}
	return order, treeOrder, nil
}

func (s *Server) workspaceTaskRows(
	c *gin.Context,
	workspaceID uint,
	context workspaceContext,
	tasks []taskgorm.WorkTask,
	matchedIDs map[uint]struct{},
) []gin.H {
	taskIDs := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	watched := s.taskWatchSet(workspaceID, currentFactUserID(c), taskIDs)
	hierarchy := s.loadWorkspaceTaskHierarchyContext(workspaceID, tasks)
	rows := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		if _, ok := context.ProjectSets[task.ProjectSetID]; !ok {
			continue
		}
		row := s.enrichWorkspaceTaskContract(taskDTO(task), task, context, hierarchy)
		row["watched"] = watched[task.ID]
		if matchedIDs != nil {
			_, matched := matchedIDs[task.ID]
			row["filter_match"] = matched
			row["context_only"] = !matched
		}
		rows = append(rows, row)
	}
	return rows
}

func applyWorkspaceTaskFilters(c *gin.Context, query *gorm.DB) *gorm.DB {
	if values := queryUintLists(c, "task_client_id", "task_client_ids"); len(values) > 0 {
		query = query.Where("task_client_id IN ?", values)
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := escapedLikePattern(q)
		query = query.Where(
			"LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(task_key) LIKE ? ESCAPE '\\' OR LOWER(description) LIKE ? ESCAPE '\\'",
			like, like, like,
		)
	}
	if values := queryCSVStrings(c, "status", "statuses"); len(values) > 0 {
		query = query.Where("LOWER(status) IN ?", values)
	}
	if values := queryCSVStrings(c, "priority", "priorities"); len(values) > 0 {
		query = query.Where("LOWER(priority) IN ?", values)
	}
	if values := queryUintLists(c, "assignee_id", "assignee_ids"); len(values) > 0 {
		query = query.Where("assignee_id IN ?", values)
	}
	if values := queryCSVStrings(c, "kinds", "filter_kinds", "kind"); len(values) > 0 {
		query = query.Where("LOWER(TRIM(kind)) IN ?", values)
	}
	if parentID := queryUint(c, "parent_id"); parentID > 0 {
		query = query.Where("parent_id = ?", parentID)
	}
	bucket := strings.ToLower(strings.TrimSpace(c.Query("due_bucket")))
	if bucket == "" && c.Query("overdue") == "1" {
		bucket = "overdue"
	}
	if bucket != "" {
		now := time.Now().UTC()
		overdueBefore := taskBusinessDayStart(c, now)
		switch bucket {
		case "overdue":
			query = query.Where("LOWER(TRIM(status)) NOT IN ?", terminalTaskStatuses()).
				Where("due_at IS NOT NULL AND due_at < ?", overdueBefore)
		case "due_soon":
			query = query.Where("LOWER(TRIM(status)) NOT IN ?", terminalTaskStatuses()).
				Where("due_at IS NOT NULL AND due_at >= ? AND due_at < ?", now, now.AddDate(0, 0, 7))
		case "later":
			query = query.Where("due_at IS NOT NULL AND due_at >= ?", now.AddDate(0, 0, 7))
		case "unset":
			query = query.Where("due_at IS NULL")
		default:
			query = query.Where("1 = 0")
		}
	}
	if c.Query("unfinished") == "1" {
		query = query.Where("LOWER(TRIM(status)) NOT IN ?", terminalTaskStatuses())
	}
	return query
}

func taskBusinessDayStart(c *gin.Context, now time.Time) time.Time {
	location := biLocation(c)
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).UTC()
}

func queryCSVStrings(c *gin.Context, names ...string) []string {
	seen := map[string]struct{}{}
	values := make([]string, 0)
	for _, name := range names {
		for _, raw := range c.Request.URL.Query()[name] {
			for _, value := range parseCSVStrings(raw) {
				if _, exists := seen[value]; exists {
					continue
				}
				seen[value] = struct{}{}
				values = append(values, value)
			}
		}
	}
	return values
}

func queryUintLists(c *gin.Context, names ...string) []uint {
	seen := map[uint]struct{}{}
	values := make([]uint, 0)
	for _, name := range names {
		for _, raw := range c.Request.URL.Query()[name] {
			for _, part := range strings.Split(raw, ",") {
				value, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
				if err != nil || value == 0 {
					continue
				}
				id := uint(value)
				if _, exists := seen[id]; exists {
					continue
				}
				seen[id] = struct{}{}
				values = append(values, id)
			}
		}
	}
	return values
}

func (s *Server) getWorkspaceTask(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	task, _, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	workspace, _ := currentWorkspace(c)
	context := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withTask(task),
	)
	hierarchy := s.loadWorkspaceTaskHierarchyContext(workspace.ID, []taskgorm.WorkTask{task})
	row := s.enrichWorkspaceTaskContract(taskDTO(task), task, context, hierarchy)
	row["watched"] = s.userWatchesTask(task.WorkspaceID, task.ID, currentFactUserID(c))
	ok(c, row)
}

func (s *Server) taskWorkspaceStats(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	scope, err := s.parseWorkspaceScope(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	query := applyWorkspaceScope(pmTaskQuery(s.db).Where("workspace_id = ?", workspace.ID), scope)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspace.ID)
	query = applyWorkspaceTaskFilters(c, query)
	if !s.applyProjectDataFilter(c, query, "work_tasks", "project_set_id") {
		return
	}
	dueSoonDays := 7
	if raw := strings.TrimSpace(c.Query("due_soon_days")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 1 && value <= 90 {
			dueSoonDays = value
		}
	}
	now := time.Now().UTC()
	until := now.AddDate(0, 0, dueSoonDays)
	var total, completed, overdue, dueSoon int64
	terminal := terminalTaskStatuses()
	if query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).Count(&total).Error != nil ||
		query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
			Where("LOWER(TRIM(status)) IN ?", terminal).Count(&completed).Error != nil ||
		query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
			Where("LOWER(TRIM(status)) NOT IN ?", terminal).
			Where("due_at IS NOT NULL AND due_at < ?", now).Count(&overdue).Error != nil ||
		query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
			Where("LOWER(TRIM(status)) NOT IN ?", terminal).
			Where("due_at IS NOT NULL AND due_at >= ? AND due_at <= ?", now, until).
			Count(&dueSoon).Error != nil {
		dbFail(c)
		return
	}
	type statusCount struct {
		Status string
		Count  int64
	}
	var counts []statusCount
	if query.Model(&taskgorm.WorkTask{}).Select("LOWER(status) AS status, COUNT(*) AS count").
		Group("LOWER(status)").Scan(&counts).Error != nil {
		dbFail(c)
		return
	}
	byStatus := gin.H{}
	for _, row := range counts {
		byStatus[row.Status] = row.Count
	}
	type kindCount struct {
		Kind  string
		Count int64
	}
	var kinds, completedKinds []kindCount
	if query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
		Select("LOWER(TRIM(kind)) AS kind, COUNT(*) AS count").
		Group("LOWER(TRIM(kind))").Scan(&kinds).Error != nil ||
		query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
			Where("LOWER(TRIM(status)) IN ?", terminal).
			Select("LOWER(TRIM(kind)) AS kind, COUNT(*) AS count").
			Group("LOWER(TRIM(kind))").Scan(&completedKinds).Error != nil {
		dbFail(c)
		return
	}
	byKind, byKindCompleted := gin.H{}, gin.H{}
	for _, row := range kinds {
		byKind[row.Kind] = row.Count
	}
	for _, row := range completedKinds {
		byKindCompleted[row.Kind] = row.Count
	}
	ok(c, gin.H{
		"total": total, "completed": completed, "incomplete": total - completed,
		"overdue": overdue, "due_soon": dueSoon, "due_soon_days": dueSoonDays,
		"by_kind": byKind, "by_kind_completed": byKindCompleted,
		"by_status": byStatus,
	})
}

func workspaceTaskNotFound(c *gin.Context) {
	fail(c, http.StatusNotFound, "task not found")
}
