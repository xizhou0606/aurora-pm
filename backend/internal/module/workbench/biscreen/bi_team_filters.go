package biscreen

import (
	"fmt"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"sort"
	"strconv"
	"time"
)

func biTeamTimeKey(value time.Time, dimension string, location *time.Location) string {
	if location == nil {
		location = time.UTC
	}
	localized := value.In(location)
	if dimension == biTeamDimensionWeek {
		year, week := localized.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	}
	return localized.Format("2006-01")
}

func biValidTeamWeekKey(key string) bool {
	var year, week int
	if _, err := fmt.Sscanf(key, "%04d-W%02d", &year, &week); err != nil || year < 1 || week < 1 || week > 53 {
		return false
	}
	return fmt.Sprintf("%04d-W%02d", year, week) == key && biTeamWeekStart(year, week) != nil
}

func biTeamWeekStart(year, week int) *time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := (int(jan4.Weekday()) + 6) % 7
	start := jan4.AddDate(0, 0, -weekday+7*(week-1))
	isoYear, isoWeek := start.ISOWeek()
	if isoYear != year || isoWeek != week {
		return nil
	}
	return &start
}

func biTeamTimeLabel(key, dimension string) string {
	if dimension == biTeamDimensionWeek {
		var year, week int
		_, _ = fmt.Sscanf(key, "%04d-W%02d", &year, &week)
		return fmt.Sprintf("%d 年第 %02d 周", year, week)
	}
	parsed, _ := time.Parse("2006-01", key)
	return parsed.Format("2006 年 01 月")
}

func biTeamDimensionOptions(
	dimension string,
	scope workspaceScope,
	ctx biWorkspaceContext,
	requirements []requirement.Snapshot,
	tasks []task.Snapshot,
	bugs []quality.BugSnapshot,
	location *time.Location,
) []biTeamDimensionOption {
	if dimension == biTeamDimensionMonth || dimension == biTeamDimensionWeek {
		if location == nil {
			location = time.UTC
		}
		periods := map[string]struct{}{}
		add := func(value time.Time) {
			if !value.IsZero() {
				periods[biTeamTimeKey(value, dimension, location)] = struct{}{}
			}
		}
		for _, row := range requirements {
			add(row.CreatedAt)
		}
		for _, row := range tasks {
			add(row.CreatedAt)
		}
		for _, row := range bugs {
			add(row.CreatedAt)
		}
		keys := make([]string, 0, len(periods))
		for key := range periods {
			keys = append(keys, key)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		rows := make([]biTeamDimensionOption, 0, len(keys))
		for _, key := range keys {
			rows = append(rows, biTeamDimensionOption{Key: key, Label: biTeamTimeLabel(key, dimension)})
		}
		return rows
	}

	visibleSets := uintSet(scope.VisibleSets)
	selectedSets := uintSet(scope.ProjectSetIDs)
	selectedProjects := uintSet(scope.ProjectIDs)
	selectedRequirementProjects := map[uint]struct{}{}
	if len(scope.RequirementIDs) > 0 {
		for _, requirement := range requirements {
			if requirement.PlanUnitID != nil {
				selectedRequirementProjects[*requirement.PlanUnitID] = struct{}{}
			}
		}
	}
	rows := make([]biTeamDimensionOption, 0, len(ctx.Projects))
	for id, project := range ctx.Projects {
		if _, ok := visibleSets[project.ProjectSetID]; !ok {
			continue
		}
		if len(selectedSets) > 0 {
			if _, ok := selectedSets[project.ProjectSetID]; !ok {
				continue
			}
		}
		if len(selectedProjects) > 0 {
			if _, ok := selectedProjects[id]; !ok {
				continue
			}
		}
		if len(scope.RequirementIDs) > 0 {
			if _, ok := selectedRequirementProjects[id]; !ok {
				continue
			}
		}
		rows = append(rows, biTeamDimensionOption{Key: strconv.FormatUint(uint64(id), 10), Label: project.Title})
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := strconv.ParseUint(rows[i].Key, 10, 64)
		right, _ := strconv.ParseUint(rows[j].Key, 10, 64)
		return left > right
	})
	return rows
}

func uintSet(values []uint) map[uint]struct{} {
	result := make(map[uint]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func biTeamRecordMatches(projectID uint, createdAt time.Time, dimension, key string, location *time.Location) bool {
	if key == "" {
		return true
	}
	if dimension == biTeamDimensionVersion {
		return strconv.FormatUint(uint64(projectID), 10) == key
	}
	if createdAt.IsZero() {
		return false
	}
	if location == nil {
		location = time.UTC
	}
	return biTeamTimeKey(createdAt, dimension, location) == key
}

func biFilterTeamRequirements(
	rows []requirement.Snapshot, dimension, key string, location *time.Location,
) []requirement.Snapshot {
	result := make([]requirement.Snapshot, 0, len(rows))
	for _, row := range rows {
		planUnitID := uint(0)
		if row.PlanUnitID != nil {
			planUnitID = *row.PlanUnitID
		}
		if biTeamRecordMatches(planUnitID, row.CreatedAt, dimension, key, location) {
			result = append(result, row)
		}
	}
	return result
}

func biFilterTeamTasks(rows []task.Snapshot, dimension, key string, location *time.Location) []task.Snapshot {
	result := make([]task.Snapshot, 0, len(rows))
	for _, row := range rows {
		if biTeamRecordMatches(row.PlanUnitID, row.CreatedAt, dimension, key, location) {
			result = append(result, row)
		}
	}
	return result
}

func biFilterTeamBugs(rows []quality.BugSnapshot, dimension, key string, location *time.Location) []quality.BugSnapshot {
	result := make([]quality.BugSnapshot, 0, len(rows))
	for _, row := range rows {
		if biTeamRecordMatches(row.PlanUnitID, row.CreatedAt, dimension, key, location) {
			result = append(result, row)
		}
	}
	return result
}
