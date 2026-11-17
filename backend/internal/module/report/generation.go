package report

import (
	"context"
	"encoding/json"
	"time"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

// ReportFacts carries the pre-bucketed facts one report composes from. The
// scheduler and interactive preview share this shape so both paths cost one
// fact load per workspace and per-subject work stays on the subject's own
// bucket.
type ReportFacts struct {
	Target  ReportTarget
	Members []MemberRollupFacts
	Tasks   []task.Snapshot
	Bugs    []quality.BugSnapshot
	// ReportedBugs keys bug snapshots by reporter's platform fact ID so
	// testing subjects aggregate 提出缺陷. Nil means the bugs port was not
	// loaded (metrics stay zero).
	ReportedBugs map[uint][]quality.BugSnapshot
	// OwnedRequirements keys requirement snapshots by product owner's platform
	// fact ID so product subjects aggregate 需求数量. Nil means the
	// requirement port is not wired (未接入, metrics stay zero).
	OwnedRequirements map[uint][]requirement.Snapshot
	// ContributionRows keys one person's day buckets by platform fact ID;
	// nil means the code-contribution port is not wired (未接入).
	ContributionRows      map[uint][]DailyContributionFact
	ContributionFreshness *CodeFreshness
	// CommitRows keys one person's period commits (with diff evidence) by
	// platform fact ID; nil means the port is not wired.
	CommitRows map[uint][]MemberCommitFact
}

// teamReportedBugs concatenates every member's reported-bug bucket for a
// team-level aggregate. A bug has exactly one reporter, so member buckets
// never overlap and no deduplication is needed.
func (facts ReportFacts) teamReportedBugs() []quality.BugSnapshot {
	union := make([]quality.BugSnapshot, 0)
	for _, member := range facts.Members {
		union = append(union, facts.ReportedBugs[member.Member.ID]...)
	}
	return union
}

// teamOwnedRequirementBuckets collects every member's owned-requirement
// bucket for the distinct team aggregate (co-owned requirements are
// deduplicated by ID at aggregation time).
func (facts ReportFacts) teamOwnedRequirementBuckets() [][]requirement.Snapshot {
	buckets := make([][]requirement.Snapshot, 0, len(facts.Members))
	for _, member := range facts.Members {
		if bucket := facts.OwnedRequirements[member.Member.ID]; len(bucket) > 0 {
			buckets = append(buckets, bucket)
		}
	}
	return buckets
}

// targetCommitRows collects the subject's own commits; group/department
// subjects accumulate every member's commits so the AI reviews the whole
// subject's work, not just one person's slice.
func (facts ReportFacts) targetCommitRows(factID uint) []MemberCommitFact {
	rows := append([]MemberCommitFact{}, facts.CommitRows[factID]...)
	for _, member := range facts.Members {
		if member.Member.ID != factID {
			rows = append(rows, facts.CommitRows[member.Member.ID]...)
		}
	}
	return rows
}

// targetContributionRows returns the raw rows for one fact ID (member kind).
func (facts ReportFacts) targetContributionRows(factID uint) []DailyContributionFact {
	return facts.ContributionRows[factID]
}

// MemberRollupFacts is one member's raw bucket inside a group report. The
// rollup metrics are aggregated inside composeReport where the period is
// known.
type MemberRollupFacts struct {
	Member MemberRef
	Tasks  []task.Snapshot
	Bugs   []quality.BugSnapshot
}

// StartReportResult is the response of a generate request: the queued
// run plus the computed facts the narrative will be based on, so the UI can
// render the preview immediately while the AI run executes.
type StartReportResult struct {
	RunID  uint   `json:"run_id"`
	Status string `json:"status"`
	Report Report `json:"report"`
}

// StartReport queues an interactive AI reporting run with an operator
// principal.
func (service *Service) StartReport(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	kind ReportTargetKind,
	targetID uint,
	periodType PeriodType,
	anchorDate string,
	location *time.Location,
) (StartReportResult, error) {
	if principalID == 0 {
		return StartReportResult{}, ErrReportQueryInvalid
	}
	report, period, err := service.buildReportWithPeriod(
		ctx, workspaceID, kind, targetID, periodType, anchorDate, location,
	)
	if err != nil {
		return StartReportResult{}, err
	}
	return service.queueReport(ctx, workspaceID, principalID, false, periodType, period, report)
}

// QueueScheduledReport composes from pre-bucketed facts and queues a
// system-initiated run. The scheduler uses it so a full pass never reloads
// workspace facts per subject.
func (service *Service) QueueScheduledReport(
	ctx context.Context,
	workspaceID uint,
	periodType PeriodType,
	period Period,
	now time.Time,
	facts ReportFacts,
) (StartReportResult, error) {
	if workspaceID == 0 || facts.Target.ID == 0 {
		return StartReportResult{}, ErrReportQueryInvalid
	}
	return service.queueReport(ctx, workspaceID, 0, true, periodType, period, composeReport(facts, period, now))
}

// MemberReportRunStatus polls one reporting run of the workspace.
func (service *Service) MemberReportRunStatus(
	ctx context.Context,
	workspaceID uint,
	runID uint,
) (ai.ReportRunResult, error) {
	if workspaceID == 0 || runID == 0 {
		return ai.ReportRunResult{}, ErrReportQueryInvalid
	}
	return service.dependencies.RunResults.FindReportRun(ctx, workspaceID, runID)
}

// LatestReportRunForView resolves the newest run generated for exactly one
// report view (subject + period type + anchor date). 定时生成的汇报没有前端
// 回执：预览接口靠它把评语与正文自动带出来；无 run 属正常情况（该视图尚
// 未生成），返回 not found 由调用方省略。
func (service *Service) LatestReportRunForView(
	ctx context.Context,
	workspaceID uint,
	kind ReportTargetKind,
	targetID uint,
	period Period,
) (ai.ReportRunResult, error) {
	if workspaceID == 0 || targetID == 0 {
		return ai.ReportRunResult{}, ErrReportQueryInvalid
	}
	return service.dependencies.RunResults.FindLatestReportRun(ctx, ai.ReportRunLatestQuery{
		WorkspaceID: workspaceID,
		Capability:  reportCapability(period.Type),
		SubjectType: reportSubjectType(kind),
		SubjectID:   targetID,
		AnchorDate:  period.AnchorDate,
	})
}

// ExportedDocument references the Feishu document created from a report run.
type ExportedDocument struct {
	DocumentID string `json:"document_id"`
	URL        string `json:"url"`
	Title      string `json:"title"`
}

// ExportReportRun publishes a completed reporting run as a Feishu document.
// The document body is the full report — facts snapshot, score table, itemised
// details, and the AI narrative — rebuilt from the run's own stored input so
// it matches what the narrative was based on. Publishing is idempotent per
// run: the first created document is persisted and every later call for the
// same run returns that document instead of creating a duplicate.
func (service *Service) ExportReportRun(
	ctx context.Context,
	workspaceID uint,
	runID uint,
) (ExportedDocument, error) {
	if workspaceID == 0 || runID == 0 {
		return ExportedDocument{}, ErrReportQueryInvalid
	}
	if service.dependencies.Exporter == nil {
		return ExportedDocument{}, ErrReportExportUnavailable
	}
	run, err := service.dependencies.RunResults.FindReportRunWithInput(ctx, workspaceID, runID)
	if err != nil {
		return ExportedDocument{}, err
	}
	if run.Status != "completed" || run.Summary == "" {
		return ExportedDocument{}, ErrReportRunNotCompleted
	}
	title := reportDocumentTitle(run)
	if service.dependencies.ExportRecords != nil {
		record, found, err := service.dependencies.ExportRecords.FindReportExport(ctx, workspaceID, runID)
		if err != nil {
			return ExportedDocument{}, err
		}
		if found {
			return ExportedDocument{DocumentID: record.DocumentID, URL: record.URL, Title: title}, nil
		}
	}
	document, err := service.dependencies.Exporter.CreateReportDocument(ctx, title, composeReportRunContent(run.InputJSON, run.Summary))
	if err != nil {
		return ExportedDocument{}, err
	}
	if service.dependencies.ExportRecords != nil {
		// 登记是尽力而为的幂等保障：并发请求抢先登记时复用先到的文档。
		claimed, err := service.dependencies.ExportRecords.ClaimReportExport(ctx, workspaceID, runID,
			ai.ReportExportRecord{DocumentID: document.DocumentID, URL: document.URL})
		if err == nil && !claimed {
			if record, found, findErr := service.dependencies.ExportRecords.FindReportExport(ctx, workspaceID, runID); findErr == nil && found {
				return ExportedDocument{DocumentID: record.DocumentID, URL: record.URL, Title: title}, nil
			}
		}
	}
	return ExportedDocument{DocumentID: document.DocumentID, URL: document.URL, Title: title}, nil
}

// ExportReportFacts publishes the current facts-only report (no AI narrative)
// as a Feishu document. It backs the always-available 上传飞书文档 button so
// publishing never requires a completed AI run first: the facts are
// re-aggregated at publish time from the same projection the preview renders.
// Each call publishes a fresh document; there is no run row to deduplicate
// against, and the frontend keeps the button single-flight while pending.
func (service *Service) ExportReportFacts(
	ctx context.Context,
	workspaceID uint,
	kind ReportTargetKind,
	targetID uint,
	periodType PeriodType,
	anchorDate string,
	location *time.Location,
) (ExportedDocument, error) {
	if workspaceID == 0 || targetID == 0 || (kind != TargetMember && kind != TargetGroup && kind != TargetDepartment) {
		return ExportedDocument{}, ErrReportQueryInvalid
	}
	if service.dependencies.Exporter == nil {
		return ExportedDocument{}, ErrReportExportUnavailable
	}
	report, period, err := service.buildReportWithPeriod(
		ctx, workspaceID, kind, targetID, periodType, anchorDate, location,
	)
	if err != nil {
		return ExportedDocument{}, err
	}
	title := "【" + periodLabel(period.Type) + "】" + report.Target.Name + " " + period.AnchorDate
	content := composeReportDocumentMarkdown(report, "")
	document, err := service.dependencies.Exporter.CreateReportDocument(ctx, title, content)
	if err != nil {
		return ExportedDocument{}, err
	}
	return ExportedDocument{DocumentID: document.DocumentID, URL: document.URL, Title: title}, nil
}

func (service *Service) queueReport(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	scheduled bool,
	periodType PeriodType,
	period Period,
	report Report,
) (StartReportResult, error) {
	input, err := json.Marshal(reportRunPayload(reportCapability(periodType), workspaceID, report))
	if err != nil {
		return StartReportResult{}, err
	}
	result, err := service.dependencies.Runs.StartReportRun(ctx, ai.ReportRunStartCommand{
		WorkspaceID: workspaceID,
		PrincipalID: principalID,
		Capability:  reportCapability(periodType),
		SubjectType: reportSubjectType(report.Target.Kind),
		SubjectID:   report.Target.ID,
		Input:       input,
		Scheduled:   scheduled,
	})
	if err != nil {
		return StartReportResult{}, err
	}
	return StartReportResult{RunID: result.RunID, Status: result.Status, Report: report}, nil
}

// reportDocumentTitle composes the Feishu document title from the run input
// subject so the title always matches the run's own facts.
func reportDocumentTitle(run ai.ReportRunResult) string {
	var input struct {
		Subject struct {
			Target struct {
				Name string `json:"name"`
			} `json:"target"`
			Period struct {
				Type       PeriodType `json:"type"`
				AnchorDate string     `json:"anchor_date"`
			} `json:"period"`
		} `json:"subject"`
	}
	if err := json.Unmarshal([]byte(run.InputJSON), &input); err == nil &&
		input.Subject.Target.Name != "" {
		return "【" + periodLabel(input.Subject.Period.Type) + "】" +
			input.Subject.Target.Name + " " + input.Subject.Period.AnchorDate
	}
	return "PM 汇报 " + run.Capability
}

func periodLabel(periodType PeriodType) string {
	switch periodType {
	case PeriodWeekly:
		return "周报"
	case PeriodMonthly:
		return "月报"
	default:
		return "日报"
	}
}

func reportCapability(periodType PeriodType) string {
	switch periodType {
	case PeriodWeekly:
		return ai.CapabilityReportWeekly
	case PeriodMonthly:
		return ai.CapabilityReportMonthly
	default:
		return ai.CapabilityReportDaily
	}
}

func reportSubjectType(kind ReportTargetKind) string {
	switch kind {
	case TargetGroup:
		return ai.ReportSubjectUserGroup
	case TargetDepartment:
		return ai.ReportSubjectDepartment
	default:
		return ai.ReportSubjectMember
	}
}
