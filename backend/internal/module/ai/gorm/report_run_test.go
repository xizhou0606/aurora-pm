package aigorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"gorm.io/gorm"
)

func newReportRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ai_report_runs?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AIRun{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestStartReportRunFailsClosedWhenDisabled(t *testing.T) {
	repository := NewReportRunRepository(newReportRunDB(t), false, "test-model")
	_, err := repository.StartReportRun(context.Background(), ai.ReportRunStartCommand{
		WorkspaceID: 1, PrincipalID: 9, Capability: ai.CapabilityReportDaily, SubjectType: ai.ReportSubjectMember, SubjectID: 101, Input: []byte(`{}`),
	})
	if err != ai.ErrReportRunUnavailable {
		t.Fatalf("err=%v, want unavailable", err)
	}
	repository = NewReportRunRepository(newReportRunDB(t), true, "")
	if _, err := repository.StartReportRun(context.Background(), ai.ReportRunStartCommand{
		WorkspaceID: 1, PrincipalID: 9, Capability: ai.CapabilityReportDaily, SubjectType: ai.ReportSubjectMember, SubjectID: 101, Input: []byte(`{}`),
	}); err != ai.ErrReportRunUnavailable {
		t.Fatalf("err=%v, want unavailable without model", err)
	}
}

func TestStartReportRunPersistsQueuedMemberRun(t *testing.T) {
	db := newReportRunDB(t)
	repository := NewReportRunRepository(db, true, "test-model")
	result, err := repository.StartReportRun(context.Background(), ai.ReportRunStartCommand{
		WorkspaceID: 1, PrincipalID: 9, Capability: ai.CapabilityReportWeekly, SubjectType: ai.ReportSubjectMember, SubjectID: 101, Input: []byte(`{"period":"week"}`),
	})
	if err != nil || result.RunID == 0 || !result.Enabled || result.Status != "queued" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var run AIRun
	if err := db.First(&run, result.RunID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Capability != ai.CapabilityReportWeekly || run.SubjectType != ai.ReportSubjectMember ||
		run.SubjectID != 101 || run.Model != "test-model" || run.Status != "queued" || run.RequestedByUser != 9 {
		t.Fatalf("run=%+v", run)
	}
}

func TestFindReportRunScopesWorkspaceCapabilityAndSummary(t *testing.T) {
	db := newReportRunDB(t)
	if err := db.Create(&[]AIRun{
		{ID: 601, WorkspaceID: 1, Capability: ai.CapabilityReportDaily, SubjectType: "member", SubjectID: 101, SubjectVersion: 1, Status: "completed", Model: "m", InputJSON: `{}`, OutputJSON: `{"summary":"今日完成 3 项任务"}`, RequestedByUser: 9},
		{ID: 602, WorkspaceID: 2, Capability: ai.CapabilityReportDaily, SubjectType: "member", SubjectID: 101, SubjectVersion: 1, Status: "completed", Model: "m", InputJSON: `{}`, OutputJSON: `{"summary":"other"}`, RequestedByUser: 9},
		{ID: 603, WorkspaceID: 1, Capability: "requirement.completeness", SubjectType: "requirement", SubjectID: 13, SubjectVersion: 1, Status: "completed", Model: "m", InputJSON: `{}`, OutputJSON: `{"summary":"secret"}`, RequestedByUser: 9},
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewReportRunRepository(db, true, "test-model")
	// 轮询读取随行返回叙述与代码质量评估：报告页要在完成后直接渲染它们，
	// 单行输出（几 KB）的代价可接受；范围裁剪（工作区/能力前缀）不变。
	result, err := repository.FindReportRun(context.Background(), 1, 601)
	if err != nil || result.Status != "completed" || result.Summary != "今日完成 3 项任务" ||
		result.SubjectID != 101 || result.SubjectType != ai.ReportSubjectMember || result.CommitQuality != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	// Export read loads the narrative and input facts.
	withInput, withInputErr := repository.FindReportRunWithInput(context.Background(), 1, 601)
	if withInputErr != nil || withInput.Summary != "今日完成 3 项任务" || withInput.InputJSON == "" {
		t.Fatalf("withInput=%+v err=%v", withInput, withInputErr)
	}
	// A foreign workspace row and a non-report capability must both be invisible.
	if _, err := repository.FindReportRun(context.Background(), 1, 602); err != ai.ErrReportRunNotFound {
		t.Fatalf("err=%v, want not found", err)
	}
	if _, err := repository.FindReportRun(context.Background(), 1, 603); err != ai.ErrReportRunNotFound {
		t.Fatalf("err=%v, want not found", err)
	}
}

func TestFindLatestReportRunMatchesViewAnchor(t *testing.T) {
	db := newReportRunDB(t)
	if err := db.Create(&[]AIRun{
		{ID: 621, WorkspaceID: 1, Capability: ai.CapabilityReportDaily, SubjectType: "member", SubjectID: 101, SubjectVersion: 1, Status: "completed", Model: "m",
			InputJSON: `{"capability":"report.daily","workspace_id":1,"subject":{"target":{"id":101},"period":{"type":"daily","anchor_date":"2026-09-09"}}}`, OutputJSON: `{"summary":"九号的日报"}`},
		{ID: 622, WorkspaceID: 1, Capability: ai.CapabilityReportDaily, SubjectType: "member", SubjectID: 101, SubjectVersion: 1, Status: "completed", Model: "m",
			InputJSON: `{"capability":"report.daily","workspace_id":1,"subject":{"target":{"id":101},"period":{"type":"daily","anchor_date":"2026-09-10"}}}`, OutputJSON: `{"summary":"十号的日报"}`},
		{ID: 623, WorkspaceID: 1, Capability: ai.CapabilityReportWeekly, SubjectType: "member", SubjectID: 101, SubjectVersion: 1, Status: "completed", Model: "m",
			InputJSON: `{"capability":"report.weekly","workspace_id":1,"subject":{"target":{"id":101},"period":{"type":"weekly","anchor_date":"2026-09-06"}}}`, OutputJSON: `{"summary":"上周的周报"}`},
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewReportRunRepository(db, true, "test-model")
	// 命中该视图（能力 + 主体 + 锚点日期）的最新一条。
	result, err := repository.FindLatestReportRun(context.Background(), ai.ReportRunLatestQuery{
		WorkspaceID: 1, Capability: ai.CapabilityReportDaily, SubjectType: ai.ReportSubjectMember, SubjectID: 101, AnchorDate: "2026-09-10",
	})
	if err != nil || result.ID != 622 || result.Summary != "十号的日报" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	// 其他周期与其他锚点互不串扰。
	weekly, weeklyErr := repository.FindLatestReportRun(context.Background(), ai.ReportRunLatestQuery{
		WorkspaceID: 1, Capability: ai.CapabilityReportWeekly, SubjectType: ai.ReportSubjectMember, SubjectID: 101, AnchorDate: "2026-09-06",
	})
	if weeklyErr != nil || weekly.ID != 623 {
		t.Fatalf("weekly=%+v err=%v", weekly, weeklyErr)
	}
	if _, missErr := repository.FindLatestReportRun(context.Background(), ai.ReportRunLatestQuery{
		WorkspaceID: 1, Capability: ai.CapabilityReportDaily, SubjectType: ai.ReportSubjectMember, SubjectID: 101, AnchorDate: "2026-09-08",
	}); missErr != ai.ErrReportRunNotFound {
		t.Fatalf("err=%v, want not found", missErr)
	}
	// 服务层对非法查询 fail closed。
	service, serviceErr := ai.NewReportRunReadService(repository)
	if serviceErr != nil {
		t.Fatal(serviceErr)
	}
	if _, invalidErr := service.FindLatestReportRun(context.Background(), ai.ReportRunLatestQuery{
		WorkspaceID: 1, Capability: "requirement.completeness", SubjectType: ai.ReportSubjectMember, SubjectID: 101, AnchorDate: "2026-09-10",
	}); invalidErr != ai.ErrReportRunInvalid {
		t.Fatalf("err=%v, want invalid", invalidErr)
	}
}

func TestFindReportRunExtractsOverallScore(t *testing.T) {
	db := newReportRunDB(t)
	if err := db.Create(&[]AIRun{
		{ID: 611, WorkspaceID: 1, Capability: ai.CapabilityReportWeekly, SubjectType: "member", SubjectID: 101, SubjectVersion: 1, Status: "completed", Model: "m", InputJSON: `{}`, OutputJSON: `{"summary":"正文","overall_comment":"评语","overall_score":86}`},
		{ID: 612, WorkspaceID: 1, Capability: ai.CapabilityReportWeekly, SubjectType: "member", SubjectID: 102, SubjectVersion: 1, Status: "completed", Model: "m", InputJSON: `{}`, OutputJSON: `{"summary":"越界分数收敛","overall_score":250}`},
		{ID: 613, WorkspaceID: 1, Capability: ai.CapabilityReportWeekly, SubjectType: "member", SubjectID: 103, SubjectVersion: 1, Status: "completed", Model: "m", InputJSON: `{}`, OutputJSON: `{"summary":"旧格式运行无评分"}`},
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewReportRunRepository(db, true, "test-model")
	result, err := repository.FindReportRun(context.Background(), 1, 611)
	if err != nil || result.OverallScore == nil || *result.OverallScore != 86 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	// 越界分数被夹回 0-100。
	clamped, clampedErr := repository.FindReportRun(context.Background(), 1, 612)
	if clampedErr != nil || clamped.OverallScore == nil || *clamped.OverallScore != 100 {
		t.Fatalf("clamped=%+v err=%v", clamped, clampedErr)
	}
	// 旧格式运行没有评分字段，保持 nil 以省略 API 字段。
	legacy, legacyErr := repository.FindReportRun(context.Background(), 1, 613)
	if legacyErr != nil || legacy.OverallScore != nil {
		t.Fatalf("legacy=%+v err=%v", legacy, legacyErr)
	}
}
