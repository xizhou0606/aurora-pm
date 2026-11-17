package report

import (
	"time"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

// UIBugMetrics 汇总设计岗位的「UI 缺陷」处理情况：缺陷域为 ui（视觉/
// 还原/交互走查类）且负责人为该成员的缺陷。这是设计岗当前唯一可从系统
// 度量的质量结果信号——上游依赖流程约定：UI/视觉走查问题录缺陷时选择
// 「UI」域。口径与 BugMetrics 一致（Resolved 按解决时间落周期，Open/Overdue
// 为当前快照）。
type UIBugMetrics struct {
	Resolved int `json:"resolved"`
	Open     int `json:"open"`
	Overdue  int `json:"overdue"`
}

// aggregateUIBugs counts one person's UI-domain defects: resolved inside the
// period window, plus the open/overdue snapshot. 域判定复用 quality 模块的
// 归一化（legacy 行空值/未知值读作 functional，不计入设计口径）。
func aggregateUIBugs(snapshots []quality.BugSnapshot, period Period, now time.Time) UIBugMetrics {
	metrics := UIBugMetrics{}
	for _, snapshot := range snapshots {
		if quality.NormalizeBugDefectDomain(snapshot.DefectDomain) != quality.BugDefectDomainUI {
			continue
		}
		if withinPeriod(snapshot.ResolvedAt, period) {
			metrics.Resolved++
		}
		if openBugStatuses[snapshot.Status] {
			metrics.Open++
			if snapshot.DueAt != nil && snapshot.DueAt.Before(now) {
				metrics.Overdue++
			}
		}
	}
	return metrics
}
