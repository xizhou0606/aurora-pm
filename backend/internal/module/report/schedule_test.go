package report

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type fakeWorkspaces struct{ ids []uint }

func (fake fakeWorkspaces) ListEnabledWorkspaceIDs(context.Context) ([]uint, error) {
	return fake.ids, nil
}

type fakeDedup struct {
	existing map[string]bool
	queries  []ai.ReportRunDedupBatchQuery
}

func dedupKey(workspaceID uint, capability, subjectType string, subjectID uint) string {
	return fmt.Sprintf("%d/%s/%s/%d", workspaceID, capability, subjectType, subjectID)
}

func (fake *fakeDedup) HasReportRun(_ context.Context, query ai.ReportRunDedupQuery) (bool, error) {
	return fake.existing[dedupKey(query.WorkspaceID, query.Capability, query.SubjectType, query.SubjectID)], nil
}

// ListReportRunSubjectIDs mirrors the batched DB query: only requested
// subjects with a persisted run come back.
func (fake *fakeDedup) ListReportRunSubjectIDs(
	_ context.Context,
	query ai.ReportRunDedupBatchQuery,
) (map[uint]bool, error) {
	fake.queries = append(fake.queries, query)
	existing := map[uint]bool{}
	for _, subjectID := range query.SubjectIDs {
		if fake.existing[dedupKey(query.WorkspaceID, query.Capability, query.SubjectType, subjectID)] {
			existing[subjectID] = true
		}
	}
	return existing, nil
}

