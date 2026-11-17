package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func (s *Server) workbenchTodoQueries(
	c *gin.Context,
) (*gorm.DB, *gorm.DB, workspaceScope, error) {
	workspace, _ := currentWorkspace(c)
	a, _ := currentActor(c)
	userID := factUserID(a.User)
	scope, err := s.parseWorkspaceScope(c)
	if err != nil {
		return nil, nil, scope, err
	}
	tasks := applyWorkspaceScope(
		pmTaskQuery(s.db).Where(
			"workspace_id = ? AND assignee_id = ? AND LOWER(status) NOT IN ?",
			workspace.ID, userID, terminalTaskStatuses(),
		), scope,
	)
	tasks = applyWorkspaceHierarchyScope(tasks, "work_tasks", workspace.ID)
	if !s.applyProjectDataFilter(c, tasks, "work_tasks", "project_set_id") {
		return nil, nil, scope, nil
	}
	terminalBugStatuses := qualityBugStatusesStorageAliases("done", "canceled")
	reviewerOwnedStatuses := qualityBugStatusesStorageAliases("review", "done")
	bugs := applyWorkspaceScope(
		s.db.Model(&qualitygorm.QualityBug{}).Where(
			"workspace_id = ? AND LOWER(TRIM(status)) NOT IN ? AND "+
				"((LOWER(TRIM(status)) IN ? AND COALESCE(reviewer_user_id, reporter_user_id) = ?) OR "+
				"((LOWER(TRIM(status)) NOT IN ? OR status IS NULL) AND assignee_user_id = ?))",
			workspace.ID, terminalBugStatuses,
			reviewerOwnedStatuses, userID, reviewerOwnedStatuses, userID,
		), scope,
	)
	bugs = applyWorkspaceHierarchyScope(bugs, "quality_bugs", workspace.ID)
	if !s.applyProjectDataFilter(c, bugs, "quality_bugs", "project_set_id") {
		return nil, nil, scope, nil
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := escapedLikePattern(q)
		tasks = tasks.Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(task_key) LIKE ? ESCAPE '\\'", like, like)
		bugs = bugs.Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(bug_key) LIKE ? ESCAPE '\\'", like, like)
	}
	return tasks, bugs, scope, nil
}

func (s *Server) workbenchTodoSummary(c *gin.Context) {
	tasksQuery, bugsQuery, scope, err := s.workbenchTodoQueries(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	wantTasks, wantBugs, wantWorkflows := workbenchTodoDomains(c.Query("domains"))
	var tasks []taskgorm.WorkTask
	var bugs []qualitygorm.QualityBug
	if (wantTasks && tasksQuery.Find(&tasks).Error != nil) ||
		(wantBugs && bugsQuery.Find(&bugs).Error != nil) {
		dbFail(c)
		return
	}
	var workflows []workflowTodoItem
	if wantWorkflows {
		workflows, err = s.workbenchWorkflowTodos(c, scope)
		if err != nil {
			dbFail(c)
			return
		}
	}
	start, end := businessDayBounds(c)
	now := time.Now().UTC()
	summary := gin.H{
		"total": len(tasks) + len(bugs) + len(workflows),
		"tasks": len(tasks), "bugs": len(bugs), "workflows": len(workflows),
		"overdue": 0, "due_today": 0, "blocked_tasks": 0, "review_bugs": 0,
		"workflow_ai_failed": 0, "mentions": 0,
	}
	for _, task := range tasks {
		if task.DueAt != nil && task.DueAt.Before(now) {
			summary["overdue"] = summary["overdue"].(int) + 1
		}
		if task.DueAt != nil && !task.DueAt.Before(start) && task.DueAt.Before(end) {
			summary["due_today"] = summary["due_today"].(int) + 1
		}
		if status := normalizeStatus(task.Status); status == "blocked" || status == "block" {
			summary["blocked_tasks"] = summary["blocked_tasks"].(int) + 1
		}
	}
	for _, bug := range bugs {
		if bug.DueAt != nil && bug.DueAt.Before(now) {
			summary["overdue"] = summary["overdue"].(int) + 1
		}
		if bug.DueAt != nil && !bug.DueAt.Before(start) && bug.DueAt.Before(end) {
			summary["due_today"] = summary["due_today"].(int) + 1
		}
		if normalizeQualityBugStatus(bug.Status) == "review" {
			summary["review_bugs"] = summary["review_bugs"].(int) + 1
		}
	}
	for _, workflow := range workflows {
		if workflow.DueAt != nil && workflow.DueAt.Before(now) {
			summary["overdue"] = summary["overdue"].(int) + 1
		}
		if workflow.DueAt != nil && !workflow.DueAt.Before(start) && workflow.DueAt.Before(end) {
			summary["due_today"] = summary["due_today"].(int) + 1
		}
		if workflow.ActionKind == "ai_retry" {
			summary["workflow_ai_failed"] = summary["workflow_ai_failed"].(int) + 1
		}
	}
	includeMentions := strings.EqualFold(strings.TrimSpace(c.Query("include_mentions")), "true") ||
		strings.TrimSpace(c.Query("include_mentions")) == "1"
	if includeMentions {
		_, total, mentionErr := s.fetchWorkbenchTodoMentions(c, scope, 0, 0)
		if mentionErr != nil {
			dbFail(c)
			return
		}
		summary["mentions"] = total
	}
	ok(c, gin.H{"summary": summary, "generated_at": now})
}

func (s *Server) workbenchTodoFeed(c *gin.Context) {
	tasksQuery, bugsQuery, scope, err := s.workbenchTodoQueries(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	feedType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("type", "task")))
	limit, offset := pagination(c, 20)
	workspace, _ := currentWorkspace(c)
	switch feedType {
	case "workflow", "workflows", "workflow_node", "workflow_nodes":
		items, workflowErr := s.workbenchWorkflowTodos(c, scope)
		if workflowErr != nil {
			dbFail(c)
			return
		}
		total := len(items)
		start := offset
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}
		rows := make([]gin.H, 0, end-start)
		for _, item := range items[start:end] {
			rows = append(rows, workflowTodoDTO(item))
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
	case "task", "tasks":
		var total int64
		if tasksQuery.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).Count(&total).Error != nil {
			dbFail(c)
			return
		}
		var tasks []taskgorm.WorkTask
		if orderWorkbenchTodoTasks(tasksQuery.Session(&gorm.Session{}), time.Now().UTC()).
			Limit(limit).Offset(offset).Find(&tasks).Error != nil {
			dbFail(c)
			return
		}
		ctx := s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withTasks(tasks),
		)
		rows := make([]gin.H, 0, len(tasks))
		for _, task := range tasks {
			rows = append(rows, ctx.enrichTask(taskDTO(task), task))
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
	case "bug", "bugs", "quality":
		var total int64
		if bugsQuery.Session(&gorm.Session{}).Count(&total).Error != nil {
			dbFail(c)
			return
		}
		var bugs []qualitygorm.QualityBug
		if orderWorkbenchTodoBugs(bugsQuery.Session(&gorm.Session{}), time.Now().UTC()).
			Limit(limit).Offset(offset).Find(&bugs).Error != nil {
			dbFail(c)
			return
		}
		ctx := s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withQualityBugs(bugs),
		)
		rows := make([]gin.H, 0, len(bugs))
		for _, bug := range bugs {
			rows = append(rows, qualityBugDTO(bug, ctx))
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
	case "mention", "mentions":
		rows, total, mentionErr := s.fetchWorkbenchTodoMentions(c, scope, limit, offset)
		if mentionErr != nil {
			dbFail(c)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
	default:
		bad(c, "type must be workflow, task, bug, or mention")
	}
}

