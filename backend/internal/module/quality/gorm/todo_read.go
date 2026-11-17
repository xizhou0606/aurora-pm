package qualitygorm

import (
	"context"
	"sort"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

func (repository *BugReadRepository) ListPrincipalBugTodos(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	filter quality.BugTodoFilter,
) (quality.BugReadPage, error) {
	reviewerStatuses := []string{"review", "resolved", "done"}
	terminalStatuses := []string{"done", "closed", "completed", "canceled", "cancelled", "duplicate"}
	query := repository.db.WithContext(ctx).Model(&QualityBug{}).
		Where("workspace_id = ? AND LOWER(TRIM(status)) NOT IN ?", workspaceID, terminalStatuses).
		Where(
			"((LOWER(TRIM(status)) IN ? AND COALESCE(reviewer_user_id, reporter_user_id) = ?) OR "+
				"((LOWER(TRIM(status)) NOT IN ? OR status IS NULL) AND assignee_user_id = ?))",
			reviewerStatuses, principalID, reviewerStatuses, principalID,
		)
	if filter.ProjectSetID > 0 {
		query = query.Where("project_set_id = ?", filter.ProjectSetID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("project_id = ?", filter.PlanUnitID)
	}
	if filter.RequirementID > 0 {
		query = query.Where("requirement_id = ?", filter.RequirementID)
	}
	if keyword := strings.ToLower(strings.TrimSpace(filter.Query)); keyword != "" {
		pattern := "%" + escapeBugReadLike(keyword) + "%"
		query = query.Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(bug_key) LIKE ? ESCAPE '\\'", pattern, pattern)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return quality.BugReadPage{}, err
	}
	var rows []QualityBug
	if err := query.Find(&rows).Error; err != nil {
		return quality.BugReadPage{}, err
	}
	sort.SliceStable(rows, func(i, j int) bool { return todoBugLess(rows[i], rows[j], filter) })
	start := filter.Offset
	if start > len(rows) {
		start = len(rows)
	}
	end := start + filter.Limit
	if end > len(rows) {
		end = len(rows)
	}
	items := make([]quality.BugSnapshot, 0, end-start)
	for _, row := range rows[start:end] {
		items = append(items, bugReadSnapshot(row))
	}
	return quality.BugReadPage{Items: items, Total: total}, nil
}

func todoBugLess(left, right QualityBug, filter quality.BugTodoFilter) bool {
	leftPriority, rightPriority := todoBugPriority(left, filter), todoBugPriority(right, filter)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if (left.DueAt == nil) != (right.DueAt == nil) {
		return left.DueAt != nil
	}
	if left.DueAt != nil && !left.DueAt.Equal(*right.DueAt) {
		return left.DueAt.Before(*right.DueAt)
	}
	if !left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.UpdatedAt.After(right.UpdatedAt)
	}
	return left.ID > right.ID
}

func todoBugPriority(row QualityBug, filter quality.BugTodoFilter) int {
	if row.DueAt != nil && row.DueAt.Before(filter.Now) {
		return 0
	}
	if status := strings.ToLower(strings.TrimSpace(row.Status)); status == "review" || status == "resolved" {
		return 1
	}
	switch strings.ToLower(strings.TrimSpace(row.SeverityKey)) {
	case "p0":
		return 2
	case "p1":
		return 3
	}
	switch strings.ToLower(strings.TrimSpace(row.Priority)) {
	case "urgent":
		return 4
	case "high":
		return 5
	default:
		return 6
	}
}

var _ quality.BugTodoStore = (*BugReadRepository)(nil)
