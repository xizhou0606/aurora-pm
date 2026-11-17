package codecontrib

import (
	"testing"
	"time"
)

func mergedAt(year int, month time.Month, day int) *time.Time {
	at := time.Date(year, month, day, 8, 0, 0, 0, time.UTC)
	return &at
}

// 构成拆分与评审覆盖：四类行数合计恒等于总数，lockfile/binary 不进任何
// 桶；带评审合入只数有 approval 的请求，自合请求计入 prs 但不计 reviewed。
func TestBuildDailyRowsSplitsLineClassesAndReviewCoverage(t *testing.T) {
	commits := []CommitFact{{
		SHA: "a", AuthorEmail: "dev@x.com", AuthorName: "开",
		CommittedAt: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
		Files: []FileChange{
			{Path: "main.go", Additions: 100, Class: FileClassSource},
			{Path: "main_test.go", Additions: 40, Class: FileClassTest},
			{Path: "README.md", Additions: 12, Class: FileClassDoc},
			{Path: "chart.yaml", Additions: 8, Class: FileClassConfig},
			{Path: "pnpm-lock.yaml", Additions: 500, Class: FileClassLockfile},
			{Path: "logo.png", Additions: 0, Binary: true, Class: FileClassBinary},
		},
	}}
	requests := []PullRequestFact{
		{
			Number: 7, AuthorLogin: "dev", MergedAt: mergedAt(2026, 9, 3),
			Files:   []FileChange{{Path: "app.tsx", Additions: 60, Class: FileClassSource}},
			Reviews: []ReviewFact{{ReviewerLogin: "peer", State: ReviewApproved, SubmittedAt: *mergedAt(2026, 9, 3)}},
		},
		{
			Number: 8, AuthorLogin: "dev", MergedAt: mergedAt(2026, 9, 4),
			Files: []FileChange{{Path: "util.ts", Additions: 20, Class: FileClassSource}},
		},
	}
	resolve := func(AuthorIdentity) Resolution {
		return Resolution{Status: BindingMatched, UserID: 5, MatchedVia: MatchEmailExact}
	}
	aggregation := BuildDailyRows(11, commits, requests, resolve, CollectionConfig{Timezone: "UTC"}, time.Now())

	rows := map[string]DailyContributionRow{}
	for _, row := range aggregation.Rows {
		rows[row.Day] = row
	}
	commitDay := rows["2026-09-02"]
	if commitDay.Commits != 1 || commitDay.Additions != 160 || commitDay.Deletions != 0 {
		t.Fatalf("commit day totals wrong: %+v", commitDay)
	}
	if commitDay.SourceAdditions != 100 || commitDay.TestAdditions != 40 ||
		commitDay.DocAdditions != 12 || commitDay.ConfigAdditions != 8 {
		t.Fatalf("commit day class split wrong: %+v", commitDay)
	}
	reviewedDay := rows["2026-09-03"]
	if reviewedDay.PRsMerged != 1 || reviewedDay.PRsReviewed != 1 || reviewedDay.SourceAdditions != 60 {
		t.Fatalf("reviewed merge day wrong: %+v", reviewedDay)
	}
	selfMergedDay := rows["2026-09-04"]
	if selfMergedDay.PRsMerged != 1 || selfMergedDay.PRsReviewed != 0 || selfMergedDay.Additions != 20 {
		t.Fatalf("self-merge day wrong: %+v", selfMergedDay)
	}
}
