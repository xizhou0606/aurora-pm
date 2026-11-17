package report

import (
	"strings"
	"testing"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

var focusTestPeriod = Period{
	Type:       PeriodWeekly,
	Start:      time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
	End:        time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
	AnchorDate: "2026-09-06",
}

func TestMemberRoleFocusClassification(t *testing.T) {
	cases := []struct {
		name     string
		roles    []string
		jobTitle string
		want     roleFocus
	}{
		{name: "测试工程师", roles: []string{"测试工程师"}, want: focusTesting},
		{name: "测试开发算测试岗", roles: []string{"测试开发"}, want: focusTesting},
		{name: "QA 大小写不敏感", roles: []string{"qa"}, want: focusTesting},
		{name: "职务兜底质量", jobTitle: "质量经理", want: focusTesting},
		{name: "产品经理", roles: []string{"产品经理"}, want: focusProduct},
		{name: "产品职务兜底", jobTitle: "产品总监", want: focusProduct},
		{name: "UI设计师", roles: []string{"UI设计师"}, want: focusDesign},
		{name: "视觉设计职务兜底", jobTitle: "视觉设计总监", want: focusDesign},
		{name: "交互设计", roles: []string{"交互设计师"}, want: focusDesign},
		{name: "产品设计按产品优先", roles: []string{"产品设计"}, want: focusProduct},
		{name: "研发岗无焦点", roles: []string{"后端"}, want: focusNone},
		{name: "管理岗无焦点", roles: []string{"部门经理"}, want: focusNone},
		{name: "空角色无焦点", want: focusNone},
		{name: "产品测试以测试优先", roles: []string{"产品测试"}, want: focusTesting},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := memberRoleFocus(ReportTarget{RoleNames: testCase.roles, JobTitle: testCase.jobTitle})
			if got != testCase.want {
				t.Fatalf("focus=%q, want %q", got, testCase.want)
			}
		})
	}
}

func TestTeamRoleFocusClassification(t *testing.T) {
	cases := []struct {
		name string
		want roleFocus
	}{
		{"测试组", focusTesting},
		{"QA 部", focusTesting},
		{"质量部", focusTesting},
		{"产品组", focusProduct},
		{"设计组", focusDesign},
		{"UI 部", focusDesign},
		{"研发组", focusNone},
		{"", focusNone},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := teamRoleFocus(ReportTarget{Name: testCase.name}); got != testCase.want {
				t.Fatalf("focus=%q, want %q", got, testCase.want)
			}
		})
	}
}

func raisedBug(id uint, reporter uint, severity, status, resolution string, createdAt time.Time) quality.BugSnapshot {
	snapshot := quality.BugSnapshot{
		ID: id, Status: status, SeverityKey: severity, CreatedAt: createdAt,
		ReporterUserID: &reporter,
	}
	if resolution != "" {
		snapshot.ResolutionReason = resolution
	}
	return snapshot
}

func TestAggregateBugReportsCountsEffectiveSeverityBuckets(t *testing.T) {
	inPeriod := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	beforePeriod := focusTestPeriod.Start.AddDate(0, 0, -1)
	afterPeriod := focusTestPeriod.End.Add(time.Hour)
	snapshots := []quality.BugSnapshot{
		raisedBug(1, 101, "p0", "triage", "", inPeriod),
		raisedBug(2, 101, "p1", "in_progress", "", inPeriod),
		raisedBug(3, 101, "p2", "done", "", inPeriod),
		// P3 计入总数但不单列。
		raisedBug(4, 101, "p3", "todo", "", inPeriod),
		// 已取消与无效结论不计。
		raisedBug(5, 101, "p0", "canceled", "", inPeriod),
		raisedBug(6, 101, "p0", "done", "duplicate", inPeriod),
		raisedBug(7, 101, "p0", "done", "as_designed", inPeriod),
		raisedBug(8, 101, "p0", "done", "external", inPeriod),
		// 周期外的有效缺陷不计。
		raisedBug(9, 101, "p0", "triage", "", beforePeriod),
		raisedBug(10, 101, "p0", "triage", "", afterPeriod),
	}
	metrics := aggregateBugReports(snapshots, focusTestPeriod)
	if metrics.Total != 4 || metrics.P0 != 1 || metrics.P1 != 1 || metrics.P2 != 1 {
		t.Fatalf("metrics=%+v", metrics)
	}
}

