package biscreen

import (
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func biMetricInt(metrics map[string]any, key string) int {
	switch value := metrics[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func biMetricFloat(metrics map[string]any, key string) float64 {
	switch value := metrics[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func biCompatibilityFields(metrics map[string]any, tasks []task.Snapshot, projectTotal int) map[string]any {
	due, assignee := 0, 0
	for _, task := range tasks {
		if task.DueAt != nil {
			due++
		}
		if task.AssigneeID != nil {
			assignee++
		}
	}
	fill := func(value int) int {
		if len(tasks) == 0 {
			return 0
		}
		return value * 100 / len(tasks)
	}
	completion := int(biMetricFloat(metrics, "completion_rate_pct"))
	overdue := biMetricInt(metrics, "overdue_tasks")
	highBugs := biMetricInt(metrics, "high_severity_bugs")
	return map[string]any{
		"summary": map[string]any{
			"company_avg_dhi": completion, "health_light": biHealthLight(overdue, highBugs),
			"on_time_rate_pct": 100 - minInt(100, overdue), "deploy_link_pct": 0,
			"active_projects": projectTotal, "exception_projects": 0,
			"release_pending": 0, "risk_high": highBugs, "risk_medium": overdue, "risk_low": 0,
		},
		"kpi": map[string]any{
			"tasks_in_flight":  biMetricInt(metrics, "in_flight_tasks"),
			"completed_period": biMetricInt(metrics, "done_tasks"),
			"on_time_rate_pct": 100 - minInt(100, overdue), "overdue_open": overdue,
			"p0_open": biMetricInt(metrics, "blocker_bugs"), "releasable_count": 0,
			"tasks_due_soon": biMetricInt(metrics, "due_soon_tasks"), "task_deploy_link_pct": 0,
		},
		"teams": []any{}, "projects": []any{}, "risks": []any{},
		"risks_by_team": []any{}, "risks_by_kind": []any{}, "release_train": []any{},
		"sparklines": []any{}, "light_distribution": map[string]any{"green": 0, "yellow": 0, "red": 0, "gray": projectTotal},
		"status_mix": []any{}, "data_quality": map[string]any{
			"due_at_fill_pct": fill(due), "assignee_fill_pct": fill(assignee),
		},
		"project_total": projectTotal, "project_exception_count": 0,
		"pm_pipeline":   map[string]any{"steady": 0, "watch": 0, "critical": 0, "idle": projectTotal},
		"team_clusters": []any{},
		"pm_narrative":  map[string]any{"headline": "PM 独立工作台", "subline": "数据仅来自当前工作区可见范围"},
	}
}

func biHealthLight(overdue, highBugs int) string {
	if highBugs > 0 {
		return "red"
	}
	if overdue > 0 {
		return "yellow"
	}
	return "green"
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
