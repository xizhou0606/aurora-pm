package codecontribgorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
	"gorm.io/gorm"
)

func newTestRepository(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(MigrationRecords()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func testConnection(workspaceID uint) codecontrib.Connection {
	return codecontrib.Connection{
		WorkspaceID: workspaceID, Name: "gitlab-main", Provider: codecontrib.ProviderGitLab,
		BaseURL: "https://gitlab.example.com", Token: "glpat-secret-1234", Enabled: true, Version: 1,
	}
}

func TestConnectionTenantIsolation(t *testing.T) {
	db := newTestRepository(t)
	repository := NewAdminRepository(db)
	ctx := context.Background()

	created, err := repository.CreateConnection(ctx, testConnection(1))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if rows, err := repository.ListConnections(ctx, 2); err != nil || len(rows) != 0 {
		t.Fatalf("other workspace must see no connections: rows=%d err=%v", len(rows), err)
	}
	if _, err := repository.GetConnection(ctx, 2, created.ID); err == nil {
		t.Fatal("cross-workspace get must fail")
	}
	if err := repository.DeleteConnection(ctx, 2, created.ID); err == nil {
		t.Fatal("cross-workspace delete must fail")
	}
	if rows, err := repository.ListConnections(ctx, 1); err != nil || len(rows) != 1 {
		t.Fatalf("own workspace must still see the connection: rows=%d err=%v", len(rows), err)
	}
}

func TestConnectionMaskedToken(t *testing.T) {
	connection := testConnection(1)
	if connection.MaskedToken() != "••••1234" {
		t.Fatalf("masked token = %q", connection.MaskedToken())
	}
	connection.Token = ""
	if connection.MaskedToken() != "" {
		t.Fatal("empty token must mask to empty")
	}
}

func TestConnectionUpdateKeepsTokenWhenInputEmpty(t *testing.T) {
	db := newTestRepository(t)
	repository := NewAdminRepository(db)
	ctx := context.Background()

	created, err := repository.CreateConnection(ctx, testConnection(1))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	created.Token = "" // admin updates metadata only
	updated, err := repository.UpdateConnection(ctx, created)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, err := repository.GetConnection(ctx, 1, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if reloaded.Token == "" {
		t.Fatal("token must survive metadata-only updates")
	}
	_ = updated
}

func TestRepositoryConfigRoundTrip(t *testing.T) {
	db := newTestRepository(t)
	repository := NewAdminRepository(db)
	ctx := context.Background()

	projectSet := uint(42)
	config := codecontrib.RepositoryConfig{
		WorkspaceID: 1, ConnectionID: 7, ExternalID: "99", Path: "ops/pm-frontend",
		DefaultBranch: "main",
		FilterRules: codecontrib.FilterRules{
			ExtraVendorPatterns: []string{"assets/"},
			CountAsSource:       []string{"go.sum"},
		},
		ProjectSetID: &projectSet, Enabled: true, Version: 1,
	}
	if _, err := repository.CreateRepository(ctx, config); err != nil {
		t.Fatalf("create: %v", err)
	}
	repositories, err := repository.ListRepositories(ctx, 1)
	if err != nil || len(repositories) != 1 {
		t.Fatalf("list: %d %v", len(repositories), err)
	}
	got := repositories[0]
	if got.Path != config.Path || got.FilterRules.ExtraVendorPatterns[0] != "assets/" ||
		got.FilterRules.CountAsSource[0] != "go.sum" || got.ProjectSetID == nil || *got.ProjectSetID != 42 {
		t.Fatalf("round trip lost data: %+v", got)
	}
}

func TestBindingUpsertNeverDowngradesAdminRow(t *testing.T) {
	db := newTestRepository(t)
	repository := NewFactsRepository(db)
	ctx := context.Background()
	confirmedBy := uint(9)

	adminRow := codecontrib.BindingRow{
		WorkspaceID: 1, Provider: codecontrib.ProviderGitHub, KeyKind: "email",
		AuthorKey: "alice@example.com", Status: codecontrib.BindingMatched,
		MatchedVia: codecontrib.MatchAdminConfirmed, UserID: uintPtr(5),
		ConfirmedBy: confirmedBy,
	}
	if err := repository.UpsertBinding(ctx, adminRow); err != nil {
		t.Fatalf("seed admin row: %v", err)
	}
	autoRow := adminRow
	autoRow.Status = codecontrib.BindingUnmatched
	autoRow.MatchedVia = ""
	autoRow.UserID = nil
	if err := repository.UpsertBinding(ctx, autoRow); err != nil {
		t.Fatalf("replay auto row: %v", err)
	}
	rows, err := repository.ListBindings(ctx, 1, nil, 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list: %d %v", len(rows), err)
	}
	if rows[0].Status != codecontrib.BindingMatched || rows[0].MatchedVia != codecontrib.MatchAdminConfirmed {
		t.Fatalf("admin row downgraded: %+v", rows[0])
	}
}

func TestBindingConfirmAndReject(t *testing.T) {
	db := newTestRepository(t)
	repository := NewFactsRepository(db)
	ctx := context.Background()

	row := codecontrib.BindingRow{
		WorkspaceID: 1, Provider: codecontrib.ProviderGitHub, KeyKind: "login",
		AuthorKey: "alice", DisplayName: "Alice", Status: codecontrib.BindingSuggested,
		SuggestedUserID: uintPtr(5),
	}
	if err := repository.UpsertBinding(ctx, row); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rows, err := repository.ListBindings(ctx, 1, []codecontrib.BindingStatus{codecontrib.BindingSuggested}, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("suggested list: %d %v", len(rows), err)
	}
	confirmed, err := repository.ConfirmBinding(ctx, 1, rows[0].ID, 5, 9, time.Now())
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Status != codecontrib.BindingMatched || confirmed.UserID == nil || *confirmed.UserID != 5 ||
		confirmed.MatchedVia != codecontrib.MatchAdminConfirmed || confirmed.ConfirmedBy != 9 {
		t.Fatalf("confirmed row wrong: %+v", confirmed)
	}
	rejected, err := repository.RejectBinding(ctx, 1, rows[0].ID, 9)
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != codecontrib.BindingUnmatched || rejected.UserID != nil {
		t.Fatalf("rejected row wrong: %+v", rejected)
	}
}

func TestFactsUpsertIdempotentAndTenantScoped(t *testing.T) {
	db := newTestRepository(t)
	repository := NewFactsRepository(db)
	ctx := context.Background()
	committedAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

	fact := codecontrib.CommitFact{
		SHA: "abc", AuthorEmail: "alice@example.com", CommittedAt: committedAt,
		AuthorAt: committedAt, Additions: 10, Deletions: 2,
		Files: []codecontrib.FileChange{{Path: "main.go", Additions: 10, Deletions: 2, Class: codecontrib.FileClassSource}},
	}
	for round := 0; round < 2; round++ {
		if err := repository.UpsertCommits(ctx, 1, 11, []codecontrib.CommitFact{fact}); err != nil {
			t.Fatalf("upsert round %d: %v", round, err)
		}
	}
	commits, err := repository.ListCommitsInWindow(ctx, 1, 11,
		codecontrib.Window{Since: committedAt.Add(-time.Hour), Until: committedAt.Add(time.Hour)})
	if err != nil || len(commits) != 1 {
		t.Fatalf("commits in window: %d %v", len(commits), err)
	}
	if commits[0].Additions != 10 || len(commits[0].Files) != 1 || commits[0].Files[0].Class != codecontrib.FileClassSource {
		t.Fatalf("commit fact round trip lost data: %+v", commits[0])
	}
	if other, err := repository.ListCommitsInWindow(ctx, 2, 11,
		codecontrib.Window{Since: committedAt.Add(-time.Hour), Until: committedAt.Add(time.Hour)}); err != nil || len(other) != 0 {
		t.Fatalf("cross-workspace read must be empty: %d %v", len(other), err)
	}

	mergedAt := committedAt
	request := codecontrib.PullRequestFact{
		Number: 7, Title: "Add feature", AuthorLogin: "alice",
		CreatedAt: committedAt, MergedAt: &mergedAt, MergeSHA: "abc",
		CommitSHAs: []string{"abc"}, Reviews: []codecontrib.ReviewFact{{
			ReviewerLogin: "bob", State: codecontrib.ReviewApproved, SubmittedAt: committedAt,
		}},
	}
	for round := 0; round < 2; round++ {
		if err := repository.UpsertPullRequests(ctx, 1, 11, []codecontrib.PullRequestFact{request}); err != nil {
			t.Fatalf("pr upsert round %d: %v", round, err)
		}
	}
	requests, err := repository.ListMergedPullRequestsInWindow(ctx, 1, 11,
		codecontrib.Window{Since: committedAt.Add(-time.Hour), Until: committedAt.Add(time.Hour)})
	if err != nil || len(requests) != 1 || len(requests[0].Reviews) != 1 || requests[0].MergeSHA != "abc" {
		t.Fatalf("pr round trip: %+v %v", requests, err)
	}
}

func TestReplaceDailyWindowAndSyncState(t *testing.T) {
	db := newTestRepository(t)
	repository := NewFactsRepository(db)
	ctx := context.Background()
	rows := []codecontrib.DailyContributionRow{
		{WorkspaceID: 1, RepoID: 11, UserID: 5, Day: "2026-09-01", Commits: 2, Additions: 30, Deletions: 4},
		{WorkspaceID: 1, RepoID: 11, UserID: 0, AuthorKey: "ghost@example.com", Day: "2026-09-01", Commits: 1},
	}
	if err := repository.ReplaceDailyWindow(ctx, 1, 11, []string{"2026-09-01"}, rows); err != nil {
		t.Fatalf("replace: %v", err)
	}
	replacement := []codecontrib.DailyContributionRow{
		{WorkspaceID: 1, RepoID: 11, UserID: 5, Day: "2026-09-01", Commits: 3, Additions: 50},
	}
	if err := repository.ReplaceDailyWindow(ctx, 1, 11, []string{"2026-09-01"}, replacement); err != nil {
		t.Fatalf("replace again: %v", err)
	}
	all, err := repository.ListDailyContributions(ctx, 1)
	if err != nil || len(all) != 1 || all[0].Commits != 3 || all[0].Additions != 50 {
		t.Fatalf("window not replaced: %+v %v", all, err)
	}
	if other, err := repository.ListDailyContributions(ctx, 2); err != nil || len(other) != 0 {
		t.Fatalf("cross-workspace daily read must be empty: %d %v", len(other), err)
	}

	if err := repository.SaveSyncRun(ctx, codecontrib.RepoSyncState{WorkspaceID: 1, RepoID: 11,
		LastError: "boom", ConsecutiveFailures: 1}); err != nil {
		t.Fatalf("save sync run: %v", err)
	}
	state, exists, err := repository.GetSyncRun(ctx, 11)
	if err != nil || !exists || state.LastError != "boom" || state.ConsecutiveFailures != 1 {
		t.Fatalf("sync run state wrong: %+v exists=%v err=%v", state, exists, err)
	}
}

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	db := newTestRepository(t)
	repository := NewAdminRepository(db)
	ctx := context.Background()

	if _, exists, err := repository.GetSettings(ctx, 1); err != nil || exists {
		t.Fatalf("absent settings must report exists=false: exists=%v err=%v", exists, err)
	}
	config := codecontrib.CollectionConfig{
		Timezone:       "UTC",
		CoauthorPolicy: "primary", BotPatterns: []string{"[bot]", "dependabot"},
		PageCap: 20, Version: 2, UpdatedBy: 7,
	}
	if _, err := repository.SaveSettings(ctx, 1, config); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, exists, err := repository.GetSettings(ctx, 1)
	if err != nil || !exists {
		t.Fatalf("settings missing after save: exists=%v err=%v", exists, err)
	}
	if got.Timezone != "UTC" || len(got.BotPatterns) != 2 || got.PageCap != 20 {
		t.Fatalf("settings round trip lost data: %+v", got)
	}
}

func uintPtr(value uint) *uint { return &value }

// 确认队列要显示 Git 作者名：display_name 为空的旧数据行从提交事实里
// 回填（email 行按作者邮箱、login 行按作者登录名取最近一次的名字），
// 已有名字的行不覆盖。
func TestListBindingsBackfillsDisplayNameFromCommits(t *testing.T) {
	db := newTestRepository(t)
	repository := NewFactsRepository(db)
	ctx := context.Background()

	older := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	facts := []codecontrib.CommitFact{
		{SHA: "c1", AuthorEmail: "dev.one@example.test", AuthorName: "Dev One", AuthorLogin: "devone",
			CommittedAt: older, AuthorAt: older},
		{SHA: "c2", AuthorEmail: "dev.one@example.test", AuthorName: "dev.one", AuthorLogin: "devone",
			CommittedAt: newer, AuthorAt: newer},
		{SHA: "c3", AuthorEmail: "devtwo@example.test", AuthorName: "开发者乙", AuthorLogin: "devtwo",
			CommittedAt: newer, AuthorAt: newer},
	}
	if err := repository.UpsertCommits(ctx, 1, 11, facts); err != nil {
		t.Fatalf("upsert commits: %v", err)
	}
	for _, row := range []codecontrib.BindingRow{
		{WorkspaceID: 1, Provider: codecontrib.ProviderGitLab, KeyKind: "email", AuthorKey: "dev.one@example.test"},
		{WorkspaceID: 1, Provider: codecontrib.ProviderGitLab, KeyKind: "email", AuthorKey: "devtwo@example.test"},
		{WorkspaceID: 1, Provider: codecontrib.ProviderGitLab, KeyKind: "login", AuthorKey: "devtwo"},
		// 其他工作区的提交不得串味。
		{WorkspaceID: 2, Provider: codecontrib.ProviderGitLab, KeyKind: "email", AuthorKey: "dev.one@example.test"},
	} {
		if err := repository.UpsertBinding(ctx, row); err != nil {
			t.Fatalf("upsert binding: %v", err)
		}
	}

	rows, err := repository.ListBindings(ctx, 1, nil, 0)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	names := map[string]string{}
	for _, row := range rows {
		names[row.AuthorKey+"/"+row.KeyKind] = row.DisplayName
	}
	if names["dev.one@example.test/email"] != "dev.one" {
		t.Fatalf("email display name = %q, want the latest commit name dev.one", names["dev.one@example.test/email"])
	}
	if names["devtwo@example.test/email"] != "开发者乙" {
		t.Fatalf("devtwo email display name = %q, want 开发者乙", names["devtwo@example.test/email"])
	}
	if names["devtwo/login"] != "开发者乙" {
		t.Fatalf("devtwo login display name = %q, want 开发者乙", names["devtwo/login"])
	}
	other, err := repository.ListBindings(ctx, 2, nil, 0)
	if err != nil || len(other) != 1 || other[0].DisplayName != "" {
		t.Fatalf("cross-workspace backfill must not leak: %+v %v", other, err)
	}
}
