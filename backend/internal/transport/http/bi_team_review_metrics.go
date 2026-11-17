package httpapi

import (
	"sort"
	"time"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func biTeamEventMonthOptions(
	existing []biTeamDimensionOption,
	tasks []task.Snapshot,
	bugs []quality.BugSnapshot,
	bugActivity []quality.BugActivitySnapshot,
	decisions []biTeamReviewDecision,
	location *time.Location,
) []biTeamDimensionOption {
	if location == nil {
		location = time.UTC
	}
	months := make(map[string]struct{}, len(existing))
	add := func(value time.Time) {
		if !value.IsZero() {
			months[value.In(location).Format("2006-01")] = struct{}{}
		}
	}
	for _, option := range existing {
		months[option.Key] = struct{}{}
	}
	for _, task := range tasks {
		if task.DueAt != nil {
			add(*task.DueAt)
		}
	}
	for _, bug := range bugs {
		for _, value := range []*time.Time{
			bug.ResolvedAt, bug.CompletedAt, bug.CanceledAt,
		} {
			if value != nil {
				add(*value)
			}
		}
	}
	for _, activity := range bugActivity {
		field := normalizeStatus(activity.FieldKey)
		status := normalizeQualityBugStatus(activity.NewValue)
		if field == "resolution_reason" || field == "assignee_user_id" || field == "status" &&
			(status == "in_progress" || status == "review" || status == "resolved") {
			add(activity.CreatedAt)
		}
	}
	for _, decision := range decisions {
		add(decision.DecidedAt)
	}
	keys := make([]string, 0, len(months))
	for key := range months {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	options := make([]biTeamDimensionOption, 0, len(keys))
	for _, key := range keys {
		parsed, _ := time.Parse("2006-01", key)
		options = append(options, biTeamDimensionOption{Key: key, Label: parsed.Format("2006 年 01 月")})
	}
	return options
}
