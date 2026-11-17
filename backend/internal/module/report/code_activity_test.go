package report

import (
	"strings"
	"testing"
	"time"
)

func codeTestPeriod() Period {
	return Period{
		Type:       PeriodWeekly,
		Start:      time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2026, 9, 6, 23, 59, 59, 0, time.UTC),
		AnchorDate: "2026-09-06",
	}
}

func TestComposeReportAddsCodeActivity(t *testing.T) {
	rows := []DailyContributionFact{
		{RepoID: 11, UserID: 5, Day: "2026-09-02", Commits: 2, Additions: 30, Deletions: 4,
			PRsMerged: 1, ReviewsCompleted: 2,
			SourceAdditions: 20, TestAdditions: 6, DocAdditions: 3, ConfigAdditions: 1, PRsReviewed: 1},
		// 周期外的行必须被排除
		{RepoID: 11, UserID: 5, Day: "2026-09-20", Commits: 9, Additions: 900},
	}
	facts := ReportFacts{
		Target:                ReportTarget{Kind: TargetMember, ID: 5, Name: "Alice"},
		ContributionRows:      map[uint][]DailyContributionFact{5: rows, 0: {{RepoID: 11, UserID: 0, Day: "2026-09-02", Commits: 1, Additions: 7}}},
		ContributionFreshness: &CodeFreshness{Degraded: true, Error: "api rate limited"},
	}
	report := composeReport(facts, codeTestPeriod(), time.Now())
	if report.Code == nil {
		t.Fatal("wired port must produce a code block")
	}
	if report.Code.Commits != 2 || report.Code.Additions != 30 || report.Code.Deletions != 4 ||
		report.Code.PRsMerged != 1 || report.Code.ReviewsCompleted != 2 {
		t.Fatalf("code totals wrong: %+v", report.Code)
	}
	if report.Code.Repos != 1 {
		t.Fatalf("repos = %d, want 1", report.Code.Repos)
	}
	if report.Code.SourceAdditions != 20 || report.Code.TestAdditions != 6 ||
		report.Code.DocAdditions != 3 || report.Code.ConfigAdditions != 1 {
		t.Fatalf("class split wrong: %+v", report.Code)
	}
	if report.Code.PRsReviewed != 1 {
		t.Fatalf("prs reviewed = %d, want 1", report.Code.PRsReviewed)
	}
	if !report.Code.Freshness.Degraded || report.Code.Freshness.Error != "api rate limited" {
		t.Fatalf("freshness wrong: %+v", report.Code.Freshness)
	}
	// 未接入端口时保持 nil，页面渲染与旧报告完全一致。
	bare := composeReport(ReportFacts{Target: facts.Target}, codeTestPeriod(), time.Now())
	if bare.Code != nil {
		t.Fatal("unwired port must not emit a code block")
	}
}

func TestGroupReportSumsMemberBuckets(t *testing.T) {
	facts := ReportFacts{
		Target: ReportTarget{Kind: TargetGroup, ID: 1, Name: "platform"},
		Members: []MemberRollupFacts{
			{Member: MemberRef{ID: 5, DisplayName: "Alice"}},
			{Member: MemberRef{ID: 6, DisplayName: "Bob"}},
		},
		ContributionRows: map[uint][]DailyContributionFact{
			5: {{RepoID: 11, UserID: 5, Day: "2026-09-02", Commits: 2, Additions: 30}},
			6: {{RepoID: 12, UserID: 6, Day: "2026-09-03", PRsMerged: 1}},
		},
		CommitRows: map[uint][]MemberCommitFact{
			5: {{UserID: 5, SHA: "a", DebugHits: 1, TodoHits: 2}, {UserID: 5, SHA: "b", TodoHits: 1}},
			// 有逐提交证据但无贡献行的成员：质量命中仍要出现在代码块里。
			6: {{UserID: 6, SHA: "c", DebugHits: 3}},
		},
		ContributionFreshness: &CodeFreshness{},
	}
	report := composeReport(facts, codeTestPeriod(), time.Now())
	if report.Code == nil || report.Code.Commits != 2 || report.Code.Additions != 30 || report.Code.PRsMerged != 1 {
		t.Fatalf("group code totals wrong: %+v", report.Code)
	}
	if report.Code.Repos != 2 {
		t.Fatalf("group repos = %d, want 2", report.Code.Repos)
	}
	if len(report.Members) != 2 || report.Members[0].Code == nil || report.Members[0].Code.Commits != 2 {
		t.Fatalf("member rollup code wrong: %+v", report.Members)
	}
	// 代码质量评比：每个成员的调试残留/TODO 命中各自聚合。
	if report.Members[0].Code.DebugHits != 1 || report.Members[0].Code.TodoHits != 3 {
		t.Fatalf("alice hygiene wrong: %+v", report.Members[0].Code)
	}
	if report.Members[1].Code == nil || report.Members[1].Code.DebugHits != 3 {
		t.Fatalf("bob hygiene wrong: %+v", report.Members[1].Code)
	}
}

