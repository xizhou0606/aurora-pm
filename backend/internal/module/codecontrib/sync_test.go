package codecontrib_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
	codecontribgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib/gorm"
)

// The sync tests run the real sqlite repositories against a scripted
// collector, exercising the full loop: collection → classification →
// PR/commit reconciliation → binding resolution → daily aggregates.
type syncFixture struct {
	service   *codecontrib.SyncService
	admin     *codecontrib.AdminService
	facts     codecontrib.FactStore
	bindings  codecontrib.BindingStore
	repoID    uint
	collector *scriptedCollector
	clock     time.Time
}

func (fixture *syncFixture) advance(d time.Duration) {
	fixture.clock = fixture.clock.Add(d)
}

type scriptedCollector struct {
	commits    []codecontrib.CommitFact
	requests   []codecontrib.PullRequestFact
	commitErr  error
	requestErr error
	// lastCommitsWindow records the collection window the service requested,
	// so tests can assert window semantics（首次全量零起点 / 定时自然窗边界）.
	lastCommitsWindow codecontrib.Window
}

func (collector *scriptedCollector) ListCommits(_ context.Context, _ codecontrib.Credentials, _ codecontrib.RepoRef, window codecontrib.Window, _ string) (codecontrib.CommitPage, error) {
	if collector.commitErr != nil {
		return codecontrib.CommitPage{}, collector.commitErr
	}
	collector.lastCommitsWindow = window
	return codecontrib.CommitPage{Facts: collector.commits}, nil
}

func (collector *scriptedCollector) ListMergedPullRequests(context.Context, codecontrib.Credentials, codecontrib.RepoRef, codecontrib.Window, string) (codecontrib.PRPage, error) {
	if collector.requestErr != nil {
		return codecontrib.PRPage{}, collector.requestErr
	}
	return codecontrib.PRPage{Facts: collector.requests}, nil
}

func (collector *scriptedCollector) VerifyConnection(context.Context, codecontrib.Credentials) (codecontrib.Account, error) {
	return codecontrib.Account{Login: "pm-bot"}, nil
}

func (collector *scriptedCollector) resetWindow() {
	collector.lastCommitsWindow = codecontrib.Window{}
}

type fakeMembers struct{ members []codecontrib.MemberCandidate }

func (source fakeMembers) FindActiveMemberCandidates(context.Context, uint) ([]codecontrib.MemberCandidate, error) {
	return source.members, nil
}

// cstLocation 是测试用的工作区时区；fixture 时钟锚在周三 2026-09-09 09:00
// （非周一、非月初，日边界就是"昨天"）。
var cstLocation = time.FixedZone("CST", 8*3600)

var fixtureClockReference = time.Date(2026, 9, 9, 9, 0, 0, 0, cstLocation)

