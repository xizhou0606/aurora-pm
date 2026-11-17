package report

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

type fakeDirectory struct {
	members     []identity.DirectoryMember
	groups      []identity.UserGroup
	departments []identity.DirectoryDepartment
	department  *identity.WorkspaceDepartment
	err         error
}

func (fake fakeDirectory) ListWorkspaceMembers(context.Context, uint) ([]identity.DirectoryMember, error) {
	return fake.members, fake.err
}

func (fake fakeDirectory) ListEnabledGroups(context.Context) ([]identity.UserGroup, error) {
	return fake.groups, nil
}

func (fake fakeDirectory) ListFeishuDepartments(context.Context) ([]identity.DirectoryDepartment, error) {
	return fake.departments, nil
}

func (fake fakeDirectory) FindWorkspaceMember(_ context.Context, _ uint, factID uint) (identity.DirectoryMember, error) {
	for _, member := range fake.members {
		if member.FactID == factID {
			return member, nil
		}
	}
	return identity.DirectoryMember{}, identity.ErrReportingDirectoryMemberNotFound
}

func (fake fakeDirectory) FindWorkspaceDepartment(
	_ context.Context, _ uint, departmentID uint,
) (identity.WorkspaceDepartment, error) {
	if fake.department != nil && fake.department.ID == departmentID {
		return *fake.department, nil
	}
	return identity.WorkspaceDepartment{}, identity.ErrReportingDirectoryMemberNotFound
}

func (fake fakeDirectory) FindWorkspaceGroup(_ context.Context, _ uint, groupID uint) (identity.WorkspaceGroup, error) {
	if groupID != 21 {
		return identity.WorkspaceGroup{}, identity.ErrReportingDirectoryMemberNotFound
	}
	group := identity.WorkspaceGroup{ID: 21, Code: "dev", Name: "研发组", Members: []identity.DirectoryMember{}}
	for _, member := range fake.members {
		if member.GroupID == groupID {
			group.Members = append(group.Members, member)
		}
	}
	return group, nil
}

type fakeTaskFacts struct {
	tasks []task.Snapshot
}

func (fake fakeTaskFacts) ListBIFacts(context.Context, uint) ([]task.Snapshot, []task.ActivitySnapshot, error) {
	return fake.tasks, nil, nil
}

type fakeBugFacts struct {
	bugs []quality.BugSnapshot
}

func (fake fakeBugFacts) ListBIFacts(context.Context, uint) ([]quality.BugSnapshot, []quality.BugActivitySnapshot, error) {
	return fake.bugs, nil, nil
}

type fakeRunStarter struct {
	result   ai.ReportRunStartResult
	err      error
	commands []ai.ReportRunStartCommand
}

func (fake *fakeRunStarter) StartReportRun(_ context.Context, command ai.ReportRunStartCommand) (ai.ReportRunStartResult, error) {
	fake.commands = append(fake.commands, command)
	return fake.result, fake.err
}

type fakeRunReader struct {
	result ai.ReportRunResult
	err    error
}

func (fake fakeRunReader) FindReportRunWithInput(context.Context, uint, uint) (ai.ReportRunResult, error) {
	return fake.result, nil
}

func (fake fakeRunReader) FindReportRun(context.Context, uint, uint) (ai.ReportRunResult, error) {
	return fake.result, fake.err
}

func (fake fakeRunReader) FindLatestReportRun(context.Context, ai.ReportRunLatestQuery) (ai.ReportRunResult, error) {
	return ai.ReportRunResult{}, ai.ErrReportRunNotFound
}

type fakeExporter struct {
	documents CreatedDocument
	title     string
	content   string
	err       error
}

func (fake *fakeExporter) CreateReportDocument(_ context.Context, title, content string) (CreatedDocument, error) {
	fake.title, fake.content = title, content
	return fake.documents, fake.err
}

