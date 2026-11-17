package biscreen

import (
	"sort"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

var biLabels = map[string]string{
	"draft": "草稿", "backlog": "待规划", "todo": "待处理", "in_dev": "开发中",
	"in_progress": "处理中", "reopened": "重新打开", "pending_test": "待测试", "blocked": "已阻塞", "review": "回归验证",
	"completed": "已完成", "done": "已完成", "cancelled": "已取消", "canceled": "已取消",
	"low": "低", "normal": "中", "medium": "中", "high": "高", "urgent": "紧急",
	"task": "任务", "subtask": "子任务", "triage": "待确认", "duplicate": "重复",
	"p0": "致命", "p1": "严重", "p2": "一般", "p3": "轻微", "none": "未设置",
}

func distributionRows(counts map[string]int, total int) []map[string]any {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		percent := float64(0)
		if total > 0 {
			percent = float64(counts[key]) * 100 / float64(total)
		}
		label := biLabels[key]
		if label == "" {
			label = key
		}
		rows = append(rows, map[string]any{"key": key, "label": label, "count": counts[key], "percent": percent})
	}
	return rows
}

func taskDistribution(tasks []task.Snapshot, field string) []map[string]any {
	counts := map[string]int{}
	for _, task := range tasks {
		key := "none"
		switch field {
		case "priority":
			key = normalizeStatus(task.Priority)
		case "kind":
			key = normalizeStatus(task.Kind)
		case "status":
			key = normalizeStatus(task.Status)
		}
		if key == "" {
			key = "none"
		}
		counts[key]++
	}
	return distributionRows(counts, len(tasks))
}

func bugDistribution(bugs []quality.BugSnapshot, field string) []map[string]any {
	counts := map[string]int{}
	for _, bug := range bugs {
		key := normalizeStatus(bug.Status)
		if field == "severity" {
			key = normalizeStatus(bug.SeverityKey)
		}
		if key == "" {
			key = "none"
		}
		counts[key]++
	}
	return distributionRows(counts, len(bugs))
}

func (a biAggregation) dueDistribution() []map[string]any {
	counts := map[string]int{"overdue": 0, "due_soon": 0, "later": 0, "unset": 0}
	unfinished := 0
	for _, task := range a.Tasks {
		if isTaskTerminal(task.Status) {
			continue
		}
		unfinished++
		switch {
		case task.DueAt == nil:
			counts["unset"]++
		case task.DueAt.Before(a.Now):
			counts["overdue"]++
		case task.DueAt.Before(a.Now.AddDate(0, 0, 7)):
			counts["due_soon"]++
		default:
			counts["later"]++
		}
	}
	labels := map[string]string{"overdue": "已逾期", "due_soon": "7 天内到期", "later": "后续到期", "unset": "未设置"}
	rows := distributionRows(counts, unfinished)
	for _, row := range rows {
		row["label"] = labels[row["key"].(string)]
	}
	return rows
}

func (a biAggregation) completionTrend() []map[string]any {
	rows := make([]map[string]any, 0, a.Days)
	for offset := a.Days - 1; offset >= 0; offset-- {
		day, start, end := a.businessDay(offset)
		created, completed := 0, 0
		for _, task := range a.Tasks {
			if !task.CreatedAt.Before(start) && task.CreatedAt.Before(end) {
				created++
			}
			if isTaskDone(task.Status) && task.CompletedAt != nil &&
				!task.CompletedAt.Before(start) && task.CompletedAt.Before(end) {
				completed++
			}
		}
		rows = append(rows, map[string]any{
			"date": day.Format("2006-01-02"), "label": day.Format("01-02"),
			"created": created, "completed": completed,
		})
	}
	return rows
}

func (a biAggregation) progressRows(byProject bool) []map[string]any {
	type progress struct{ total, done, overdue int }
	counts := map[uint]*progress{}
	for _, task := range a.Tasks {
		id := task.RequirementID
		if byProject {
			id = task.PlanUnitID
		}
		row := counts[id]
		if row == nil {
			row = &progress{}
			counts[id] = row
		}
		row.total++
		if isTaskDone(task.Status) {
			row.done++
		}
		if !isTaskTerminal(task.Status) && task.DueAt != nil && task.DueAt.Before(a.Now) {
			row.overdue++
		}
	}
	ids := make([]uint, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	rows := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		label := a.Ctx.Requirements[id].Title
		if byProject {
			label = a.Ctx.Projects[id].Title
		}
		row := counts[id]
		rate := float64(0)
		if row.total > 0 {
			rate = float64(row.done) * 100 / float64(row.total)
		}
		rows = append(rows, map[string]any{
			"id": id, "label": label, "total_tasks": row.total, "done_tasks": row.done,
			"overdue_tasks": row.overdue, "done_rate_pct": rate,
		})
	}
	return rows
}

func (a biAggregation) versionProgress() []map[string]any     { return a.progressRows(true) }
func (a biAggregation) requirementProgress() []map[string]any { return a.progressRows(false) }

func (a biAggregation) versionRankings() []map[string]any {
	progress := a.versionProgress()
	bugCounts := map[uint]int{}
	for _, bug := range a.Bugs {
		if isBugOpen(bug.Status) {
			bugCounts[bug.PlanUnitID]++
		}
	}
	rows := make([]map[string]any, 0, len(progress))
	for _, item := range progress {
		id := item["id"].(uint)
		project := a.Ctx.Projects[id]
		rows = append(rows, map[string]any{
			"milestone_id": id, "milestone_kind": project.Type, "title": project.Title,
			"team_id": project.TeamID, "project_id": project.ProjectSetID,
			"due_at": project.PlannedEndAt, "status": project.Status,
			"total_tasks": item["total_tasks"], "done_tasks": item["done_tasks"],
			"overdue_tasks": item["overdue_tasks"], "open_bugs": bugCounts[id],
			"done_rate_pct": item["done_rate_pct"],
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["done_rate_pct"].(float64) < rows[j]["done_rate_pct"].(float64) })
	return rows
}

func (a biAggregation) ownerBoard() []map[string]any {
	type owner struct{ total, overdue, done int }
	counts := map[uint]*owner{}
	for _, task := range a.Tasks {
		id := uint(0)
		if task.AssigneeID != nil {
			id = *task.AssigneeID
		}
		row := counts[id]
		if row == nil {
			row = &owner{}
			counts[id] = row
		}
		row.total++
		if isTaskDone(task.Status) {
			row.done++
		}
		if !isTaskTerminal(task.Status) && task.DueAt != nil && task.DueAt.Before(a.Now) {
			row.overdue++
		}
	}
	rows := make([]map[string]any, 0, len(counts))
	for id, row := range counts {
		name := "未分配"
		avatarURL := ""
		if id > 0 {
			name = biUserDisplayName(a.Ctx, &id)
			avatarURL = biUserAvatarURL(a.Ctx, &id)
			if name == "" {
				name = retiredUserDisplayName(id)
			}
		}
		rows = append(rows, map[string]any{
			"owner_id": id, "owner_name": name, "owner_avatar_url": avatarURL,
			"total_tasks":   row.total,
			"overdue_tasks": row.overdue, "normal_tasks": row.total - row.overdue - row.done, "done_tasks": row.done,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["total_tasks"].(int) > rows[j]["total_tasks"].(int) })
	return rows
}
