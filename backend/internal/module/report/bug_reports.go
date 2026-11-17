package report

import (
	"strings"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

// BugReportMetrics 汇总统计周期内一个成员「提出的缺陷」：以提出人
// （reporter_user_id）计，剔除已取消与判定无效（重复/设计如此/外部）的
// 记录，与 BI 有效缺陷口径一致。P0/P1/P2 三档并列计数，P3（轻微）不单列。
// 这是测试岗位汇报的核心事实面板与 AI 评语的主要依据。
type BugReportMetrics struct {
	Total int `json:"total"`
	P0    int `json:"p0"`
	P1    int `json:"p1"`
	P2    int `json:"p2"`
}

// BucketBugsByReporter partitions bug snapshots once per workspace pass by
// reporter, mirroring BucketTasksByAssignee. Bugs without an attributed
// reporter belong to no bucket.
func BucketBugsByReporter(snapshots []quality.BugSnapshot) map[uint][]quality.BugSnapshot {
	buckets := make(map[uint][]quality.BugSnapshot)
	for _, snapshot := range snapshots {
		if snapshot.ReporterUserID == nil || *snapshot.ReporterUserID == 0 {
			continue
		}
		buckets[*snapshot.ReporterUserID] = append(buckets[*snapshot.ReporterUserID], snapshot)
	}
	return buckets
}

// aggregateBugReports counts one person's raised bugs inside the period
// window. The window edges follow the same day-accurate semantics as
// aggregateCode: Start inclusive, End exclusive.
func aggregateBugReports(snapshots []quality.BugSnapshot, period Period) BugReportMetrics {
	metrics := BugReportMetrics{}
	for _, snapshot := range snapshots {
		if !effectiveReportedBug(snapshot) || !withinPeriod(&snapshot.CreatedAt, period) {
			continue
		}
		metrics.Total++
		switch strings.ToLower(strings.TrimSpace(snapshot.SeverityKey)) {
		case "p0":
			metrics.P0++
		case "p1":
			metrics.P1++
		case "p2":
			metrics.P2++
		}
	}
	return metrics
}

// effectiveReportedBug mirrors the BI 有效缺陷口径: not canceled and no
// invalid resolution conclusion (duplicate/as_designed/external).
func effectiveReportedBug(snapshot quality.BugSnapshot) bool {
	status := strings.ToLower(strings.TrimSpace(snapshot.Status))
	if status == "canceled" || status == "cancelled" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(snapshot.ResolutionReason)) {
	case "duplicate", "as_designed", "external":
		return false
	default:
		return true
	}
}