func newTestService(t *testing.T, tasks []task.Snapshot, bugs []quality.BugSnapshot, runs *fakeRunStarter) *Service {
	t.Helper()
	service, err := NewService(Dependencies{
		Directory: fakeDirectory{members: []identity.DirectoryMember{
			{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupID: 21, GroupCode: "dev", GroupName: "研发组", RoleNames: []string{"后端"}, AvatarURL: "https://example.test/a.png"},
			{ID: 12, FactID: 102, Username: "beta", DisplayName: "Beta", GroupID: 21, GroupCode: "dev", GroupName: "研发组"},
		}, groups: []identity.UserGroup{
			{ID: 21, Code: "dev", Name: "研发组", Enabled: true},
		}},
		Tasks: fakeTaskFacts{tasks: tasks}, Bugs: fakeBugFacts{bugs: bugs},
		Runs: runs, RunResults: fakeRunReader{},
	})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) // Wednesday
	service.now = func() time.Time { return base }
	return service
}

func uintPtr(value uint) *uint           { return &value }
func timePtr(value time.Time) *time.Time { return &value }

func TestMemberDirectoryGroupsByRoleGroup(t *testing.T) {
	service := newTestService(t, nil, nil, &fakeRunStarter{})
	view, err := service.MemberDirectory(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if view.Total != 2 || len(view.Groups) != 1 || len(view.Ungrouped) != 0 {
		t.Fatalf("view=%+v", view)
	}
	if view.Groups[0].Name != "研发组" || len(view.Groups[0].Members) != 2 {
		t.Fatalf("group=%+v", view.Groups[0])
	}
	// Avatars flow through the canonical directory projection.
	if view.Groups[0].Members[0].AvatarURL != "https://example.test/a.png" {
		t.Fatalf("avatar=%q", view.Groups[0].Members[0].AvatarURL)
	}
	if _, err := service.MemberDirectory(context.Background(), 0); !errors.Is(err, ErrReportQueryInvalid) {
		t.Fatalf("err=%v, want invalid query", err)
	}
}

func TestMemberDirectoryCarriesDepartmentIDs(t *testing.T) {
	// 回归锁定：目录接口的部门节点必须带内部 id——前端靠它作为部门汇报的
	// subject 引用，缺失时整个组织树会被前端过滤掉。
	service := newTestService(t, nil, nil, &fakeRunStarter{})
	service.dependencies.Directory = fakeDirectory{
		members: []identity.DirectoryMember{
			{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupID: 21, GroupName: "研发组", DepartmentID: "od-a"},
		},
		groups: []identity.UserGroup{{ID: 21, Code: "dev", Name: "研发组", Enabled: true}},
		departments: []identity.DirectoryDepartment{
			{ID: 5, OpenID: "od-a", ParentID: "0", Name: "前端一组", MemberCount: 1},
		},
	}
	view, err := service.MemberDirectory(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Departments) != 1 || view.Departments[0].ID != 5 || view.Departments[0].OpenID != "od-a" {
		t.Fatalf("departments=%+v", view.Departments)
	}
}

func TestResolvePeriodWeekStartsOnMonday(t *testing.T) {
	anchor, err := ParseAnchorDate("2026-09-02", time.UTC, time.Date(2026, 9, 2, 23, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	period, err := ResolvePeriod(PeriodWeekly, anchor, time.UTC, anchor.AddDate(0, 0, 8))
	if err != nil {
		t.Fatal(err)
	}
	if period.Start.UTC() != time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("start=%v", period.Start)
	}
	if period.End.UTC() != time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("end=%v", period.End)
	}
	// AnchorDate 归一为覆盖区间最后一天（落款日期）。
	if period.AnchorDate != "2026-09-06" {
		t.Fatalf("anchor=%q", period.AnchorDate)
	}
	if _, err := ResolvePeriod(PeriodType("yearly"), anchor, time.UTC, anchor.AddDate(0, 0, 8)); !errors.Is(err, ErrReportPeriodInvalid) {
		t.Fatalf("err=%v, want invalid period", err)
	}
}

// 周报只回顾已结束的完整周期：锚点落在本周时自动回退到上周（R4 产品口径）。
func TestResolvePeriodShiftsIncompleteWeeklyToLastWeek(t *testing.T) {
	monday, err := ParseAnchorDate("2026-09-07", time.UTC, time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	period, err := ResolvePeriod(PeriodWeekly, monday, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	if period.Start.UTC() != time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("start=%v", period.Start)
	}
	if period.End.UTC() != time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("end=%v", period.End)
	}
	if period.AnchorDate != "2026-09-06" {
		t.Fatalf("anchor=%q", period.AnchorDate)
	}

	// 月报同理：锚点在本月时回退到上一个自然月。
	period, err = ResolvePeriod(PeriodMonthly, monday, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	if period.Start.UTC() != time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("month start=%v", period.Start)
	}
	if period.End.UTC() != time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("month end=%v", period.End)
	}

	// 日报保持锚点当天（晨间进展口径），不回退。
	period, err = ResolvePeriod(PeriodDaily, monday, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	if period.Start.UTC() != time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("daily start=%v", period.Start)
	}

	// 历史完整周期不回退：锚点选已结束月份（8 月）任意一天，窗口保持该月。
	august, err := ParseAnchorDate("2026-08-15", time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	period, err = ResolvePeriod(PeriodMonthly, august, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	if period.Start.UTC() != time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).UTC() {
		t.Fatalf("august start=%v", period.Start)
	}
	if period.AnchorDate != "2026-08-31" {
		t.Fatalf("august anchor=%q", period.AnchorDate)
	}
}

func aggregateFixtures(period Period) ([]task.Snapshot, []quality.BugSnapshot) {
	tasks := []task.Snapshot{
		{ID: 1, TaskKey: "T-1", Title: "done", Status: "completed", AssigneeID: uintPtr(101), CompletedAt: timePtr(period.Start.Add(time.Hour)), CreatedAt: period.Start},
		{ID: 2, TaskKey: "T-2", Title: "active", Status: "in_progress", AssigneeID: uintPtr(101), DueAt: timePtr(period.Start.Add(-time.Hour)), CreatedAt: period.Start},
		{ID: 3, TaskKey: "T-3", Title: "group mate", Status: "completed", AssigneeID: uintPtr(102), CompletedAt: timePtr(period.Start.Add(2 * time.Hour)), CreatedAt: period.Start},
		{ID: 4, TaskKey: "T-4", Title: "cancelled", Status: "cancelled", AssigneeID: uintPtr(101), CreatedAt: period.Start},
	}
	bugs := []quality.BugSnapshot{
		{ID: 5, BugKey: "B-1", Title: "resolved", Status: "review", AssigneeUserID: uintPtr(101), ResolvedAt: timePtr(period.Start.Add(2 * time.Hour))},
		{ID: 6, BugKey: "B-2", Title: "closed", Status: "done", AssigneeUserID: uintPtr(101), CompletedAt: timePtr(period.Start.Add(3 * time.Hour))},
		{ID: 7, BugKey: "B-3", Title: "open overdue", Status: "todo", AssigneeUserID: uintPtr(101), DueAt: timePtr(period.Start.Add(-time.Hour))},
	}
	return tasks, bugs
}

func TestMemberReportAggregatesTaskAndBugFacts(t *testing.T) {
	period, err := ResolvePeriod(PeriodDaily, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), time.UTC, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 8))
	if err != nil {
		t.Fatal(err)
	}
	tasks, bugs := aggregateFixtures(period)
	service := newTestService(t, tasks, bugs, &fakeRunStarter{})
	result, err := service.MemberReport(context.Background(), 7, 101, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != TargetMember || result.Target.ID != 101 || result.Target.Name != "Alpha" {
		t.Fatalf("target=%+v", result.Target)
	}
	if result.Tasks.Completed != 1 || len(result.Tasks.CompletedItems) != 1 || result.Tasks.CompletedItems[0].Key != "T-1" {
		t.Fatalf("tasks=%+v", result.Tasks)
	}
	if result.Tasks.Created != 2 || result.Tasks.Active != 1 || result.Tasks.Overdue != 1 {
		t.Fatalf("tasks=%+v", result.Tasks)
	}
	if result.Bugs.Resolved != 1 || result.Bugs.Closed != 1 || result.Bugs.Open != 2 || result.Bugs.Overdue != 1 {
		t.Fatalf("bugs=%+v", result.Bugs)
	}
}

