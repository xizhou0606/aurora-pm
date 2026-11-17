package biscreen

import (
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"strings"
	"time"
)

type biAggregation struct {
	Tasks    []task.Snapshot
	Bugs     []quality.BugSnapshot
	Ctx      biWorkspaceContext
	Now      time.Time
	Location *time.Location
	Days     int
}

func biUserFactIDs(tasks []task.Snapshot, bugs []quality.BugSnapshot) []uint {
	ids := make([]uint, 0, len(tasks)*2+len(bugs)*3)
	appendID := func(id *uint) {
		if id != nil && *id > 0 {
			ids = append(ids, *id)
		}
	}
	for _, task := range tasks {
		appendID(task.AssigneeID)
		appendID(task.ReporterUserID)
	}
	for _, bug := range bugs {
		appendID(bug.AssigneeUserID)
		appendID(bug.ReporterUserID)
		appendID(bug.ReviewerUserID)
	}
	return mergeSortedUserIDs(ids)
}

func (a biAggregation) businessDay(offset int) (time.Time, time.Time, time.Time) {
	location := a.Location
	if location == nil {
		location = time.UTC
	}
	localNow := a.Now.In(location)
	localStart := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location,
	).AddDate(0, 0, -offset)
	return localStart, localStart.UTC(), localStart.AddDate(0, 0, 1).UTC()
}

func biScopeDTO(scope workspaceScope, ctx biWorkspaceContext, period string) map[string]any {
	labels := make([]string, 0)
	for _, id := range scope.ProjectSetIDs {
		if row, ok := ctx.ProjectSets[id]; ok {
			labels = append(labels, row.Name)
		}
	}
	for _, id := range scope.ProjectIDs {
		if row, ok := ctx.Projects[id]; ok {
			labels = append(labels, row.Title)
		}
	}
	for _, id := range scope.RequirementIDs {
		if row, ok := ctx.Requirements[id]; ok {
			labels = append(labels, row.Title)
		}
	}
	mode, label := "global", "全部项目"
	if len(labels) > 0 {
		mode, label = "project_scope", strings.Join(labels, " / ")
	}
	return map[string]any{
		"mode": mode, "label": label, "global": mode == "global", "global_limited": false,
		"project_set_ids": scope.ProjectSetIDs, "project_ids": scope.ProjectIDs,
		"requirement_ids": scope.RequirementIDs, "period": period,
	}
}

func isTaskDone(status string) bool {
	status = normalizeStatus(status)
	for _, value := range completedTaskStatuses() {
		if status == value {
			return true
		}
	}
	return false
}

func isTaskTerminal(status string) bool {
	return task.IsTerminalStatus(status)
}

func isBugOpen(status string) bool {
	switch normalizeStatus(status) {
	case "done", "closed", "completed", "canceled", "cancelled":
		return false
	default:
		return true
	}
}

func biScreenPercent(part, total int64) int {
	if total <= 0 || part <= 0 {
		return 0
	}
	percent := int((part*100 + total/2) / total)
	if percent > 100 {
		return 100
	}
	return percent
}

func (a biAggregation) overviewMetrics() map[string]any {
	metrics := map[string]any{
		"total_tasks": len(a.Tasks), "in_flight_tasks": 0, "overdue_tasks": 0,
		"due_soon_tasks": 0, "unassigned_tasks": 0, "current_scope_count": len(a.Tasks),
		"open_bugs": 0, "blocker_bugs": 0, "high_severity_bugs": 0,
		"pending_review_bugs": 0, "overdue_open_bugs": 0, "blocked_tasks": 0,
		"done_tasks":          0,
		"completion_rate_pct": 0, "milestone_progress_pct": 0,
	}
	for _, task := range a.Tasks {
		done := isTaskDone(task.Status)
		terminal := isTaskTerminal(task.Status)
		if done {
			metrics["done_tasks"] = metrics["done_tasks"].(int) + 1
		} else if !terminal {
			metrics["in_flight_tasks"] = metrics["in_flight_tasks"].(int) + 1
		}
		if !terminal && task.AssigneeID == nil {
			metrics["unassigned_tasks"] = metrics["unassigned_tasks"].(int) + 1
		}
		if !terminal && task.DueAt != nil && task.DueAt.Before(a.Now) {
			metrics["overdue_tasks"] = metrics["overdue_tasks"].(int) + 1
		}
		if !terminal && task.DueAt != nil && task.DueAt.After(a.Now) && task.DueAt.Before(a.Now.AddDate(0, 0, 7)) {
			metrics["due_soon_tasks"] = metrics["due_soon_tasks"].(int) + 1
		}
		if normalizeStatus(task.Status) == "blocked" {
			metrics["blocked_tasks"] = metrics["blocked_tasks"].(int) + 1
		}
	}
	for _, bug := range a.Bugs {
		if !isBugOpen(bug.Status) {
			continue
		}
		metrics["open_bugs"] = metrics["open_bugs"].(int) + 1
		if bug.DueAt != nil && bug.DueAt.Before(a.Now) {
			metrics["overdue_open_bugs"] = metrics["overdue_open_bugs"].(int) + 1
		}
		highSeverity := isHighSeverity(bug.SeverityKey)
		if highSeverity || isBIQualityHighPriority(bug.Priority) {
			metrics["blocker_bugs"] = metrics["blocker_bugs"].(int) + 1
		}
		if highSeverity {
			metrics["high_severity_bugs"] = metrics["high_severity_bugs"].(int) + 1
		}
		if isBIQualityReview(bug.Status) {
			metrics["pending_review_bugs"] = metrics["pending_review_bugs"].(int) + 1
		}
	}
	if len(a.Tasks) > 0 {
		rate := biScreenPercent(int64(metrics["done_tasks"].(int)), int64(len(a.Tasks)))
		metrics["completion_rate_pct"], metrics["milestone_progress_pct"] = rate, rate
	}
	return metrics
}
