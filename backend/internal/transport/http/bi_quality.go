package httpapi

import (
	"sort"
	"time"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

func (a biAggregation) qualityTrend() []map[string]any {
	rows := make([]map[string]any, 0, a.Days)
	for offset := a.Days - 1; offset >= 0; offset-- {
		day, start, end := a.businessDay(offset)
		open, high, created, resolved := 0, 0, 0, 0
		for _, bug := range a.Bugs {
			if !bug.CreatedAt.Before(start) && bug.CreatedAt.Before(end) {
				created++
			}
			if bug.CreatedAt.Before(end) {
				terminalAt := biBugTerminalAt(bug)
				if terminalAt == nil || !terminalAt.Before(end) {
					open++
					if isHighSeverity(bug.SeverityKey) {
						high++
					}
				}
			}
			resolvedAt := biBugResolvedAt(bug)
			if resolvedAt != nil && !resolvedAt.Before(start) && resolvedAt.Before(end) {
				resolved++
			}
		}
		rows = append(rows, map[string]any{
			"date": day.Format("2006-01-02"), "label": day.Format("01-02"),
			"open_bugs": open, "high_severity_bugs": high,
			"created_bugs": created, "resolved_bugs": resolved,
		})
	}
	return rows
}

func biBugTerminalAt(bug quality.BugSnapshot) *time.Time {
	status := normalizeStatus(bug.Status)
	switch status {
	case "done", "completed", "closed":
		if bug.CompletedAt != nil {
			return bug.CompletedAt
		}
		if bug.ResolvedAt != nil {
			return bug.ResolvedAt
		}
	case "canceled", "cancelled":
		if bug.CanceledAt != nil {
			return bug.CanceledAt
		}
	default:
		return nil
	}
	if bug.UpdatedAt.IsZero() {
		return nil
	}
	return &bug.UpdatedAt
}

func biBugResolvedAt(bug quality.BugSnapshot) *time.Time {
	if bug.ResolvedAt != nil {
		return bug.ResolvedAt
	}
	status := normalizeStatus(bug.Status)
	if status != "done" && status != "completed" && status != "closed" {
		return nil
	}
	if bug.CompletedAt != nil {
		return bug.CompletedAt
	}
	if bug.UpdatedAt.IsZero() {
		return nil
	}
	return &bug.UpdatedAt
}

func isHighSeverity(value string) bool {
	value = normalizeStatus(value)
	return value == "p0" || value == "p1" || value == "fatal" || value == "critical"
}

func isBIQualityHighPriority(value string) bool {
	value = normalizeStatus(value)
	return value == "urgent" || value == "high"
}

func isBIQualityReview(value string) bool {
	value = normalizeStatus(value)
	return value == "review" || value == "resolved"
}

func biQualitySeverityBucket(value string) string {
	switch normalizeStatus(value) {
	case "p0", "fatal":
		return "fatal"
	case "p1", "critical":
		return "critical"
	case "p2", "major":
		return "major"
	default:
		return "minor"
	}
}

func (a biAggregation) qualitySeverityDistribution() []map[string]any {
	counts := map[string]int{"fatal": 0, "critical": 0, "major": 0, "minor": 0}
	total := 0
	for _, bug := range a.Bugs {
		if !isBugOpen(bug.Status) {
			continue
		}
		counts[biQualitySeverityBucket(bug.SeverityKey)]++
		total++
	}
	items := []struct {
		key, label, tone string
	}{
		{key: "fatal", label: "致命", tone: "red"},
		{key: "critical", label: "严重", tone: "orange"},
		{key: "major", label: "一般", tone: "amber"},
		{key: "minor", label: "轻微", tone: "teal"},
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		percent := 0
		if total > 0 && counts[item.key] > 0 {
			percent = (counts[item.key]*100 + total/2) / total
		}
		rows = append(rows, map[string]any{
			"key": item.key, "label": item.label, "count": counts[item.key],
			"percent": percent, "tone": item.tone,
		})
	}
	return rows
}

func (a biAggregation) versionQualityRisks() []map[string]any {
	type risk struct{ open, high int }
	counts := map[uint]*risk{}
	for _, bug := range a.Bugs {
		if !isBugOpen(bug.Status) {
			continue
		}
		row := counts[bug.PlanUnitID]
		if row == nil {
			row = &risk{}
			counts[bug.PlanUnitID] = row
		}
		row.open++
		if isHighSeverity(bug.SeverityKey) {
			row.high++
		}
	}
	rows := make([]map[string]any, 0, len(counts))
	for id, row := range counts {
		level, label := "low", "低风险"
		if row.high > 0 {
			level, label = "high", "高风险"
		} else if row.open >= 3 {
			level, label = "medium", "中风险"
		}
		rows = append(rows, map[string]any{
			"version_id": id, "version_name": a.Ctx.Projects[id].Title,
			"open_bugs": row.open, "high_severity_bugs": row.high,
			"risk_level": level, "risk_label": label,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["open_bugs"].(int) > rows[j]["open_bugs"].(int) })
	return rows
}

func (a biAggregation) qualityOwnerLoad() []map[string]any {
	type load struct{ pending, review, overdue, open, high int }
	counts := map[uint]*load{}
	for _, bug := range a.Bugs {
		if !isBugOpen(bug.Status) {
			continue
		}
		owner := biQualityCurrentOwner(bug)
		id := uint(0)
		if owner != nil {
			id = *owner
		}
		row := counts[id]
		if row == nil {
			row = &load{}
			counts[id] = row
		}
		row.open++
		if isBIQualityReview(bug.Status) {
			row.review++
		} else {
			row.pending++
		}
		if bug.DueAt != nil && bug.DueAt.Before(a.Now) {
			row.overdue++
		}
		if isHighSeverity(bug.SeverityKey) {
			row.high++
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
		percent := float64(0)
		if row.open > 0 {
			percent = float64(row.high) * 100 / float64(row.open)
		}
		rows = append(rows, map[string]any{
			"owner_id": id, "owner_name": name, "owner_avatar_url": avatarURL,
			"pending_bugs": row.pending,
			"review_bugs":  row.review, "overdue_bugs": row.overdue, "open_bugs": row.open,
			"high_severity_bugs": row.high, "high_severity_pct": percent,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["open_bugs"].(int) > rows[j]["open_bugs"].(int) })
	return rows
}
