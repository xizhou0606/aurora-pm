package taskgorm

import (
	"context"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

func (repository *ReadRepository) OverviewStats(ctx context.Context, workspaceID uint, filter task.ListFilter) (task.OverviewStats, error) {
	query := repository.filteredTasks(ctx, workspaceID, filter)
	terminal := terminalStatuses()
	now := filter.AsOf.UTC()
	until := now.AddDate(0, 0, 7)
	result := task.OverviewStats{DueSoonDays: 7, ByKind: map[string]int64{}, ByKindCompleted: map[string]int64{}, ByStatus: map[string]int64{}}
	if err := countTasks(query, &result.Total); err != nil {
		return task.OverviewStats{}, err
	}
	if err := countWhere(query, &result.Completed, "LOWER(TRIM(status)) IN ?", terminal); err != nil {
		return task.OverviewStats{}, err
	}
	result.Incomplete = result.Total - result.Completed
	if err := countWhere(query, &result.Overdue, "LOWER(TRIM(status)) NOT IN ? AND due_at IS NOT NULL AND due_at < ?", terminal, now); err != nil {
		return task.OverviewStats{}, err
	}
	if err := countWhere(query, &result.DueSoon, "LOWER(TRIM(status)) NOT IN ? AND due_at IS NOT NULL AND due_at >= ? AND due_at <= ?", terminal, now, until); err != nil {
		return task.OverviewStats{}, err
	}
	var err error
	if result.ByStatus, err = groupedCounts(query, "LOWER(TRIM(status))"); err != nil {
		return task.OverviewStats{}, err
	}
	if result.ByKind, err = groupedCounts(query, "LOWER(TRIM(kind))"); err != nil {
		return task.OverviewStats{}, err
	}
	if result.ByKindCompleted, err = groupedCounts(query.Where("LOWER(TRIM(status)) IN ?", terminal), "LOWER(TRIM(kind))"); err != nil {
		return task.OverviewStats{}, err
	}
	return result, nil
}

func (repository *ReadRepository) DeliveryStats(ctx context.Context, workspaceID uint, filter task.ListFilter) (task.DeliveryStats, error) {
	query := repository.filteredTasks(ctx, workspaceID, filter)
	terminal := terminalStatuses()
	result := task.DeliveryStats{ByStatus: map[string]int64{}, ByPriority: map[string]int64{}}
	if err := countTasks(query, &result.Total); err != nil {
		return task.DeliveryStats{}, err
	}
	if err := countWhere(query, &result.Completed, "LOWER(TRIM(status)) IN ?", terminal); err != nil {
		return task.DeliveryStats{}, err
	}
	result.Open = result.Total - result.Completed
	if err := countWhere(query, &result.Overdue, "LOWER(TRIM(status)) NOT IN ? AND due_at IS NOT NULL AND due_at < ?", terminal, filter.AsOf.UTC()); err != nil {
		return task.DeliveryStats{}, err
	}
	var err error
	if result.ByStatus, err = groupedCounts(query, "LOWER(TRIM(status))"); err != nil {
		return task.DeliveryStats{}, err
	}
	if result.ByPriority, err = groupedCounts(query, "LOWER(TRIM(priority))"); err != nil {
		return task.DeliveryStats{}, err
	}
	if result.Total > 0 {
		result.CompletionRate = float64(result.Completed) * 100 / float64(result.Total)
	}
	return result, nil
}

func (repository *ReadRepository) DueCompletionStats(ctx context.Context, workspaceID uint, filter task.ListFilter) (task.DueCompletionStats, error) {
	query := repository.filteredTasks(ctx, workspaceID, filter)
	now := filter.AsOf.UTC()
	dayStart := dayStartUTC(now)
	nextDay := dayStart.AddDate(0, 0, 1)
	dueSoonEnd := nextDay.AddDate(0, 0, 7)
	terminal := terminalStatuses()
	result := task.DueCompletionStats{AsOf: now, Timezone: "UTC"}
	checks := []struct {
		destination *int64
		condition   string
		args        []any
	}{
		{&result.Overdue, "LOWER(TRIM(status)) NOT IN ? AND due_at IS NOT NULL AND due_at < ?", []any{terminal, dayStart}},
		{&result.DueToday, "LOWER(TRIM(status)) NOT IN ? AND due_at >= ? AND due_at < ?", []any{terminal, dayStart, nextDay}},
		{&result.DueSoon, "LOWER(TRIM(status)) NOT IN ? AND due_at >= ? AND due_at < ?", []any{terminal, nextDay, dueSoonEnd}},
		{&result.Later, "LOWER(TRIM(status)) NOT IN ? AND due_at >= ?", []any{terminal, dueSoonEnd}},
		{&result.OpenWithoutDue, "LOWER(TRIM(status)) NOT IN ? AND due_at IS NULL", []any{terminal}},
		{&result.CompletedOnTime, "LOWER(TRIM(status)) IN ? AND due_at IS NOT NULL AND completed_at IS NOT NULL AND completed_at <= due_at", []any{terminal}},
		{&result.CompletedLate, "LOWER(TRIM(status)) IN ? AND due_at IS NOT NULL AND completed_at IS NOT NULL AND completed_at > due_at", []any{terminal}},
		{&result.CompletedWithoutDue, "LOWER(TRIM(status)) IN ? AND due_at IS NULL", []any{terminal}},
	}
	for _, check := range checks {
		if err := countWhere(query, check.destination, check.condition, check.args...); err != nil {
			return task.DueCompletionStats{}, err
		}
	}
	if err := countTasks(query, &result.Total); err != nil {
		return task.DueCompletionStats{}, err
	}
	return result, nil
}

func (repository *ReadRepository) AssigneeStatusStats(ctx context.Context, workspaceID uint, filter task.ListFilter) (task.AssigneeStatusStats, error) {
	query := repository.filteredTasks(ctx, workspaceID, filter)
	type countRow struct {
		AssigneeID *uint
		Status     string
		Count      int64
	}
	var rows []countRow
	if err := query.Session(&gorm.Session{}).Model(&WorkTask{}).
		Select("assignee_id, LOWER(TRIM(status)) AS status, COUNT(*) AS count").
		Group("assignee_id, LOWER(TRIM(status))").Order("assignee_id ASC, status ASC").Scan(&rows).Error; err != nil {
		return task.AssigneeStatusStats{}, err
	}
	result := task.AssigneeStatusStats{Items: []task.AssigneeStatusItem{}}
	index := map[uint]int{}
	unassigned := -1
	for _, row := range rows {
		position := -1
		if row.AssigneeID == nil || *row.AssigneeID == 0 {
			position = unassigned
			if position < 0 {
				result.Items = append(result.Items, task.AssigneeStatusItem{ByStatus: map[string]int64{}})
				position = len(result.Items) - 1
				unassigned = position
			}
		} else if existing, exists := index[*row.AssigneeID]; exists {
			position = existing
		} else {
			id := *row.AssigneeID
			result.Items = append(result.Items, task.AssigneeStatusItem{AssigneeID: &id, ByStatus: map[string]int64{}})
			position = len(result.Items) - 1
			index[id] = position
		}
		result.Items[position].ByStatus[row.Status] = row.Count
		result.Items[position].Total += row.Count
	}
	result.TotalAssignees = len(result.Items)
	return result, nil
}

func countTasks(query *gorm.DB, destination *int64) error {
	return query.Session(&gorm.Session{}).Model(&WorkTask{}).Count(destination).Error
}

func countWhere(query *gorm.DB, destination *int64, condition string, args ...any) error {
	return query.Session(&gorm.Session{}).Model(&WorkTask{}).Where(condition, args...).Count(destination).Error
}

func groupedCounts(query *gorm.DB, expression string) (map[string]int64, error) {
	type countRow struct {
		Key   string
		Count int64
	}
	var rows []countRow
	if err := query.Session(&gorm.Session{}).Model(&WorkTask{}).
		Select(expression + " AS key, COUNT(*) AS count").Group(expression).Order("key ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		key := strings.TrimSpace(row.Key)
		if key == "" {
			key = "unknown"
		}
		result[key] = row.Count
	}
	return result, nil
}