func TestAggregateRequirementMetricsStatusDistributionAndPeriod(t *testing.T) {
	inPeriod := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	beforePeriod := focusTestPeriod.Start.AddDate(0, 0, -1)
	requirementAt := func(id uint, status string, createdAt time.Time) requirement.Snapshot {
		return requirement.Snapshot{ID: id, Status: status, CreatedAt: createdAt, ProductOwnerUserIDs: []uint{101}}
	}
	metrics := aggregateRequirementMetrics([]requirement.Snapshot{
		requirementAt(1, "done", inPeriod),
		requirementAt(2, "acceptance", beforePeriod),
		requirementAt(3, "development", inPeriod),
		requirementAt(4, "grooming", inPeriod),
		requirementAt(5, "backlog", inPeriod),
	}, focusTestPeriod)
	if metrics.Total != 5 || metrics.CreatedInPeriod != 4 ||
		metrics.Done != 1 || metrics.Acceptance != 1 || metrics.Development != 1 || metrics.Pending != 2 {
		t.Fatalf("metrics=%+v", metrics)
	}
}

func TestAggregateRequirementMetricsDistinctDeduplicatesCoOwnership(t *testing.T) {
	createdAt := focusTestPeriod.Start.AddDate(0, 0, 2)
	shared := requirement.Snapshot{ID: 7, Status: "done", CreatedAt: createdAt, ProductOwnerUserIDs: []uint{101, 102}}
	metrics := aggregateRequirementMetricsDistinct(
		[][]requirement.Snapshot{{{ID: 7, Status: "done", CreatedAt: createdAt, ProductOwnerUserIDs: []uint{101}}, shared}},
		focusTestPeriod,
	)
	if metrics.Total != 1 || metrics.Done != 1 {
		t.Fatalf("metrics=%+v", metrics)
	}
}

func focusedMemberFacts(name string, roles []string) ReportFacts {
	return ReportFacts{
		Target: ReportTarget{Kind: TargetMember, ID: 101, Name: name, RoleNames: roles},
		Tasks:  []task.Snapshot{},
		Bugs:   []quality.BugSnapshot{},
		ReportedBugs: map[uint][]quality.BugSnapshot{
			101: {
				raisedBug(1, 101, "p0", "triage", "", focusTestPeriod.Start.AddDate(0, 0, 1)),
				raisedBug(2, 101, "p2", "todo", "", focusTestPeriod.Start.AddDate(0, 0, 1)),
			},
		},
		OwnedRequirements: map[uint][]requirement.Snapshot{
			101: {{ID: 5, Status: "development", CreatedAt: focusTestPeriod.Start.AddDate(0, 0, 1), ProductOwnerUserIDs: []uint{101}}},
		},
		ContributionRows: map[uint][]DailyContributionFact{
			101: {{UserID: 101, RepoID: 1, Day: "2026-09-01", Commits: 3, Additions: 100}},
		},
	}
}

func TestComposeReportTestingMemberSwapsCodeForBugReports(t *testing.T) {
	report := composeReport(focusedMemberFacts("赵六", []string{"测试工程师"}), focusTestPeriod, focusTestPeriod.End)
	if report.Code != nil {
		t.Fatal("testing member must not carry a code block")
	}
	if report.BugReports == nil || report.BugReports.Total != 2 || report.BugReports.P0 != 1 || report.BugReports.P2 != 1 {
		t.Fatalf("bugReports=%+v", report.BugReports)
	}
	// 代码维度自动退出雷达，不稀释综合分。
	for _, dimension := range report.Scores.Dimensions {
		if dimension.Key == scoreKeyCode || dimension.Key == scoreKeyCodeQuality {
			t.Fatalf("code dimension %q must be absent for testing members", dimension.Key)
		}
	}
}

