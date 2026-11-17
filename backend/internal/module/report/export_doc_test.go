package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func sampleReport(kind ReportTargetKind) Report {
	period := Period{
		Type:       PeriodWeekly,
		Start:      time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		AnchorDate: "2026-09-06",
	}
	completedAt := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	due := completedAt.AddDate(0, 0, 2)
	report := composeReport(ReportFacts{
		Target: ReportTarget{Kind: kind, ID: 11, Name: "张三"},
		Tasks:  []task.Snapshot{taskSnapshot(kind, "T-1", "完成登录改造", "completed", &completedAt, &due)},
		Bugs:   []quality.BugSnapshot{bugSnapshot("B-9", "修复越权", "p1", "resolved", &completedAt, &due)},
	}, period, period.End)
	return report
}

// The snapshot builders below keep the test independent from the BI read
// modules while matching the aggregation input shape.
func taskSnapshot(_ ReportTargetKind, key, title, status string, completedAt, dueAt *time.Time) task.Snapshot {
	return task.Snapshot{
		ID: 1, TaskKey: key, Title: title, Status: status,
		CompletedAt: completedAt, DueAt: dueAt,
		CreatedAt: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
	}
}

func bugSnapshot(key, title, severity, status string, resolvedAt, dueAt *time.Time) quality.BugSnapshot {
	return quality.BugSnapshot{
		ID: 9, BugKey: key, Title: title, Status: status,
		SeverityKey: severity, ResolvedAt: resolvedAt, DueAt: dueAt,
	}
}

func TestComposeReportDocumentMarkdownCarriesFactsScoresAndNarrative(t *testing.T) {
	report := sampleReport(TargetMember)
	markdown := composeReportDocumentMarkdown(report, "## 📌 本周概览\n一切顺利。")

	checks := []string{
		"> 📌 对象：张三",
		"统计区间：2026-08-31 至 2026-09-06",
		"## 📊 数据指标",
		"| 完成任务 | 1 |",
		"## 🎯 多维评分（综合",
		"| 交付完成 |",
		"| 按期交付 |",
		"评分为统计口径自动计算",
		"## ✅ 本周完成",
		"T-1 完成登录改造",
		"## 🔄 进行中与风险",
		"## 🤖 AI 评述",
		"一切顺利",
	}
	for _, want := range checks {
		if !strings.Contains(markdown, want) {
			t.Fatalf("document markdown missing %q in:\n%s", want, markdown)
		}
	}
}

func TestComposeReportDocumentMarkdownGroupCarriesMemberTable(t *testing.T) {
	report := sampleReport(TargetGroup)
	report.Members = []MemberRollup{{
		Member: MemberRef{ID: 3, DisplayName: "李四"},
		Tasks:  TaskMetrics{Completed: 3, Active: 1},
		Bugs:   BugMetrics{Resolved: 1, Open: 2},
	}}
	markdown := composeReportDocumentMarkdown(report, "团队进展。")
	if !strings.Contains(markdown, "## 👥 成员概览") {
		t.Fatalf("group document must carry the member table:\n%s", markdown)
	}
	if !strings.Contains(markdown, "| 李四 | 3 | 1 | 0 | 1 | 2 |") {
		t.Fatalf("member rollup row must render:\n%s", markdown)
	}
}

func TestComposeReportRunContentFallsBackToSummaryOnLegacyInput(t *testing.T) {
	if got := composeReportRunContent("not-json", "纯评述"); got != "纯评述" {
		t.Fatalf("garbage input must degrade to summary, got %q", got)
	}
	payload, err := json.Marshal(reportRunPayload(ai.CapabilityReportWeekly, 7, sampleReport(TargetMember)))
	if err != nil {
		t.Fatal(err)
	}
	content := composeReportRunContent(string(payload), "评述内容")
	if !strings.Contains(content, "多维评分") || !strings.Contains(content, "评述内容") {
		t.Fatalf("run content must carry facts and narrative:\n%s", content)
	}
}

func TestComposeReportDocumentMarkdownTruncationNote(t *testing.T) {
	period := Period{Type: PeriodDaily, AnchorDate: "2026-09-06"}
	tasks := TaskMetrics{Completed: 23, CompletedItems: make([]TaskReportItem, 0, reportedItemsLimit)}
	for index := 0; index < reportedItemsLimit; index++ {
		completedAt := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
		tasks.CompletedItems = append(tasks.CompletedItems, TaskReportItem{
			ID: uint(index + 1), Title: "任务", CompletedAt: &completedAt,
		})
	}
	markdown := composeReportDocumentMarkdown(Report{
		Target: ReportTarget{Kind: TargetMember, ID: 1, Name: "张三"},
		Period: period, Tasks: tasks, Scores: ComputeReportScores(tasks, BugMetrics{}, period, nil),
	}, "评述")
	if !strings.Contains(markdown, "（共 23 条，仅展示 20 条）") {
		t.Fatalf("document must explain truncation:\n%s", markdown)
	}
}