func newSyncFixture(t *testing.T) *syncFixture {
	t.Helper()
	fixture := &syncFixture{clock: fixtureClockReference}
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(codecontribgorm.MigrationRecords()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	adminRepository := codecontribgorm.NewAdminRepository(db)
	factsRepository := codecontribgorm.NewFactsRepository(db)
	collector := &scriptedCollector{}
	members := fakeMembers{members: []codecontrib.MemberCandidate{
		{FactID: 5, Username: "alice", Email: "alice@example.com"},
		{FactID: 6, Username: "bob", Email: "bob@example.com"},
	}}
	syncService, err := codecontrib.NewSyncService(codecontrib.SyncDependencies{
		Connections: adminRepository, Repositories: adminRepository,
		Settings: adminRepository, Bindings: factsRepository, Facts: factsRepository,
		Collectors: map[codecontrib.Provider]codecontrib.Collector{codecontrib.ProviderGitLab: collector},
		Members:    members,
		Now:        func() time.Time { return fixture.clock },
	})
	if err != nil {
		t.Fatalf("new sync service: %v", err)
	}
	adminService, err := codecontrib.NewAdminService(codecontrib.AdminDependencies{
		Connections: adminRepository, Repositories: adminRepository,
		Settings: adminRepository, Bindings: factsRepository,
		Collectors: map[codecontrib.Provider]codecontrib.Collector{codecontrib.ProviderGitLab: collector},
		Members:    members, Reaggregator: syncService,
	})
	if err != nil {
		t.Fatalf("new admin service: %v", err)
	}
	if _, err := adminService.CreateConnection(context.Background(), 1, 9, codecontrib.ConnectionInput{
		Name: "gl", Provider: codecontrib.ProviderGitLab, Token: "glpat-x", Enabled: boolPtr(true),
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	repository, err := adminService.CreateRepository(context.Background(), 1, 9, codecontrib.RepositoryInput{
		ConnectionID: 1, ExternalID: "42", Path: "ops/pm", DefaultBranch: "main", Enabled: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	fixture.service = syncService
	fixture.admin = adminService
	fixture.facts = factsRepository
	fixture.bindings = factsRepository
	fixture.repoID = repository.ID
	fixture.collector = collector
	return fixture
}

func (fixture *syncFixture) seedSyncRun(t *testing.T, fullSyncedAt, lastSuccessAt time.Time) {
	t.Helper()
	if err := fixture.facts.SaveSyncRun(context.Background(), codecontrib.RepoSyncState{
		RepoID: fixture.repoID, WorkspaceID: 1,
		LastSyncedAt:  &lastSuccessAt,
		LastSuccessAt: &lastSuccessAt,
		FullSyncedAt:  &fullSyncedAt,
	}); err != nil {
		t.Fatalf("seed sync run: %v", err)
	}
}

func boolPtr(value bool) *bool { return &value }

func uintPtr(value uint) *uint { return &value }

// fixtureTime builds dates relative to the fixed reference so sampled facts
// always land on deterministic workspace-timezone days.
func fixtureTime(dayOffset int, hour int) time.Time {
	local := fixtureClockReference.In(cstLocation).AddDate(0, 0, -dayOffset)
	return time.Date(local.Year(), local.Month(), local.Day(), hour, 0, 0, 0, cstLocation)
}

// cstDay builds the UTC instant of a workspace-timezone day start.
func cstDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, cstLocation).UTC()
}

func TestSyncWorkspaceBindsAggregatesAndExcludesBots(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.collector.commits = []codecontrib.CommitFact{
		{ // 直接 push，正常成员
			SHA: "c1", AuthorEmail: "alice@example.com", AuthorName: "Alice",
			AuthorAt: fixtureTime(1, 10), CommittedAt: fixtureTime(1, 10),
			Files: []codecontrib.FileChange{{Path: "main.go", Additions: 10, Deletions: 2}},
		},
		{ // 属于 PR 7 的提交：行数按 PR 归属，不重复计
			SHA: "c2", AuthorEmail: "alice@example.com",
			AuthorAt: fixtureTime(1, 11), CommittedAt: fixtureTime(1, 11),
			Files: []codecontrib.FileChange{{Path: "main.go", Additions: 100, Deletions: 0}},
		},
		{ // 机器人
			SHA: "c4", AuthorEmail: "bot@example.com", AuthorLogin: "dependabot[bot]",
			AuthorAt: fixtureTime(2, 10), CommittedAt: fixtureTime(2, 10),
			Files: []codecontrib.FileChange{{Path: "go.sum", Additions: 500}},
		},
	}
	// PR 作者只有 provider login，管理员预置 login 别名映射到 alice。
	if err := fixture.bindings.UpsertBinding(context.Background(), codecontrib.BindingRow{
		WorkspaceID: 1, Provider: codecontrib.ProviderGitLab, KeyKind: "login",
		AuthorKey: "alice", Status: codecontrib.BindingMatched,
		MatchedVia: codecontrib.MatchAdminConfirmed, UserID: uintPtr(5),
	}); err != nil {
		t.Fatalf("seed login binding: %v", err)
	}
	mergedAt := fixtureTime(2, 12)
	fixture.collector.requests = []codecontrib.PullRequestFact{{
		Number: 7, Title: "Feature", AuthorLogin: "alice",
		CreatedAt: fixtureTime(1, 12), MergedAt: &mergedAt, MergeSHA: "c3",
		CommitSHAs: []string{"c2"}, Additions: 50, Deletions: 5,
		Files: []codecontrib.FileChange{{Path: "feat.go", Additions: 50, Deletions: 5}},
		Reviews: []codecontrib.ReviewFact{{
			ReviewerLogin: "bob", State: codecontrib.ReviewApproved, SubmittedAt: fixtureTime(2, 9),
		}},
	}}

	result, err := fixture.service.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(result.RepoOutcomes) != 1 || result.RepoOutcomes[0].Error != "" {
		t.Fatalf("outcomes wrong: %+v", result.RepoOutcomes)
	}
	// 首轮即接入全量：窗口起点为零值（全部 refs、全部历史）。
	if !fixture.collector.lastCommitsWindow.Since.IsZero() {
		t.Fatalf("first pass must pull full history, window since = %v", fixture.collector.lastCommitsWindow.Since)
	}

	rows, err := fixture.facts.ListDailyContributions(context.Background(), 1)
	if err != nil {
		t.Fatalf("list daily: %v", err)
	}
	// 跨日求和：直接 push 的提交与 PR 合入分属两个自然日。
	totals := map[uint]*codecontrib.DailyContributionRow{}
	for index := range rows {
		row := &rows[index]
		total, exists := totals[row.UserID]
		if !exists {
			total = &codecontrib.DailyContributionRow{UserID: row.UserID}
			totals[row.UserID] = total
		}
		total.Commits += row.Commits
		total.Additions += row.Additions
		total.Deletions += row.Deletions
		total.PRsMerged += row.PRsMerged
		total.ReviewsCompleted += row.ReviewsCompleted
	}
	alice, ok := totals[5]
	if !ok {
		t.Fatalf("alice bucket missing: %+v", rows)
	}
	// 提交行数只有直接 push 的 c1；PR 交付行数来自请求级 diff。
	if alice.Commits != 1 || alice.Additions != 60 || alice.Deletions != 7 || alice.PRsMerged != 1 {
		t.Fatalf("alice aggregate wrong: %+v (rows %+v)", alice, rows)
	}
	// bob 只有建议未确认 → 进入未归属桶，绝不并进任何成员。
	unattributed, ok := totals[0]
	if !ok || unattributed.ReviewsCompleted != 1 {
		t.Fatalf("unattributed review bucket wrong: %+v", rows)
	}
	for _, row := range rows {
		if row.UserID == 0 && row.AuthorKey != "bob" {
			t.Fatalf("unexpected unattributed bucket: %+v", row)
		}
	}
	if _, ok := totals[6]; ok {
		t.Fatal("suggested identity must not enter a person bucket")
	}

	bindings, err := fixture.bindings.ListBindings(context.Background(), 1, nil, 0)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	statuses := map[string]codecontrib.BindingStatus{}
	for _, row := range bindings {
		statuses[row.AuthorKey] = row.Status
	}
	if statuses["alice@example.com"] != codecontrib.BindingMatched || statuses["bob"] != codecontrib.BindingSuggested {
		t.Fatalf("binding statuses wrong: %+v", statuses)
	}
	// 绑定行必须带上 Git 作者名，确认队列才能显示人名。
	for _, row := range bindings {
		if row.AuthorKey == "alice@example.com" && row.DisplayName != "Alice" {
			t.Fatalf("email binding display name = %q, want Alice", row.DisplayName)
		}
	}
	if statuses["bot@example.com"] != codecontrib.BindingBot && statuses["dependabot[bot]"] != codecontrib.BindingBot {
		t.Fatalf("bot identity not flagged: %+v", statuses)
	}

	state, exists, err := fixture.facts.GetSyncRun(context.Background(), fixture.repoID)
	if err != nil || !exists || state.LastError != "" || state.ConsecutiveFailures != 0 {
		t.Fatalf("sync state wrong: %+v exists=%v err=%v", state, exists, err)
	}
	// 全量完成必须落标记。
	if state.FullSyncedAt == nil {
		t.Fatalf("first successful pass must record the full pull: %+v", state)
	}
}

func TestSyncFailureIsRecordedNotRenderedAsZero(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.collector.commitErr = errors.New("api rate limited")

	result, err := fixture.service.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync must swallow per-repo errors: %v", err)
	}
	if result.RepoOutcomes[0].Error == "" {
		t.Fatal("outcome must carry the failure")
	}
	state, exists, err := fixture.facts.GetSyncRun(context.Background(), fixture.repoID)
	if err != nil || !exists || state.LastError == "" || state.ConsecutiveFailures != 1 {
		t.Fatalf("failure state wrong: %+v exists=%v err=%v", state, exists, err)
	}
	rows, err := fixture.facts.ListDailyContributions(context.Background(), 1)
	if err != nil || len(rows) != 0 {
		t.Fatalf("failed sync must not write aggregates: %d %v", len(rows), err)
	}
	// 全量尚未成功，下轮必须仍然先补全量。
	if state.FullSyncedAt != nil {
		t.Fatalf("failed pass must not claim the full pull: %+v", state)
	}
}

func TestBindingConfirmationReachesAggregates(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.collector.commits = []codecontrib.CommitFact{{
		SHA: "c1", AuthorEmail: "ghost@example.com", AuthorName: "Ghost",
		AuthorAt: fixtureTime(1, 10), CommittedAt: fixtureTime(1, 10),
		Files: []codecontrib.FileChange{{Path: "main.go", Additions: 7}},
	}}
	if _, syncErr := fixture.service.SyncWorkspace(context.Background(), 1); syncErr != nil {
		t.Fatalf("sync: %v", syncErr)
	}
	bindings, err := fixture.bindings.ListBindings(context.Background(), 1,
		[]codecontrib.BindingStatus{codecontrib.BindingUnmatched}, 10)
	if err != nil || len(bindings) != 1 {
		all, _ := fixture.bindings.ListBindings(context.Background(), 1, nil, 0)
		t.Fatalf("unmatched queue: %d %v; all rows: %+v", len(bindings), err, all)
	}
	// 非工作区成员不可确认（fail closed）。
	if _, err := fixture.admin.ConfirmBinding(context.Background(), 1, bindings[0].ID, 999, 9); err == nil {
		t.Fatal("confirming a non-member must fail")
	}
	// 确认到 alice（fact 5）后重聚合应把未归属桶换成 alice。
	if _, err := fixture.admin.ConfirmBinding(context.Background(), 1, bindings[0].ID, 5, 9); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	rows, err := fixture.facts.ListDailyContributions(context.Background(), 1)
	if err != nil {
		t.Fatalf("list daily: %v", err)
	}
	if len(rows) != 1 || rows[0].UserID != 5 || rows[0].Additions != 7 {
		t.Fatalf("reaggregated rows wrong: %+v", rows)
	}
}

// 严格按日期统计：绑定确认触发的重算必须覆盖全部已存历史，20 天前的
// 提交在确认绑定后也要改归属。
func TestReaggregateReachesFullStoredHistory(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.collector.commits = []codecontrib.CommitFact{{
		SHA: "old", AuthorEmail: "ghost@example.com", AuthorName: "Ghost",
		AuthorAt: fixtureTime(20, 10), CommittedAt: fixtureTime(20, 10),
		Files: []codecontrib.FileChange{{Path: "main.go", Additions: 3}},
	}}
	if _, err := fixture.service.SyncWorkspace(context.Background(), 1); err != nil {
		t.Fatalf("sync: %v", err)
	}
	bindings, err := fixture.bindings.ListBindings(context.Background(), 1,
		[]codecontrib.BindingStatus{codecontrib.BindingUnmatched}, 10)
	if err != nil || len(bindings) != 1 {
		t.Fatalf("unmatched queue: %d %v", len(bindings), err)
	}
	if _, err := fixture.admin.ConfirmBinding(context.Background(), 1, bindings[0].ID, 5, 9); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	rows, err := fixture.facts.ListDailyContributions(context.Background(), 1)
	if err != nil {
		t.Fatalf("list daily: %v", err)
	}
	if len(rows) != 1 || rows[0].UserID != 5 || rows[0].Additions != 3 {
		t.Fatalf("full-history reaggregate wrong: %+v", rows)
	}
}

func TestSyncDueWorkspacesFullPullsOnFirstPass(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.collector.commits = []codecontrib.CommitFact{{
		SHA: "c1", AuthorEmail: "alice@example.com", AuthorName: "Alice",
		AuthorAt: fixtureTime(1, 10), CommittedAt: fixtureTime(1, 10),
		Files: []codecontrib.FileChange{{Path: "main.go", Additions: 10, Deletions: 2}},
	}}

	result, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock)
	if err != nil {
		t.Fatalf("sync due: %v", err)
	}
	if len(result.RepoOutcomes) != 1 || result.RepoOutcomes[0].Error != "" {
		t.Fatalf("outcomes: %+v", result.RepoOutcomes)
	}
	if !fixture.collector.lastCommitsWindow.Since.IsZero() {
		t.Fatalf("first pass must pull full history, window since = %v", fixture.collector.lastCommitsWindow.Since)
	}
	state, exists, err := fixture.facts.GetSyncRun(context.Background(), fixture.repoID)
	if err != nil || !exists || state.FullSyncedAt == nil {
		t.Fatalf("full pull must be recorded: %v %v %+v", exists, err, state)
	}

	// 同一天内重复调用幂等：09:00 已跑过就不再触发。
	fixture.collector.resetWindow()
	fixture.advance(time.Hour)
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("same-day repeat: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.IsZero() {
		t.Fatalf("same-day repeat must not re-run: window = %+v", fixture.collector.lastCommitsWindow)
	}
}

func TestSyncDueWorkspacesDailyWindowAtNine(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.seedSyncRun(t,
		time.Date(2026, 9, 7, 9, 0, 0, 0, cstLocation),
		time.Date(2026, 9, 8, 9, 0, 0, 0, cstLocation))

	// 08:59 未到触发点。
	fixture.clock = time.Date(2026, 9, 9, 8, 59, 0, 0, cstLocation)
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("before trigger: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.IsZero() {
		t.Fatalf("no sync may run before 09:00: window = %+v", fixture.collector.lastCommitsWindow)
	}
	// 09:00 触发：起点 = 昨天 0 点（9/8）。
	fixture.clock = time.Date(2026, 9, 9, 9, 0, 0, 0, cstLocation)
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("at trigger: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.Equal(cstDay(2026, time.September, 8)) {
		t.Fatalf("daily window since = %v, want 2026-09-08 day start", fixture.collector.lastCommitsWindow.Since)
	}
	// 当天 10:00 不再触发（今天已成功同步过）。
	fixture.collector.resetWindow()
	fixture.clock = time.Date(2026, 9, 9, 10, 0, 0, 0, cstLocation)
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("same day repeat: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.IsZero() {
		t.Fatalf("same-day repeat must not re-run: window = %+v", fixture.collector.lastCommitsWindow)
	}
	// 次日 09:00 再次触发，起点 = 昨天（9/9）0 点。
	fixture.clock = time.Date(2026, 9, 10, 9, 0, 0, 0, cstLocation)
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("next day trigger: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.Equal(cstDay(2026, time.September, 9)) {
		t.Fatalf("next-day window since = %v, want 2026-09-09 day start", fixture.collector.lastCommitsWindow.Since)
	}
}

func TestSyncDueWorkspacesMondayCoversLastWeek(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.seedSyncRun(t,
		time.Date(2026, 9, 7, 9, 0, 0, 0, cstLocation),
		time.Date(2026, 9, 11, 9, 0, 0, 0, cstLocation))
	fixture.clock = time.Date(2026, 9, 14, 9, 30, 0, 0, cstLocation) // 周一
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("monday sync: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.Equal(cstDay(2026, time.September, 7)) {
		t.Fatalf("monday window since = %v, want last Monday day start", fixture.collector.lastCommitsWindow.Since)
	}
}

func TestSyncDueWorkspacesFirstOfMonthCoversLastMonth(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.seedSyncRun(t,
		time.Date(2026, 9, 1, 9, 0, 0, 0, cstLocation),
		time.Date(2026, 9, 30, 9, 0, 0, 0, cstLocation))
	fixture.clock = time.Date(2026, 10, 1, 9, 30, 0, 0, cstLocation) // 月初 1 号
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("first-of-month sync: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.Equal(cstDay(2026, time.September, 1)) {
		t.Fatalf("monthly window since = %v, want 2026-09-01 day start", fixture.collector.lastCommitsWindow.Since)
	}
}

// 故障自愈：几天没同步成功后，窗口起点自动退到上次成功采集日，
// 断档期间的数据在下一次触发时自动补齐。
func TestSyncDueWorkspacesHealsFromLastSuccess(t *testing.T) {
	fixture := newSyncFixture(t)
	fixture.seedSyncRun(t,
		time.Date(2026, 9, 1, 9, 0, 0, 0, cstLocation),
		time.Date(2026, 9, 2, 9, 0, 0, 0, cstLocation))
	fixture.clock = time.Date(2026, 9, 9, 9, 30, 0, 0, cstLocation)
	if _, err := fixture.service.SyncDueWorkspaces(context.Background(), fixture.clock); err != nil {
		t.Fatalf("healing sync: %v", err)
	}
	if !fixture.collector.lastCommitsWindow.Since.Equal(cstDay(2026, time.September, 2)) {
		t.Fatalf("healing window since = %v, want last-success day start", fixture.collector.lastCommitsWindow.Since)
	}
}
