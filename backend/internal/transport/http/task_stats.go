package httpapi

import (
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type taskStats struct {
	Total      int64
	Completed  int64
	Open       int64
	InProgress int64
	Todo       int64
}

type taskStatusCount struct {
	Status string
	Count  int64 `gorm:"column:task_count"`
}

func loadTaskStats(db *gorm.DB, where string, args ...any) taskStats {
	var rows []taskStatusCount
	pmTaskQuery(db).
		Model(&taskgorm.WorkTask{}).
		Select("status, COUNT(*) AS task_count").
		Where(where, args...).
		Group("status").
		Scan(&rows)
	return summarizeTaskStatusCounts(rows)
}

// loadTaskStatsGrouped aggregates PM task status counts per owner column in a
// single GROUP BY so list endpoints avoid one query per row. ownerIDs must be
// pre-scoped to the workspace; the workspace filter keeps the query inside the
// tenancy boundary regardless.
func loadTaskStatsGrouped(db *gorm.DB, ownerColumn string, workspaceID uint, ownerIDs []uint) map[uint]taskStats {
	result := make(map[uint]taskStats, len(ownerIDs))
	if len(ownerIDs) == 0 {
		return result
	}
	var rows []struct {
		OwnerID uint
		Status  string
		Count   int64 `gorm:"column:task_count"`
	}
	pmTaskQuery(db).
		Model(&taskgorm.WorkTask{}).
		Select(ownerColumn+" AS owner_id, status, COUNT(*) AS task_count").
		Where("workspace_id = ?", workspaceID).
		Where(ownerColumn+" IN ?", ownerIDs).
		Group(ownerColumn + ", status").
		Scan(&rows)
	countsByOwner := make(map[uint][]taskStatusCount, len(ownerIDs))
	for _, row := range rows {
		countsByOwner[row.OwnerID] = append(countsByOwner[row.OwnerID], taskStatusCount{Status: row.Status, Count: row.Count})
	}
	for ownerID, counts := range countsByOwner {
		result[ownerID] = summarizeTaskStatusCounts(counts)
	}
	return result
}

func loadTaskStatsByRequirement(db *gorm.DB, workspaceID uint, requirementIDs []uint) map[uint]taskStats {
	return loadTaskStatsGrouped(db, "requirement_id", workspaceID, requirementIDs)
}

// loadTaskStatsByPlanUnit keys work_tasks.project_id, which stores the owning
// plan unit id.
func loadTaskStatsByPlanUnit(db *gorm.DB, workspaceID uint, planUnitIDs []uint) map[uint]taskStats {
	return loadTaskStatsGrouped(db, "project_id", workspaceID, planUnitIDs)
}

func loadTaskStatsByProjectSet(db *gorm.DB, workspaceID uint, projectSetIDs []uint) map[uint]taskStats {
	return loadTaskStatsGrouped(db, "project_set_id", workspaceID, projectSetIDs)
}

func summarizeTaskStatusCounts(rows []taskStatusCount) taskStats {
	completed := taskStatusSet(completedTaskStatuses())
	terminal := taskStatusSet(terminalTaskStatuses())
	inProgress := taskStatusSet(inProgressTaskStatuses())
	todo := taskStatusSet(todoTaskStatuses())
	var stats taskStats
	for _, row := range rows {
		stats.Total += row.Count
		if completed[row.Status] {
			stats.Completed += row.Count
		}
		if !terminal[row.Status] {
			stats.Open += row.Count
		}
		if inProgress[row.Status] {
			stats.InProgress += row.Count
		}
		if todo[row.Status] {
			stats.Todo += row.Count
		}
	}
	return stats
}

func taskStatusSet(statuses []string) map[string]bool {
	result := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		result[status] = true
	}
	return result
}