func TestExportDocIncludesCodeSection(t *testing.T) {
	facts := ReportFacts{
		Target: ReportTarget{Kind: TargetMember, ID: 5, Name: "Alice"},
		ContributionRows: map[uint][]DailyContributionFact{
			5: {{RepoID: 11, UserID: 5, Day: "2026-09-02", Commits: 2, Additions: 30, Deletions: 4}},
		},
		ContributionFreshness: &CodeFreshness{Degraded: true, Error: "token expired"},
	}
	report := composeReport(facts, codeTestPeriod(), time.Now())
	document := composeReportDocumentMarkdown(report, "")
	if !strings.Contains(document, "## 💻 代码活动") {
		t.Fatalf("document missing code section:\n%s", document)
	}
	if !strings.Contains(document, "token expired") || !strings.Contains(document, "不代表当期无活动") {
		t.Fatal("degraded freshness must be stated explicitly")
	}
	// 未接入时文档不含空章节。
	bare := composeReport(ReportFacts{Target: ReportTarget{Kind: TargetMember, ID: 5, Name: "Alice"}}, codeTestPeriod(), time.Now())
	if strings.Contains(composeReportDocumentMarkdown(bare, ""), "代码活动") {
		t.Fatal("unwired report must not contain an empty code section")
	}
}

func TestComposeReportCarriesCommitReviews(t *testing.T) {
	commits := []MemberCommitFact{
		{UserID: 5, SHA: "abcdef1234567890", CommittedAt: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
			Subject: "重构配置读取", Additions: 300, Deletions: 100, Files: 4,
			DebugHits: 2, TodoHits: 1, DiffSample: "console.log(1)"},
		{UserID: 5, SHA: "bbbb0000", CommittedAt: time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC),
			Subject: "修复空指针", Additions: 12, Deletions: 6, Files: 2},
	}
	facts := ReportFacts{
		Target:     ReportTarget{Kind: TargetMember, ID: 5, Name: "Alice"},
		CommitRows: map[uint][]MemberCommitFact{5: commits},
	}
	report := composeReport(facts, codeTestPeriod(), time.Now())
	if report.Code == nil {
		t.Fatal("commit rows must produce a code block")
	}
	if report.Code.DebugHits != 2 || report.Code.TodoHits != 1 {
		t.Fatalf("hit totals wrong: %+v", report.Code)
	}
	if len(report.Code.CommitReviews) != 2 {
		t.Fatalf("commit reviews = %d, want 2", len(report.Code.CommitReviews))
	}
	first := report.Code.CommitReviews[0]
	if first.SHA != "abcdef12" || first.Day != "2026-09-02" || first.DiffSample == "" {
		t.Fatalf("first review item wrong: %+v", first)
	}
	if report.Code.CommitReviews[1].DiffSample != "" {
		t.Fatalf("commit without sample must stay empty")
	}
}
