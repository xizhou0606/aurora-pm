package report

import (
	"testing"
	"time"

	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func scoreByKey(scores ReportScores) map[string]ScoreDimension {
	byKey := make(map[string]ScoreDimension, len(scores.Dimensions))
	for _, dimension := range scores.Dimensions {
		byKey[dimension.Key] = dimension
	}
	return byKey
}

func TestComputeReportScoresEmptyReportCollapsesToZero(t *testing.T) {
	scores := ComputeReportScores(TaskMetrics{}, BugMetrics{}, Period{Type: PeriodWeekly}, nil)
	if scores.Overall != 0 {
		t.Fatalf("empty report must score 0, got %d", scores.Overall)
	}
	if len(scores.Dimensions) != 5 {
		t.Fatalf("expected 5 dimensions, got %d", len(scores.Dimensions))
	}
	for _, dimension := range scores.Dimensions {
		if dimension.Score != 0 {
			t.Fatalf("empty report dimension %s must score 0, got %d", dimension.Key, dimension.Score)
		}
	}
}

func TestComputeReportScoresHealthyDelivery(t *testing.T) {
	tasks := TaskMetrics{
		Completed: 4, CompletedOnTime: 4,
		Active: 1,
	}
	bugs := BugMetrics{
		Resolved: 2, ResolvedOnTime: 1,
		Open: 1,
	}
	// 交付完成 6/8=75；按期 5/6=83；质量 2/3=66；逾期控制 100；交付量 6/10 基线=60。
	scores := ComputeReportScores(tasks, bugs, Period{Type: PeriodWeekly}, nil)
	byKey := scoreByKey(scores)
	if byKey["delivery"].Score != 75 {
		t.Fatalf("delivery score = %d, want 75", byKey["delivery"].Score)
	}
	if byKey["punctuality"].Score != 83 {
		t.Fatalf("punctuality score = %d, want 83", byKey["punctuality"].Score)
	}
	if byKey["quality"].Score != 66 {
		t.Fatalf("quality score = %d, want 66", byKey["quality"].Score)
	}
	if byKey["overdue_control"].Score != 100 {
		t.Fatalf("overdue control score = %d, want 100", byKey["overdue_control"].Score)
	}
	if byKey["volume"].Score != 60 {
		t.Fatalf("volume score = %d, want 60", byKey["volume"].Score)
	}
	if byKey["delivery"].Basis == "" {
		t.Fatal("score basis must explain the numbers")
	}
}

func TestComputeReportScoresOverduePenaltyAndVolumeCap(t *testing.T) {
	tasks := TaskMetrics{
		Completed: 30, CompletedOnTime: 30,
		Active: 2, Overdue: 3,
	}
	// 交付 30/32=93；逾期控制 100-3/32=90；交付量 30/25 基线封顶 100。
	scores := ComputeReportScores(tasks, BugMetrics{}, Period{Type: PeriodMonthly}, nil)
	byKey := scoreByKey(scores)
	if byKey["delivery"].Score != 93 {
		t.Fatalf("delivery score = %d, want 93", byKey["delivery"].Score)
	}
	if byKey["overdue_control"].Score != 91 {
		t.Fatalf("overdue control score = %d, want 91", byKey["overdue_control"].Score)
	}
	if byKey["volume"].Score != 100 {
		t.Fatalf("volume score = %d, want capped 100", byKey["volume"].Score)
	}
}

func TestComputeReportScoresNoBugsIsPerfectQuality(t *testing.T) {
	tasks := TaskMetrics{Completed: 2, CompletedOnTime: 2}
	scores := ComputeReportScores(tasks, BugMetrics{}, Period{Type: PeriodDaily}, nil)
	byKey := scoreByKey(scores)
	if byKey["quality"].Score != 100 {
		t.Fatalf("no-bugs quality score = %d, want 100", byKey["quality"].Score)
	}
	if byKey["overdue_control"].Score != 100 {
		t.Fatalf("no-overdue control score = %d, want 100", byKey["overdue_control"].Score)
	}
}

func TestAggregateCountsOnTimeBeforeTruncation(t *testing.T) {
	// 25 completed items: the first 24 on time, the last one late. The
	// displayed list truncates to 20, but the on-time count must cover all 25.
	period := Period{
		Type:  PeriodWeekly,
		Start: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
	}
	snapshots := make([]task.Snapshot, 0, 25)
	for index := 0; index < 25; index++ {
		completedAt := time.Date(2026, 9, 1+index/24, 8, 0, 0, 0, time.UTC)
		due := completedAt.AddDate(0, 0, 1)
		if index == 24 {
			// The 25th item completes a day after its due date.
			due = completedAt.AddDate(0, 0, -1)
		}
		snapshots = append(snapshots, task.Snapshot{
			ID: uint(index + 1), Title: "task", Status: "completed",
			CompletedAt: &completedAt, DueAt: &due,
			CreatedAt: period.Start,
		})
	}
	metrics := aggregateTasks(snapshots, period, period.End)
	if metrics.Completed != 25 {
		t.Fatalf("completed = %d, want 25", metrics.Completed)
	}
	if metrics.CompletedOnTime != 24 {
		t.Fatalf("completed on time = %d, want 24", metrics.CompletedOnTime)
	}
	if len(metrics.CompletedItems) != reportedItemsLimit {
		t.Fatalf("displayed items = %d, want %d", len(metrics.CompletedItems), reportedItemsLimit)
	}
}

func TestComputeReportScoresCodeDimensionBlendsLinesAndCollab(t *testing.T) {
	tasks := TaskMetrics{Completed: 2, CompletedOnTime: 2}
	// 周基线：行数 1500，协作 8。行数 300+100=400 → 26；合并 PR 1 + 评审 3 = 4 → 50；
	// 代码贡献 = (26+50)/2 = 38；代码质量独立成轴，无命中 → 100；维度数 5 → 7。
	code := &CodeActivityReport{Additions: 300, Deletions: 100, PRsMerged: 1, ReviewsCompleted: 3}
	scores := ComputeReportScores(tasks, BugMetrics{}, Period{Type: PeriodWeekly}, code)
	byKey := scoreByKey(scores)
	if len(scores.Dimensions) != 7 {
		t.Fatalf("wired code report must add code + code_quality dimensions, got %d", len(scores.Dimensions))
	}
	if byKey["code"].Score != 38 {
		t.Fatalf("code score = %d, want 38", byKey["code"].Score)
	}
	if byKey["code_quality"].Score != 100 {
		t.Fatalf("code quality = %d, want 100", byKey["code_quality"].Score)
	}
}

func TestComputeReportScoresCodeQualityHygienePenalty(t *testing.T) {
	// 新增 900 行、调试残留 2 处、TODO 2 处 → 扣 (2*6+2*3)*1000/900=20 分 → 80。
	code := &CodeActivityReport{Additions: 900, Deletions: 100, DebugHits: 2, TodoHits: 2}
	scores := ComputeReportScores(TaskMetrics{}, BugMetrics{}, Period{Type: PeriodWeekly}, code)
	byKey := scoreByKey(scores)
	if byKey["code_quality"].Score != 80 {
		t.Fatalf("code quality = %d, want 80", byKey["code_quality"].Score)
	}
	if byKey["code_quality"].Basis == "" {
		t.Fatal("code quality basis must explain the deduction")
	}
}

func TestComputeReportScoresCodeZeroActivityAndLineCap(t *testing.T) {
	zero := &CodeActivityReport{}
	scores := ComputeReportScores(TaskMetrics{}, BugMetrics{}, Period{Type: PeriodWeekly}, zero)
	byKey := scoreByKey(scores)
	if byKey["code"].Score != 0 {
		t.Fatalf("zero-activity code score = %d, want 0", byKey["code"].Score)
	}
	if byKey["code"].Basis != "本期无归属的代码活动" {
		t.Fatalf("zero-activity basis = %q", byKey["code"].Basis)
	}
	if byKey["code_quality"].Score != 0 || byKey["code_quality"].Basis != "本期无归属的代码活动" {
		t.Fatalf("zero-activity code quality wrong: %+v", byKey["code_quality"])
	}
	capped := &CodeActivityReport{Additions: 5000, Deletions: 5000, PRsMerged: 20, ReviewsCompleted: 20}
	scores = ComputeReportScores(TaskMetrics{}, BugMetrics{}, Period{Type: PeriodWeekly}, capped)
	byKey = scoreByKey(scores)
	// 行数 10000/1500 与协作 40/8 都封顶 100 → 代码维度 100。
	if byKey["code"].Score != 100 {
		t.Fatalf("capped code score = %d, want 100", byKey["code"].Score)
	}
}