// 汇报目标必须透传生效角色与飞书原始职务：前端头衔区按 role_names 渲染角色
// chips，role_names 为空时用 job_title 做「职务」兜底标注，两者都空则不渲染。
func TestMemberReportTargetCarriesRoleNamesAndJobTitle(t *testing.T) {
	service := newTestService(t, nil, nil, &fakeRunStarter{})
	service.dependencies.Directory = fakeDirectory{
		members: []identity.DirectoryMember{
			{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupID: 21, GroupName: "研发组", RoleNames: []string{"后端", "技术负责人"}, JobTitle: "高级工程师"},
			{ID: 12, FactID: 102, Username: "beta", DisplayName: "Beta", GroupID: 21, GroupName: "研发组", JobTitle: "测试工程师"},
			{ID: 13, FactID: 103, Username: "gamma", DisplayName: "Gamma", GroupID: 21, GroupName: "研发组"},
		},
	}
	result, err := service.MemberReport(context.Background(), 7, 101, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Target.RoleNames) != 2 || result.Target.RoleNames[0] != "后端" || result.Target.RoleNames[1] != "技术负责人" {
		t.Fatalf("role names=%+v", result.Target.RoleNames)
	}
	if result.Target.JobTitle != "高级工程师" {
		t.Fatalf("job title=%q", result.Target.JobTitle)
	}
	// HTTP 契约：非空 JobTitle 以 job_title 字段输出。
	payload, err := json.Marshal(result.Target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"job_title":"高级工程师"`) {
		t.Fatalf("payload=%s", payload)
	}

	// 角色为空时职务仍然透传（职务兜底来源）。
	fallback, err := service.MemberReport(context.Background(), 7, 102, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(fallback.Target.RoleNames) != 0 || fallback.Target.JobTitle != "测试工程师" {
		t.Fatalf("fallback target=%+v", fallback.Target)
	}

	// 角色与职务都空：不输出 job_title，前端据此不渲染任何角色元素。
	bare, err := service.MemberReport(context.Background(), 7, 103, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(bare.Target.RoleNames) != 0 || bare.Target.JobTitle != "" {
		t.Fatalf("bare target=%+v", bare.Target)
	}
	payload, err = json.Marshal(bare.Target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "job_title") {
		t.Fatalf("bare payload=%s", payload)
	}
}

func TestGroupReportAggregatesAllGroupMembers(t *testing.T) {
	period, err := ResolvePeriod(PeriodDaily, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), time.UTC, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 8))
	if err != nil {
		t.Fatal(err)
	}
	tasks, _ := aggregateFixtures(period)
	service := newTestService(t, tasks, nil, &fakeRunStarter{})
	result, err := service.GroupReport(context.Background(), 7, 21, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != TargetGroup || result.Target.Name != "研发组" {
		t.Fatalf("target=%+v", result.Target)
	}
	// The group rollup covers both members: two completed tasks in total.
	if result.Tasks.Completed != 2 {
		t.Fatalf("tasks=%+v", result.Tasks)
	}
	if _, err := service.GroupReport(context.Background(), 7, 99, PeriodDaily, "", time.UTC); !errors.Is(err, identity.ErrReportingDirectoryMemberNotFound) {
		t.Fatalf("err=%v, want member not found", err)
	}
}

func TestDepartmentReportAggregatesSubtreeMembers(t *testing.T) {
	period, err := ResolvePeriod(PeriodDaily, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), time.UTC, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 8))
	if err != nil {
		t.Fatal(err)
	}
	tasks, _ := aggregateFixtures(period)
	service := newTestService(t, tasks, nil, &fakeRunStarter{})
	members := []identity.DirectoryMember{
		{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupID: 21, GroupName: "研发组", RoleNames: []string{"后端"}, DepartmentID: "od-parent"},
		{ID: 12, FactID: 102, Username: "beta", DisplayName: "Beta", GroupID: 21, GroupName: "研发组", DepartmentID: "od-child"},
	}
	service.dependencies.Directory = fakeDirectory{
		members: members,
		groups:  []identity.UserGroup{{ID: 21, Code: "dev", Name: "研发组", Enabled: true}},
		departments: []identity.DirectoryDepartment{
			{ID: 5, OpenID: "od-parent", ParentID: "0", Name: "前端一组", MemberCount: 1},
			{ID: 6, OpenID: "od-child", ParentID: "od-parent", Name: "前端二组", MemberCount: 1},
		},
		department: &identity.WorkspaceDepartment{
			ID: 5, OpenID: "od-parent", Name: "前端一组",
			// The service recomputes the subtree; this member list is what the
			// fake returns (matching the od-parent subtree).
			Members: []identity.DirectoryMember{members[0], members[1]},
		},
	}
	result, err := service.DepartmentReport(context.Background(), 7, 5, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != TargetDepartment || result.Target.Name != "前端一组" {
		t.Fatalf("target=%+v", result.Target)
	}
	if result.Tasks.Completed != 2 {
		t.Fatalf("department aggregate tasks=%+v", result.Tasks)
	}
	if len(result.Members) != 2 {
		t.Fatalf("member rollups=%d, want 2", len(result.Members))
	}
	if _, err := service.DepartmentReport(context.Background(), 7, 99, PeriodDaily, "", time.UTC); !errors.Is(err, identity.ErrReportingDirectoryMemberNotFound) {
		t.Fatalf("err=%v, want department not found", err)
	}
	// 事实版导出文档带部门后缀与成员概览。
	markdown := composeReportDocumentMarkdown(result, "")
	if !strings.Contains(markdown, "（部门）") || !strings.Contains(markdown, "成员概览") {
		t.Fatalf("department document markdown incomplete:\n%s", markdown)
	}
}

func TestMemberReportFailsClosedForUnknownMember(t *testing.T) {
	service := newTestService(t, nil, nil, &fakeRunStarter{})
	if _, err := service.MemberReport(context.Background(), 7, 999, PeriodDaily, "", time.UTC); !errors.Is(err, identity.ErrReportingDirectoryMemberNotFound) {
		t.Fatalf("err=%v, want member not found", err)
	}
}

func TestStartReportQueuesAISkillRun(t *testing.T) {
	runs := &fakeRunStarter{result: ai.ReportRunStartResult{RunID: 77, Status: "queued", Enabled: true}}
	service := newTestService(t, nil, nil, runs)
	result, err := service.StartReport(context.Background(), 7, 9, TargetMember, 101, PeriodWeekly, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != 77 || result.Report.Target.ID != 101 {
		t.Fatalf("result=%+v", result)
	}
	if len(runs.commands) != 1 {
		t.Fatalf("commands=%+v", runs.commands)
	}
	command := runs.commands[0]
	if command.Capability != ai.CapabilityReportWeekly || command.SubjectID != 101 ||
		command.SubjectType != ai.ReportSubjectMember || command.PrincipalID != 9 || command.WorkspaceID != 7 {
		t.Fatalf("command=%+v", command)
	}
	var input struct {
		Subject struct {
			Target ReportTarget `json:"target"`
			Period Period       `json:"period"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(command.Input, &input); err != nil {
		t.Fatal(err)
	}
	if input.Subject.Target.ID != 101 || input.Subject.Period.Type != PeriodWeekly {
		t.Fatalf("input=%+v", input)
	}

	if _, err := service.StartReport(context.Background(), 7, 9, TargetGroup, 21, PeriodDaily, "", time.UTC); err != nil {
		t.Fatal(err)
	}
	if runs.commands[1].SubjectType != ai.ReportSubjectUserGroup || runs.commands[1].SubjectID != 21 {
		t.Fatalf("group command=%+v", runs.commands[1])
	}
}

func TestStartReportFailsClosedWithoutAI(t *testing.T) {
	service := newTestService(t, nil, nil, &fakeRunStarter{err: ai.ErrReportRunUnavailable})
	_, err := service.StartReport(context.Background(), 7, 9, TargetMember, 101, PeriodDaily, "", time.UTC)
	if !errors.Is(err, ai.ErrReportRunUnavailable) {
		t.Fatalf("err=%v, want unavailable", err)
	}
	if _, err := service.StartReport(context.Background(), 7, 9, TargetMember, 999, PeriodDaily, "", time.UTC); !errors.Is(err, identity.ErrReportingDirectoryMemberNotFound) {
		t.Fatalf("err=%v, want member not found", err)
	}
}

func TestExportReportRunPublishesCompletedNarrative(t *testing.T) {
	completed := ai.ReportRunResult{
		ID: 77, Status: "completed", Capability: ai.CapabilityReportDaily,
		SubjectType: ai.ReportSubjectMember, SubjectID: 101, Summary: "今日完成 3 项任务",
		InputJSON: `{"subject":{"target":{"name":"Alpha"},"period":{"type":"daily","anchor_date":"2026-09-02"}}}`,
	}
	service, err := NewService(Dependencies{
		Directory: fakeDirectory{}, Tasks: fakeTaskFacts{}, Bugs: fakeBugFacts{},
		Runs: &fakeRunStarter{}, RunResults: fakeRunReader{result: completed},
		Exporter: &fakeExporter{documents: CreatedDocument{DocumentID: "doc-1", URL: "https://example.test/docx/doc-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := service.ExportReportRun(context.Background(), 7, 77)
	if err != nil {
		t.Fatal(err)
	}
	if document.DocumentID != "doc-1" || document.URL != "https://example.test/docx/doc-1" {
		t.Fatalf("document=%+v", document)
	}
	exporter := service.dependencies.Exporter.(*fakeExporter)
	if exporter.title != "【日报】Alpha 2026-09-02" || exporter.content != "今日完成 3 项任务" {
		t.Fatalf("exported title=%q content=%q", exporter.title, exporter.content)
	}

	// An unfinished run fails closed before touching the exporter.
	pending := completed
	pending.Status = "running"
	unfinished, err := NewService(Dependencies{
		Directory: fakeDirectory{}, Tasks: fakeTaskFacts{}, Bugs: fakeBugFacts{},
		Runs: &fakeRunStarter{}, RunResults: fakeRunReader{result: pending},
		Exporter: &fakeExporter{documents: CreatedDocument{DocumentID: "doc-2", URL: "https://example.test/docx/doc-2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unfinished.ExportReportRun(context.Background(), 7, 77); !errors.Is(err, ErrReportRunNotCompleted) {
		t.Fatalf("err=%v, want run not completed", err)
	}
	service2, err := NewService(Dependencies{
		Directory: fakeDirectory{}, Tasks: fakeTaskFacts{}, Bugs: fakeBugFacts{},
		Runs: &fakeRunStarter{}, RunResults: fakeRunReader{result: pending},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service2.ExportReportRun(context.Background(), 7, 77); !errors.Is(err, ErrReportExportUnavailable) {
		t.Fatalf("err=%v, want export unavailable", err)
	}
}

func TestMemberReportRunStatusScopesWorkspace(t *testing.T) {
	service, err := NewService(Dependencies{
		Directory: fakeDirectory{}, Tasks: fakeTaskFacts{}, Bugs: fakeBugFacts{},
		Runs: &fakeRunStarter{}, RunResults: fakeRunReader{err: ai.ErrReportRunNotFound},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.MemberReportRunStatus(context.Background(), 7, 77); !errors.Is(err, ai.ErrReportRunNotFound) {
		t.Fatalf("err=%v, want not found", err)
	}
	if _, err := service.MemberReportRunStatus(context.Background(), 0, 77); !errors.Is(err, ErrReportQueryInvalid) {
		t.Fatalf("err=%v, want invalid query", err)
	}
}

func TestCompletedItemsTruncateToNewest(t *testing.T) {
	period, err := ResolvePeriod(PeriodDaily, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), time.UTC, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 8))
	if err != nil {
		t.Fatal(err)
	}
	// 21 completed tasks with DESCENDING ids over ASCENDING completion times:
	// the newest completed item is id 21 and must open the list (R3).
	tasks := make([]task.Snapshot, 0, 21)
	for index := 1; index <= 21; index++ {
		completedAt := period.Start.Add(time.Duration(index) * time.Minute)
		tasks = append(tasks, task.Snapshot{
			ID: uint(index), TaskKey: "T", Title: "task", Status: "completed",
			AssigneeID: uintPtr(101), CompletedAt: timePtr(completedAt), CreatedAt: period.Start,
		})
	}
	service := newTestService(t, tasks, nil, &fakeRunStarter{})
	result, err := service.MemberReport(context.Background(), 7, 101, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if result.Tasks.Completed != 21 {
		t.Fatalf("completed=%d, want 21", result.Tasks.Completed)
	}
	if len(result.Tasks.CompletedItems) != reportedItemsLimit {
		t.Fatalf("items=%d, want %d", len(result.Tasks.CompletedItems), reportedItemsLimit)
	}
	if result.Tasks.CompletedItems[0].ID != 21 || result.Tasks.CompletedItems[0].Key != "T" {
		t.Fatalf("first item=%+v", result.Tasks.CompletedItems[0])
	}
	if result.Tasks.CompletedItems[19].ID != 2 {
		t.Fatalf("last item id=%d, want 2", result.Tasks.CompletedItems[19].ID)
	}
}

func TestOpenItemsSortBySeverityThenDue(t *testing.T) {
	period, err := ResolvePeriod(PeriodDaily, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), time.UTC, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 8))
	if err != nil {
		t.Fatal(err)
	}
	lateDue := period.Start.Add(-48 * time.Hour)
	soonDue := period.Start.Add(24 * time.Hour)
	bugs := []quality.BugSnapshot{
		{ID: 1, BugKey: "B-p3", Title: "p3 undated", Status: "todo", AssigneeUserID: uintPtr(101), SeverityKey: "p3"},
		{ID: 2, BugKey: "B-p2", Title: "p2 due late", Status: "todo", AssigneeUserID: uintPtr(101), SeverityKey: "p2", DueAt: timePtr(lateDue)},
		{ID: 3, BugKey: "B-p0", Title: "p0 due soon", Status: "todo", AssigneeUserID: uintPtr(101), SeverityKey: "p0", DueAt: timePtr(soonDue)},
	}
	service := newTestService(t, nil, bugs, &fakeRunStarter{})
	result, err := service.MemberReport(context.Background(), 7, 101, PeriodDaily, "2026-09-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, item := range result.Bugs.OpenItems {
		got = append(got, item.Key)
	}
	if len(got) != 3 || got[0] != "B-p0" || got[1] != "B-p2" || got[2] != "B-p3" {
		t.Fatalf("open order=%v", got)
	}
}