func TestComposeReportProductMemberSwapsCodeForRequirements(t *testing.T) {
	report := composeReport(focusedMemberFacts("钱七", []string{"产品经理"}), focusTestPeriod, focusTestPeriod.End)
	if report.Code != nil {
		t.Fatal("product member must not carry a code block")
	}
	if report.Requirements == nil || report.Requirements.Total != 1 || report.Requirements.Development != 1 {
		t.Fatalf("requirements=%+v", report.Requirements)
	}
	if report.BugReports != nil {
		t.Fatal("product member must not carry bug reports")
	}
}

func TestComposeReportDeveloperMemberKeepsCode(t *testing.T) {
	report := composeReport(focusedMemberFacts("张三", []string{"后端"}), focusTestPeriod, focusTestPeriod.End)
	if report.Code == nil || report.Code.Commits != 3 {
		t.Fatalf("code=%+v", report.Code)
	}
	if report.BugReports != nil || report.Requirements != nil {
		t.Fatal("developer member must not carry focus facts")
	}
}

func TestComposeReportTestingTeamSwapsAggregateAndRollups(t *testing.T) {
	memberA := memberRef(identity.DirectoryMember{FactID: 101, DisplayName: "赵六", RoleNames: []string{"测试工程师"}})
	memberB := memberRef(identity.DirectoryMember{FactID: 102, DisplayName: "孙八", RoleNames: []string{"测试工程师"}})
	report := composeReport(ReportFacts{
		Target: ReportTarget{Kind: TargetGroup, ID: 31, Name: "测试组"},
		Tasks:  []task.Snapshot{},
		Bugs:   []quality.BugSnapshot{},
		Members: []MemberRollupFacts{
			{Member: memberA}, {Member: memberB},
		},
		ReportedBugs: map[uint][]quality.BugSnapshot{
			101: {raisedBug(1, 101, "p0", "triage", "", focusTestPeriod.Start.AddDate(0, 0, 1))},
			102: {raisedBug(2, 102, "p1", "todo", "", focusTestPeriod.Start.AddDate(0, 0, 1))},
		},
		OwnedRequirements: map[uint][]requirement.Snapshot{},
		ContributionRows: map[uint][]DailyContributionFact{
			101: {{UserID: 101, RepoID: 1, Day: "2026-09-01", Commits: 9, Additions: 500}},
		},
	}, focusTestPeriod, focusTestPeriod.End)
	if report.Code != nil {
		t.Fatal("testing team must not carry an aggregate code block")
	}
	if report.BugReports == nil || report.BugReports.Total != 2 || report.BugReports.P0 != 1 || report.BugReports.P1 != 1 {
		t.Fatalf("bugReports=%+v", report.BugReports)
	}
	if len(report.Members) != 2 {
		t.Fatalf("members=%+v", report.Members)
	}
	for _, rollup := range report.Members {
		if rollup.Code != nil {
			t.Fatalf("rollup %s must not carry code", rollup.Member.DisplayName)
		}
		if rollup.BugReports == nil || rollup.BugReports.Total != 1 {
			t.Fatalf("rollup %s bugReports=%+v", rollup.Member.DisplayName, rollup.BugReports)
		}
	}
}

