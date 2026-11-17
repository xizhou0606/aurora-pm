package httpapi

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type nativeTaskStatsCount struct {
	Key   string `gorm:"column:key"`
	Count int64  `gorm:"column:count"`
}

func (s *Server) nativeTaskStatsQuery(c *gin.Context) (*gorm.DB, identitygorm.Workspace, bool) {
	workspace, _ := currentWorkspace(c)
	scope, err := s.parseWorkspaceScope(c)
	if err != nil {
		bad(c, err.Error())
		return nil, identitygorm.Workspace{}, false
	}
	query := applyWorkspaceScope(
		pmTaskQuery(s.db).Where("workspace_id = ?", workspace.ID), scope,
	)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspace.ID)
	query = applyWorkspaceTaskFilters(c, query)
	query, err = applyWorkspaceTaskContractFilters(c, query)
	if err != nil {
		bad(c, err.Error())
		return nil, identitygorm.Workspace{}, false
	}
	if !s.applyProjectDataFilter(c, query, "work_tasks", "project_set_id") {
		return nil, identitygorm.Workspace{}, false
	}
	return query, workspace, true
}

func (s *Server) nativeTaskDeliveryStats(c *gin.Context) {
	query, _, valid := s.nativeTaskStatsQuery(c)
	if !valid {
		return
	}
	var total, completed, overdue int64
	now := time.Now().UTC()
	terminal := terminalTaskStatuses()
	if query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).Count(&total).Error != nil ||
		query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
			Where("LOWER(TRIM(status)) IN ?", terminal).Count(&completed).Error != nil ||
		query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
			Where("LOWER(TRIM(status)) NOT IN ?", terminal).
			Where("due_at IS NOT NULL AND due_at < ?", now).Count(&overdue).Error != nil {
		dbFail(c)
		return
	}
	byStatus, err := nativeTaskGroupedCounts(query, "LOWER(TRIM(status))")
	if err != nil {
		dbFail(c)
		return
	}
	byPriority, err := nativeTaskGroupedCounts(query, "LOWER(TRIM(priority))")
	if err != nil {
		dbFail(c)
		return
	}
	completionRate := float64(0)
	if total > 0 {
		completionRate = float64(completed) * 100 / float64(total)
	}
	ok(c, gin.H{
		"total": total, "completed": completed, "open": total - completed,
		"overdue": overdue, "completion_rate": completionRate,
		"by_status": byStatus, "by_priority": byPriority,
	})
}

func (s *Server) nativeTaskDueCompletionStats(c *gin.Context) {
	query, _, valid := s.nativeTaskStatsQuery(c)
	if !valid {
		return
	}
	now := time.Now().UTC()
	dayStart := taskBusinessDayStart(c, now)
	nextDay := dayStart.AddDate(0, 0, 1)
	dueSoonEnd := nextDay.AddDate(0, 0, 7)
	terminal := terminalTaskStatuses()
	counts := gin.H{}
	checks := []struct {
		name      string
		condition string
		args      []any
	}{
		{name: "overdue", condition: "LOWER(TRIM(status)) NOT IN ? AND due_at IS NOT NULL AND due_at < ?", args: []any{terminal, dayStart}},
		{name: "due_today", condition: "LOWER(TRIM(status)) NOT IN ? AND due_at >= ? AND due_at < ?", args: []any{terminal, dayStart, nextDay}},
		{name: "due_soon", condition: "LOWER(TRIM(status)) NOT IN ? AND due_at >= ? AND due_at < ?", args: []any{terminal, nextDay, dueSoonEnd}},
		{name: "later", condition: "LOWER(TRIM(status)) NOT IN ? AND due_at >= ?", args: []any{terminal, dueSoonEnd}},
		{name: "open_without_due", condition: "LOWER(TRIM(status)) NOT IN ? AND due_at IS NULL", args: []any{terminal}},
		{name: "completed_on_time", condition: "LOWER(TRIM(status)) IN ? AND due_at IS NOT NULL AND completed_at IS NOT NULL AND completed_at <= due_at", args: []any{terminal}},
		{name: "completed_late", condition: "LOWER(TRIM(status)) IN ? AND due_at IS NOT NULL AND completed_at IS NOT NULL AND completed_at > due_at", args: []any{terminal}},
		{name: "completed_without_due", condition: "LOWER(TRIM(status)) IN ? AND due_at IS NULL", args: []any{terminal}},
	}
	for _, check := range checks {
		var count int64
		if query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
			Where(check.condition, check.args...).Count(&count).Error != nil {
			dbFail(c)
			return
		}
		counts[check.name] = count
	}
	var total int64
	if query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).Count(&total).Error != nil {
		dbFail(c)
		return
	}
	counts["total"] = total
	counts["as_of"] = now
	counts["timezone"] = biLocation(c).String()
	ok(c, counts)
}

func (s *Server) nativeTaskAssigneeStatusStats(c *gin.Context) {
	query, workspace, valid := s.nativeTaskStatsQuery(c)
	if !valid {
		return
	}
	type countRow struct {
		AssigneeID *uint
		Status     string
		Count      int64
	}
	var rows []countRow
	if query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
		Select("assignee_id, LOWER(TRIM(status)) AS status, COUNT(*) AS count").
		Group("assignee_id, LOWER(TRIM(status))").
		Order("assignee_id ASC, status ASC").Scan(&rows).Error != nil {
		dbFail(c)
		return
	}
	type aggregate struct {
		AssigneeID *uint
		ByStatus   gin.H
		Total      int64
	}
	aggregates := make([]*aggregate, 0)
	byID := map[uint]*aggregate{}
	var unassigned *aggregate
	for _, row := range rows {
		var target *aggregate
		if row.AssigneeID == nil || *row.AssigneeID == 0 {
			if unassigned == nil {
				unassigned = &aggregate{ByStatus: gin.H{}}
				aggregates = append(aggregates, unassigned)
			}
			target = unassigned
		} else {
			target = byID[*row.AssigneeID]
			if target == nil {
				id := *row.AssigneeID
				target = &aggregate{AssigneeID: &id, ByStatus: gin.H{}}
				byID[id] = target
				aggregates = append(aggregates, target)
			}
		}
		target.ByStatus[row.Status] = row.Count
		target.Total += row.Count
	}
	refs := workspaceContextReferences{}
	for _, row := range aggregates {
		refs.UserIDs = appendOptionalID(refs.UserIDs, row.AssigneeID)
	}
	context := s.loadWorkspaceContext(workspace.ID, refs)
	result := make([]gin.H, 0, len(aggregates))
	for _, row := range aggregates {
		result = append(result, gin.H{
			"assignee_id": row.AssigneeID, "assignee_name": userDisplayName(context, row.AssigneeID),
			"by_status": row.ByStatus, "total": row.Total,
		})
	}
	ok(c, gin.H{"items": result, "total_assignees": len(result)})
}

func nativeTaskGroupedCounts(query *gorm.DB, expression string) (gin.H, error) {
	var rows []nativeTaskStatsCount
	if err := query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).
		Select(expression + " AS key, COUNT(*) AS count").
		Group(expression).Order("key ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := gin.H{}
	for _, row := range rows {
		key := strings.TrimSpace(row.Key)
		if key == "" {
			key = "unknown"
		}
		result[key] = row.Count
	}
	return result, nil
}