func orderWorkbenchTodoTasks(query *gorm.DB, now time.Time) *gorm.DB {
	priorityOrder := fmt.Sprintf(`CASE
		WHEN due_at IS NOT NULL AND due_at < '%s' THEN 0
		WHEN LOWER(TRIM(status)) IN ('blocked', 'block') THEN 1
		WHEN LOWER(TRIM(priority)) = 'urgent' THEN 2
		WHEN LOWER(TRIM(priority)) = 'high' THEN 3
		ELSE 4 END ASC`, now.Format("2006-01-02 15:04:05.999999999-07:00"))
	return query.Order(priorityOrder).
		Order("CASE WHEN due_at IS NULL THEN 1 ELSE 0 END ASC").
		Order("due_at ASC, updated_at DESC, id DESC")
}

func orderWorkbenchTodoBugs(query *gorm.DB, now time.Time) *gorm.DB {
	priorityOrder := fmt.Sprintf(`CASE
		WHEN due_at IS NOT NULL AND due_at < '%s' THEN 0
		WHEN LOWER(TRIM(status)) IN ('review', 'resolved') THEN 1
		WHEN LOWER(TRIM(severity_key)) = 'p0' THEN 2
		WHEN LOWER(TRIM(severity_key)) = 'p1' THEN 3
		WHEN LOWER(TRIM(priority)) = 'urgent' THEN 4
		WHEN LOWER(TRIM(priority)) = 'high' THEN 5
		ELSE 6 END ASC`, now.Format("2006-01-02 15:04:05.999999999-07:00"))
	return query.Order(priorityOrder).
		Order("CASE WHEN due_at IS NULL THEN 1 ELSE 0 END ASC").
		Order("due_at ASC, updated_at DESC, id DESC")
}

func workbenchTodoDomains(raw string) (tasks, bugs, workflows bool) {
	if strings.TrimSpace(raw) == "" {
		return true, true, true
	}
	for _, value := range parseCSVStrings(raw) {
		switch value {
		case "task", "tasks":
			tasks = true
		case "bug", "bugs", "quality":
			bugs = true
		case "workflow", "workflows", "workflow_node", "workflow_nodes":
			workflows = true
		}
	}
	return tasks, bugs, workflows
}

func businessDayBounds(c *gin.Context) (time.Time, time.Time) {
	location := time.UTC
	if value := strings.TrimSpace(c.GetHeader("X-Timezone")); value != "" {
		if parsed, err := time.LoadLocation(value); err == nil {
			location = parsed
		}
	}
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	return start.UTC(), start.AddDate(0, 0, 1).UTC()
}