func TestComposeReportProductTeamAggregateDeduplicatesRequirements(t *testing.T) {
	memberA := memberRef(identity.DirectoryMember{FactID: 101, DisplayName: "钱七", RoleNames: []string{"产品经理"}})
	memberB := memberRef(identity.DirectoryMember{FactID: 102, DisplayName: "孙八", RoleNames: []string{"产品经理"}})
	createdAt := focusTestPeriod.Start.AddDate(0, 0, 1)
	// 需求 7 由两位成员共同负责：个人各计一次，团队聚合去重为一次。
	report := composeReport(ReportFacts{
		Target: ReportTarget{Kind: TargetDepartment, ID: 41, Name: "产品部"},
		Tasks:  []task.Snapshot{},
		Bugs:   []quality.BugSnapshot{},
		Members: []MemberRollupFacts{
			{Member: memberA}, {Member: memberB},
		},
		ReportedBugs: map[uint][]quality.BugSnapshot{},
		OwnedRequirements: map[uint][]requirement.Snapshot{
			101: {{ID: 7, Status: "done", CreatedAt: createdAt, ProductOwnerUserIDs: []uint{101, 102}}},
			102: {{ID: 7, Status: "done", CreatedAt: createdAt, ProductOwnerUserIDs: []uint{101, 102}}},
		},
		ContributionRows: map[uint][]DailyContributionFact{},
	}, focusTestPeriod, focusTestPeriod.End)
	if report.Code != nil {
		t.Fatal("product team must not carry an aggregate code block")
	}
	if report.Requirements == nil || report.Requirements.Total != 1 {
		t.Fatalf("requirements=%+v", report.Requirements)
	}
	for _, rollup := range report.Members {
		if rollup.Requirements == nil || rollup.Requirements.Total != 1 {
			t.Fatalf("rollup %s requirements=%+v", rollup.Member.DisplayName, rollup.Requirements)
		}
	}
}

func TestExportDocumentRendersRoleFocusSections(t *testing.T) {
	testingReport := composeReport(focusedMemberFacts("赵六", []string{"测试工程师"}), focusTestPeriod, focusTestPeriod.End)
	testingMarkdown := composeReportDocumentMarkdown(testingReport, "")
	if !strings.Contains(testingMarkdown, "## 🐞 提出 Bug") || !strings.Contains(testingMarkdown, "P0 致命") {
		t.Fatalf("testing export must carry the bug report section: %s", testingMarkdown)
	}
	if strings.Contains(testingMarkdown, "## 💻 代码活动") {
		t.Fatal("testing export must not carry the code section")
	}

	productReport := composeReport(focusedMemberFacts("钱七", []string{"产品经理"}), focusTestPeriod, focusTestPeriod.End)
	productMarkdown := composeReportDocumentMarkdown(productReport, "")
	if !strings.Contains(productMarkdown, "## 📋 需求概览") || !strings.Contains(productMarkdown, "本期新增") {
		t.Fatalf("product export must carry the requirement section: %s", productMarkdown)
	}
}

func TestAggregateUIBugsCountsOnlyUIDomain(t *testing.T) {
	inPeriod := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	beforePeriod := focusTestPeriod.Start.AddDate(0, 0, -1)
	bug := func(id uint, domain, status string, resolvedAt *time.Time, dueAt *time.Time) quality.BugSnapshot {
		return quality.BugSnapshot{ID: id, DefectDomain: domain, Status: status, ResolvedAt: resolvedAt, DueAt: dueAt, SeverityKey: "p2"}
	}
	now := focusTestPeriod.End.AddDate(0, 0, 3)
	metrics := aggregateUIBugs([]quality.BugSnapshot{
		bug(1, "ui", "resolved", &inPeriod, nil),
		bug(2, "UI", "done", &inPeriod, nil),                        // 大小写归一
		bug(3, "ui", "review", nil, nil),                            // 待处理
		bug(4, "ui", "review", nil, timePtr(now.AddDate(0, 0, -1))), // 待处理且已逾期
		bug(5, "functional", "resolved", &inPeriod, nil),            // 功能域不计
		bug(6, "", "resolved", &inPeriod, nil),                      // 空域按 functional 不计
		bug(7, "ui", "resolved", &beforePeriod, nil),                // 周期外解决不计
	}, focusTestPeriod, now)
	if metrics.Resolved != 2 || metrics.Open != 2 || metrics.Overdue != 1 {
		t.Fatalf("metrics=%+v", metrics)
	}
}