func newSchedulerService(t *testing.T, runs *fakeRunStarter) *Service {
	t.Helper()
	service, err := NewService(Dependencies{
		Directory: fakeDirectory{members: []identity.DirectoryMember{
			{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupID: 21, GroupCode: "dev", GroupName: "研发组"},
			{ID: 12, FactID: 102, Username: "beta", DisplayName: "Beta"},
		}, groups: []identity.UserGroup{
			{ID: 21, Code: "dev", Name: "研发组", Enabled: true},
		}},
		Tasks: fakeTaskFacts{}, Bugs: fakeBugFacts{},
		Runs: runs, RunResults: fakeRunReader{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestGenerateForWorkspaceQueuesMembersAndGroupsWithDedup(t *testing.T) {
	runs := &fakeRunStarter{result: ai.ReportRunStartResult{RunID: 1, Status: "queued", Enabled: true}}
	service := newSchedulerService(t, runs)
	dedup := &fakeDedup{existing: map[string]bool{}}
	scheduler, err := NewReportScheduler(service, dedup, fakeWorkspaces{ids: []uint{7}})
	if err != nil {
		t.Fatal(err)
	}
	location := ScheduleLocation()
	summary, err := scheduler.GenerateForWorkspace(
		context.Background(), 7, PeriodDaily, "2026-09-04",
		time.Date(2026, 9, 4, 0, 0, 0, 0, location), time.Date(2026, 9, 5, 0, 0, 0, 0, location), location,
	)
	if err != nil {
		t.Fatal(err)
	}
	// Two members + one group with members; ungrouped beta contributes no group.
	if summary.Created != 3 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("summary=%+v", summary)
	}
	// Scheduled runs carry no operator principal.
	for _, command := range runs.commands {
		if !command.Scheduled || command.PrincipalID != 0 {
			t.Fatalf("command=%+v", command)
		}
	}

	// Simulate persistence: the first pass's runs are now visible to dedup.
	for _, command := range runs.commands {
		dedup.existing[dedupKey(
			command.WorkspaceID, command.Capability, command.SubjectType, command.SubjectID,
		)] = true
	}

	// A second pass for the same anchor date skips every subject.
	summary, err = scheduler.GenerateForWorkspace(
		context.Background(), 7, PeriodDaily, "2026-09-04",
		time.Date(2026, 9, 4, 0, 0, 0, 0, location), time.Date(2026, 9, 5, 0, 0, 0, 0, location), location,
	)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Skipped != 3 || summary.Created != 0 {
		t.Fatalf("second pass summary=%+v", summary)
	}
}

func TestGenerateForWorkspaceQueuesDepartmentReports(t *testing.T) {
	runs := &fakeRunStarter{result: ai.ReportRunStartResult{RunID: 1, Status: "queued", Enabled: true}}
	service, err := NewService(Dependencies{
		Directory: fakeDirectory{
			members: []identity.DirectoryMember{
				{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", DepartmentID: "od-a"},
			},
			groups:      []identity.UserGroup{},
			departments: []identity.DirectoryDepartment{{ID: 5, OpenID: "od-a", Name: "产品部", MemberCount: 1}},
			department: &identity.WorkspaceDepartment{ID: 5, Name: "产品部", Members: []identity.DirectoryMember{
				{FactID: 101, Username: "alpha", DisplayName: "Alpha", DepartmentID: "od-a"},
			}},
		},
		Tasks: fakeTaskFacts{}, Bugs: fakeBugFacts{},
		Runs: runs, RunResults: fakeRunReader{},
	})
	if err != nil {
		t.Fatal(err)
	}
	dedup := &fakeDedup{existing: map[string]bool{}}
	scheduler, err := NewReportScheduler(service, dedup, fakeWorkspaces{ids: []uint{7}})
	if err != nil {
		t.Fatal(err)
	}
	location := ScheduleLocation()
	summary, err := scheduler.GenerateForWorkspace(
		context.Background(), 7, PeriodWeekly, "2026-09-07",
		time.Date(2026, 9, 7, 0, 0, 0, 0, location), time.Date(2026, 9, 8, 0, 0, 0, 0, location), location,
	)
	if err != nil {
		t.Fatal(err)
	}
	// One member + one department run; the empty group catalog adds nothing.
	if summary.Created != 2 {
		t.Fatalf("summary=%+v", summary)
	}
	var departmentRun *ai.ReportRunStartCommand
	for index := range runs.commands {
		if runs.commands[index].SubjectType == ai.ReportSubjectDepartment {
			departmentRun = &runs.commands[index]
		}
	}
	if departmentRun == nil || departmentRun.SubjectID != 5 || departmentRun.Capability != ai.CapabilityReportWeekly {
		t.Fatalf("department commands=%+v", runs.commands)
	}
	// 部门汇报的输入必须带子树成员聚合，评语才能点名成员贡献。
	if !strings.Contains(string(departmentRun.Input), `"display_name":"Alpha"`) {
		t.Fatalf("department input missing member rollup: %.200s", departmentRun.Input)
	}
}

func TestGenerateForAllWorkspacesWalksEnabledWorkspaces(t *testing.T) {
	runs := &fakeRunStarter{result: ai.ReportRunStartResult{RunID: 1, Status: "queued", Enabled: true}}
	service := newSchedulerService(t, runs)
	scheduler, err := NewReportScheduler(service, &fakeDedup{}, fakeWorkspaces{ids: []uint{7, 8}})
	if err != nil {
		t.Fatal(err)
	}
	summaries, err := scheduler.GenerateForAllWorkspaces(context.Background(), PeriodWeekly, ScheduleLocation())
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("summaries=%+v", summaries)
	}
	// Alpha belongs to workspace-agnostic groups; every workspace sees the
	// same directory (members are global facts keyed by membership).
	if summaries[0].Created != 3 || summaries[1].Created != 3 {
		t.Fatalf("summaries=%+v", summaries)
	}
}

// TestScheduledPassCoversPreviousCompletePeriod pins the anchor decision of a
// scheduled pass: the daily generated at 09:00 covers the PREVIOUS natural
// day, while weekly/monthly keep the trigger-day anchor and rely on
// ResolvePeriod's previous-complete-period fallback (回补) to cover the last
// finished week/month.
func TestScheduledPassCoversPreviousCompletePeriod(t *testing.T) {
	runs := &fakeRunStarter{result: ai.ReportRunStartResult{RunID: 1, Status: "queued", Enabled: true}}
	service := newSchedulerService(t, runs)
	scheduler, err := NewReportScheduler(service, &fakeDedup{}, fakeWorkspaces{ids: []uint{7}})
	if err != nil {
		t.Fatal(err)
	}
	location := ScheduleLocation()
	fixed := func(year int, month time.Month, day, hour, minute int) func() time.Time {
		return func() time.Time { return time.Date(year, month, day, hour, minute, 0, 0, location) }
	}

	// 日报：周二 09:05 触发 → 覆盖周一（前一日）。
	scheduler.now = fixed(2026, 9, 8, 9, 5) // Tuesday
	summaries, err := scheduler.GenerateForAllWorkspaces(context.Background(), PeriodDaily, location)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].AnchorDate != "2026-09-07" {
		t.Fatalf("daily summary anchor = %+v, want 2026-09-07", summaries)
	}
	start, end, anchor := firstQueuedPeriodDays(t, runs, location)
	if start != "2026-09-07" || end != "2026-09-08" || anchor != "2026-09-07" {
		t.Fatalf("daily window = %s..%s anchor %s, want 2026-09-07..2026-09-08", start, end, anchor)
	}

	// 周报：周一 09:05 触发 → 回补上一完整周（上周一 00:00 至本周一 00:00）。
	runs.commands = nil
	scheduler.now = fixed(2026, 9, 7, 9, 5) // Monday
	summaries, err = scheduler.GenerateForAllWorkspaces(context.Background(), PeriodWeekly, location)
	if err != nil {
		t.Fatal(err)
	}
	start, end, anchor = firstQueuedPeriodDays(t, runs, location)
	if start != "2026-08-31" || end != "2026-09-07" || anchor != "2026-09-06" {
		t.Fatalf("weekly window = %s..%s anchor %s, want 2026-08-31..2026-09-07 anchor 2026-09-06", start, end, anchor)
	}

	// 月报：1 号 09:05 触发 → 回补上一完整月。
	runs.commands = nil
	scheduler.now = fixed(2026, 9, 1, 9, 5)
	if _, err = scheduler.GenerateForAllWorkspaces(context.Background(), PeriodMonthly, location); err != nil {
		t.Fatal(err)
	}
	start, end, anchor = firstQueuedPeriodDays(t, runs, location)
	if start != "2026-08-01" || end != "2026-09-01" || anchor != "2026-08-31" {
		t.Fatalf("monthly window = %s..%s anchor %s, want 2026-08-01..2026-09-01 anchor 2026-08-31", start, end, anchor)
	}
}

// firstQueuedPeriodDays reads the covered [start, end) business days plus the
// report's anchor date out of the first queued run payload.
func firstQueuedPeriodDays(t *testing.T, runs *fakeRunStarter, location *time.Location) (string, string, string) {
	t.Helper()
	if len(runs.commands) == 0 {
		t.Fatal("expected at least one queued run")
	}
	var payload struct {
		Subject struct {
			Period struct {
				Start      time.Time `json:"start"`
				End        time.Time `json:"end"`
				AnchorDate string    `json:"anchor_date"`
			} `json:"period"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(runs.commands[0].Input, &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Subject.Period.Start.In(location).Format(anchorDateLayout),
		payload.Subject.Period.End.In(location).Format(anchorDateLayout),
		payload.Subject.Period.AnchorDate
}

// TestDueTriggerMoments is the table-driven contract for the 09:00 trigger
// determination: daily fires once per natural day, weekly only on Monday
// 09:00, monthly only on the 1st 09:00 — each exactly once per occurrence,
// with stall backfill (回补) and month-end rollover covered.
func TestDueTriggerMoments(t *testing.T) {
	location := ScheduleLocation() // Asia/Shanghai, no DST
	at := func(year int, month time.Month, day, hour, minute int) time.Time {
		return time.Date(year, month, day, hour, minute, 0, 0, location)
	}
	tests := []struct {
		name       string
		periodType PeriodType
		previous   time.Time
		now        time.Time
		want       bool
	}{
		// 日报：每个自然日 09:00 触发一次。
		{"daily fires crossing 09:00", PeriodDaily, at(2026, 9, 4, 8, 59), at(2026, 9, 4, 9, 0), true}, // Friday
		{"daily consumed for the rest of the day", PeriodDaily, at(2026, 9, 4, 9, 0), at(2026, 9, 4, 23, 0), false},
		{"daily silent before 09:00", PeriodDaily, at(2026, 9, 4, 23, 0), at(2026, 9, 5, 2, 0), false},
		{"daily fires again next natural day", PeriodDaily, at(2026, 9, 4, 12, 0), at(2026, 9, 5, 9, 0), true},
		{"daily backfills a stalled ticker", PeriodDaily, at(2026, 9, 4, 8, 0), at(2026, 9, 4, 10, 0), true},
		// 周报：仅周一 09:00。
		{"weekly fires Monday crossing 09:00", PeriodWeekly, at(2026, 9, 7, 8, 59), at(2026, 9, 7, 9, 0), true}, // Monday
		{"weekly silent on Tuesday", PeriodWeekly, at(2026, 9, 1, 11, 59), at(2026, 9, 1, 12, 0), false},
		{"weekly silent later the same week", PeriodWeekly, at(2026, 9, 7, 9, 5), at(2026, 9, 13, 21, 0), false}, // Mon → Sunday
		{"weekly silent Sunday after Monday fired", PeriodWeekly, at(2026, 9, 12, 20, 0), at(2026, 9, 13, 21, 0), false},
		{"weekly fires on week rollover", PeriodWeekly, at(2026, 9, 13, 20, 0), at(2026, 9, 14, 9, 0), true},
		{"weekly backfills a stall across Monday", PeriodWeekly, at(2026, 9, 6, 20, 0), at(2026, 9, 8, 10, 0), true},
		{"weekly consumed after backfill", PeriodWeekly, at(2026, 9, 8, 10, 0), at(2026, 9, 9, 10, 0), false},
		// 月报：仅 1 号 09:00，月末跨月必须触发。
		{"monthly fires on the 1st crossing 09:00", PeriodMonthly, at(2026, 10, 1, 8, 59), at(2026, 10, 1, 9, 0), true},
		{"monthly silent on the 2nd", PeriodMonthly, at(2026, 9, 2, 8, 59), at(2026, 9, 2, 9, 0), false},
		{"monthly silent at month end", PeriodMonthly, at(2026, 1, 30, 9, 0), at(2026, 1, 31, 9, 0), false},
		{"monthly fires across month end", PeriodMonthly, at(2026, 1, 31, 23, 0), at(2026, 2, 1, 9, 1), true},
		{"monthly consumed after firing", PeriodMonthly, at(2026, 2, 1, 9, 5), at(2026, 2, 28, 9, 0), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Due(test.periodType, location, test.previous, test.now); got != test.want {
				t.Fatalf("Due(%s, prev=%s, now=%s) = %v, want %v",
					test.periodType, test.previous, test.now, got, test.want)
			}
		})
	}
	// 未知周期恒不触发；nil location 回退到业务时区（UTC+8）判定。
	if Due(PeriodType("bogus"), location, at(2026, 9, 4, 8, 0), at(2026, 9, 4, 10, 0)) {
		t.Fatal("unknown period type must never fire")
	}
	utcPrevious := time.Date(2026, 9, 4, 0, 59, 0, 0, time.UTC) // 08:59 in UTC+8
	utcNow := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC)       // 09:00 in UTC+8
	if !Due(PeriodDaily, nil, utcPrevious, utcNow) {
		t.Fatal("nil location must fall back to the business timezone")
	}
}

// TestTriggerAnchorIsUniquePerOccurrence pins the per-granularity anchor used
// to suppress duplicate generations inside one window.
func TestTriggerAnchorIsUniquePerOccurrence(t *testing.T) {
	location := ScheduleLocation()
	at := func(year int, month time.Month, day, hour int) time.Time {
		return time.Date(year, month, day, hour, 0, 0, 0, location)
	}
	tests := []struct {
		name       string
		periodType PeriodType
		moment     time.Time
		want       string
	}{
		{"daily anchors the containing day", PeriodDaily, at(2026, 9, 8, 9), "2026-09-08"},
		{"daily anchors the day before 09:00 too", PeriodDaily, at(2026, 9, 8, 7), "2026-09-08"},
		{"weekly anchors the ISO Monday", PeriodWeekly, at(2026, 9, 9, 12), "2026-09-07"},   // Wednesday
		{"weekly Sunday keeps its Monday", PeriodWeekly, at(2026, 9, 13, 21), "2026-09-07"}, // Sunday
		{"monthly anchors the 1st", PeriodMonthly, at(2026, 9, 20, 9), "2026-09-01"},
		{"monthly February anchor", PeriodMonthly, at(2026, 2, 15, 9), "2026-02-01"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			anchor, ok := TriggerAnchor(test.periodType, location, test.moment)
			if !ok || anchor != test.want {
				t.Fatalf("TriggerAnchor(%s, %s) = (%q, %v), want (%q, true)",
					test.periodType, test.moment, anchor, ok, test.want)
			}
		})
	}
	if _, ok := TriggerAnchor(PeriodType("bogus"), location, at(2026, 9, 8, 9)); ok {
		t.Fatal("unknown period type must not yield an anchor")
	}
}

func TestInstructionsDifferForMemberAndGroup(t *testing.T) {
	period, err := ResolvePeriod(PeriodWeekly,
		time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), time.UTC,
		time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if reportInstructionsFor(Report{Period: period}) == reportInstructionsFor(Report{Target: ReportTarget{Kind: TargetGroup}, Period: period}) {
		t.Fatal("group and member reporting skills must differ")
	}
	// 周报使用周度措辞，且仍以统计区间锚定。
	memberText := reportInstructionsFor(Report{Period: period})
	if !strings.Contains(memberText, "本周概览") || !strings.Contains(memberText, "下周计划") {
		t.Fatalf("weekly member instructions must use weekly wording: %s", memberText)
	}
	if !strings.Contains(memberText, "统计区间 2026-08-31 至 2026-09-06") {
		t.Fatalf("member instructions must carry the concrete range: %s", memberText)
	}
	// 日报与月报必须换成各自的周期措辞，不能再复用周报口径。
	dailyPeriod, err := ResolvePeriod(PeriodDaily,
		time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), time.UTC,
		time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	dailyText := reportInstructionsFor(Report{Period: dailyPeriod})
	if !strings.Contains(dailyText, "今日完成") || !strings.Contains(dailyText, "明日计划") {
		t.Fatalf("daily member instructions must use daily wording: %s", dailyText)
	}
	if strings.Contains(dailyText, "本周") {
		t.Fatalf("daily member instructions must not reuse weekly wording: %s", dailyText)
	}
	monthlyPeriod, err := ResolvePeriod(PeriodMonthly,
		time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), time.UTC,
		time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	monthlyText := reportInstructionsFor(Report{Period: monthlyPeriod})
	if !strings.Contains(monthlyText, "本月概览") || !strings.Contains(monthlyText, "下月计划") {
		t.Fatalf("monthly member instructions must use monthly wording: %s", monthlyText)
	}
	groupText := reportInstructionsFor(Report{Target: ReportTarget{Kind: TargetGroup}, Period: period})
	if !strings.Contains(groupText, "下周重点") {
		t.Fatalf("weekly group instructions must use weekly focus wording: %s", groupText)
	}
	payload := reportRunPayload(ai.CapabilityReportWeekly, 7, Report{Target: ReportTarget{Kind: TargetGroup, ID: 21, Name: "研发组"}, Period: period})
	instructions, ok := payload["instructions"].(string)
	if !ok || instructions != groupReportInstructions(period, focusNone) {
		t.Fatalf("group payload must embed the group skill instructions")
	}
}