func TestComposeReportDesignMemberSwapsCodeForUIBugs(t *testing.T) {
	facts := focusedMemberFacts("周九", []string{"UI设计师"})
	// focusedMemberFacts 的 Bugs 桶为空；补一条 UI 域缺陷（负责人口径）。
	facts.Bugs = []quality.BugSnapshot{
		{ID: 21, DefectDomain: "ui", Status: "resolved", SeverityKey: "p1", ResolvedAt: timePtr(focusTestPeriod.Start.AddDate(0, 0, 2))},
		{ID: 22, DefectDomain: "ui", Status: "review", SeverityKey: "p2"},
		{ID: 23, DefectDomain: "functional", Status: "resolved", SeverityKey: "p2", ResolvedAt: timePtr(focusTestPeriod.Start.AddDate(0, 0, 2))},
	}
	report := composeReport(facts, focusTestPeriod, focusTestPeriod.End)
	if report.Code != nil {
		t.Fatal("design member must not carry a code block")
	}
	if report.UIBugs == nil || report.UIBugs.Resolved != 1 || report.UIBugs.Open != 1 {
		t.Fatalf("uiBugs=%+v", report.UIBugs)
	}
	if report.BugReports != nil || report.Requirements != nil {
		t.Fatal("design member must not carry other focus facts")
	}
	for _, dimension := range report.Scores.Dimensions {
		if dimension.Key == scoreKeyCode || dimension.Key == scoreKeyCodeQuality {
			t.Fatalf("code dimension %q must be absent for design members", dimension.Key)
		}
	}
}

func TestComposeReportDesignTeamRollupsCarryUIBugs(t *testing.T) {
	memberA := memberRef(identity.DirectoryMember{FactID: 101, DisplayName: "周九", RoleNames: []string{"UI设计师"}})
	uiBug := quality.BugSnapshot{ID: 31, DefectDomain: "ui", Status: "resolved", SeverityKey: "p2", ResolvedAt: timePtr(focusTestPeriod.Start.AddDate(0, 0, 1))}
	report := composeReport(ReportFacts{
		Target: ReportTarget{Kind: TargetGroup, ID: 51, Name: "设计组"},
		Tasks:  []task.Snapshot{},
		Bugs:   []quality.BugSnapshot{uiBug},
		Members: []MemberRollupFacts{
			{Member: memberA, Bugs: []quality.BugSnapshot{uiBug}},
		},
		ReportedBugs:      map[uint][]quality.BugSnapshot{},
		OwnedRequirements: map[uint][]requirement.Snapshot{},
		ContributionRows:  map[uint][]DailyContributionFact{},
	}, focusTestPeriod, focusTestPeriod.End)
	if report.Code != nil {
		t.Fatal("design team must not carry an aggregate code block")
	}
	if report.UIBugs == nil || report.UIBugs.Resolved != 1 {
		t.Fatalf("uiBugs=%+v", report.UIBugs)
	}
	if len(report.Members) != 1 || report.Members[0].UIBugs == nil || report.Members[0].UIBugs.Resolved != 1 {
		t.Fatalf("rollups=%+v", report.Members)
	}
}

func TestExportDocumentRendersDesignUIBugSection(t *testing.T) {
	facts := focusedMemberFacts("周九", []string{"UI设计师"})
	facts.Bugs = []quality.BugSnapshot{
		{ID: 21, DefectDomain: "ui", Status: "resolved", SeverityKey: "p1", ResolvedAt: timePtr(focusTestPeriod.Start.AddDate(0, 0, 2))},
	}
	report := composeReport(facts, focusTestPeriod, focusTestPeriod.End)
	markdown := composeReportDocumentMarkdown(report, "")
	if !strings.Contains(markdown, "## 🎨 UI 缺陷") || !strings.Contains(markdown, "本期解决") {
		t.Fatalf("design export must carry the UI bug section: %s", markdown)
	}
	if strings.Contains(markdown, "## 💻 代码活动") {
		t.Fatal("design export must not carry the code section")
	}
}
