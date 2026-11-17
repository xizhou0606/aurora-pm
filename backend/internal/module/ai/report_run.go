package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Report capabilities are the built-in daily/weekly/monthly reporting skills
// executed by the shared AI runtime. The runtime owns the per-capability
// prompt; PM supplies the aggregated member or role-group facts as the run
// input and the narrative comes back as the run output summary.
const (
	ReportCapabilityPrefix  = "report."
	CapabilityReportDaily   = "report.daily"
	CapabilityReportWeekly  = "report.weekly"
	CapabilityReportMonthly = "report.monthly"
	ReportSubjectMember     = "member"
	ReportSubjectUserGroup  = "user_group"
	ReportSubjectDepartment = "department"
)

var (
	ErrReportRunUnavailable = errors.New("AI report run is unavailable")
	ErrReportRunInvalid     = errors.New("AI report run query is invalid")
	ErrReportRunNotFound    = errors.New("AI report run not found")
)

// ReportRunCapabilities enumerates the supported reporting skills.
func ReportRunCapabilities() []string {
	return []string{CapabilityReportDaily, CapabilityReportWeekly, CapabilityReportMonthly}
}

func IsReportCapability(capability string) bool {
	return strings.HasPrefix(strings.TrimSpace(capability), ReportCapabilityPrefix)
}

func IsReportRunSubject(subjectType string) bool {
	switch strings.TrimSpace(subjectType) {
	case ReportSubjectMember, ReportSubjectUserGroup, ReportSubjectDepartment:
		return true
	default:
		return false
	}
}

// ReportRunStartCommand requests one AI reporting run for a workspace member
// or role group. Input carries the aggregated task/bug facts; it is stored
// verbatim so the runtime skill can generate the narrative from PM facts
// only.
type ReportRunStartCommand struct {
	WorkspaceID uint
	PrincipalID uint
	Capability  string
	SubjectType string
	SubjectID   uint
	Input       json.RawMessage
	// Scheduled marks system-initiated runs from the report scheduler; they
	// carry no operator principal (RequestedByUser 0 = system).
	Scheduled bool
}

type ReportRunStartResult struct {
	RunID   uint
	Status  string
	Enabled bool
}

type ReportRunStartStore interface {
	StartReportRun(context.Context, ReportRunStartCommand) (ReportRunStartResult, error)
}

// ReportRunStartService creates queued reporting runs. It fails closed when
// the AI capability is disabled or no runtime model is configured.
type ReportRunStartService struct {
	store   ReportRunStartStore
	enabled bool
}

func NewReportRunStartService(store ReportRunStartStore, enabled bool) (*ReportRunStartService, error) {
	if store == nil {
		return nil, ErrReportRunUnavailable
	}
	return &ReportRunStartService{store: store, enabled: enabled}, nil
}

func (service *ReportRunStartService) StartReportRun(
	ctx context.Context,
	command ReportRunStartCommand,
) (ReportRunStartResult, error) {
	command.Capability = strings.TrimSpace(command.Capability)
	command.SubjectType = strings.TrimSpace(command.SubjectType)
	if service == nil || service.store == nil || !service.enabled || command.WorkspaceID == 0 ||
		command.SubjectID == 0 || !IsReportRunSubject(command.SubjectType) ||
		!IsReportCapability(command.Capability) || len(command.Input) == 0 || !json.Valid(command.Input) {
		return ReportRunStartResult{}, ErrReportRunUnavailable
	}
	// Interactive runs need an operator principal; scheduled runs are system.
	if !command.Scheduled && command.PrincipalID == 0 {
		return ReportRunStartResult{}, ErrReportRunUnavailable
	}
	return service.store.StartReportRun(ctx, command)
}

// ReportRunResult is the polled state of one reporting run. Summary carries
// the AI narrative and is only populated for completed runs. InputJSON is
// internal (used by the report domain to compose the export title).
// CommitQuality is the dedicated code-quality skill's structured assessment,
// surfaced so the report page can render it beside the commit evidence.
type ReportRunResult struct {
	ID             uint                  `json:"id"`
	Status         string                `json:"status"`
	Capability     string                `json:"capability"`
	SubjectType    string                `json:"subject_type"`
	SubjectID      uint                  `json:"subject_id"`
	Summary        string                `json:"summary,omitempty"`
	ErrorMessage   string                `json:"error_message,omitempty"`
	CreatedAt      time.Time             `json:"created_at,omitempty"`
	CompletedAt    *time.Time            `json:"completed_at,omitempty"`
	InputJSON      string                `json:"-"`
	CommitQuality  *CommitQualitySummary `json:"commit_quality,omitempty"`
	OverallComment string                `json:"overall_comment,omitempty"`
	// OverallScore 是叙述技能按岗位侧重给出的 0-100 综合评分；缺失（旧
	// 运行、模型未输出）时保持 nil，API 字段随之省略。
	OverallScore *int `json:"overall_score,omitempty"`
	// DimensionComments carries the per-ranking AI commentary (tasks, quality,
	// code contribution) extracted from the run output; team reports fill all
	// three, member runs leave them empty.
	DimensionComments *ReportDimensionComments `json:"dimension_comments,omitempty"`
}

// ReportDimensionComments is one AI short paragraph per ranking dimension,
// rendered beside the 任务/质量/代码贡献 member rankings on team reports.
type ReportDimensionComments struct {
	Tasks   string `json:"tasks,omitempty"`
	Quality string `json:"quality,omitempty"`
	Code    string `json:"code,omitempty"`
}

// CommitQualitySummary is the code-quality skill's comprehensive assessment.
type CommitQualitySummary struct {
	Score        int                    `json:"score"`
	Conclusion   string                 `json:"conclusion"`
	CommonIssues []string               `json:"common_issues,omitempty"`
	Risks        []string               `json:"risks,omitempty"`
	Suggestions  []string               `json:"suggestions,omitempty"`
	Commits      []CommitQualityVerdict `json:"commits,omitempty"`
}

// CommitQualityVerdict is one commit's AI quality verdict.
type CommitQualityVerdict struct {
	SHA     string   `json:"sha"`
	Verdict string   `json:"verdict"`
	Summary string   `json:"summary"`
	Issues  []string `json:"issues,omitempty"`
}

// ReportRunReadStore reads reporting runs scoped to a workspace and the
// report capability prefix so unrelated AI runs can never be surfaced.
// FindReportRun is the polling read and skips large payload columns;
// FindReportRunWithInput additionally loads input_json/output_json for
// export paths. FindLatestReportRun backs the preview endpoint: scheduled
// runs have no frontend receipt, so the page rediscovers the newest run of
// the exact view (capability + subject + input period anchor) on open.
type ReportRunReadStore interface {
	FindReportRun(ctx context.Context, workspaceID uint, runID uint) (ReportRunResult, error)
	FindReportRunWithInput(ctx context.Context, workspaceID uint, runID uint) (ReportRunResult, error)
	FindLatestReportRun(ctx context.Context, query ReportRunLatestQuery) (ReportRunResult, error)
}

// ReportRunLatestQuery pins the view a preview is asking for: the subject,
// the report capability (period type), and the resolved anchor date. The
// anchor match guarantees the narrative was generated for exactly the period
// the facts preview shows.
type ReportRunLatestQuery struct {
	WorkspaceID uint
	Capability  string
	SubjectType string
	SubjectID   uint
	AnchorDate  string
}

// ReportExportRecord is the published-document mapping of one reporting run.
type ReportExportRecord struct {
	DocumentID string
	URL        string
}

// ReportExportStore persists at most one published document per reporting
// run so retries and double submissions reuse the same document instead of
// creating duplicates. Implementations must scope every access by workspace
// and the report capability prefix.
type ReportExportStore interface {
	FindReportExport(ctx context.Context, workspaceID uint, runID uint) (ReportExportRecord, bool, error)
	// ClaimReportExport atomically registers the document for the run and
	// reports whether this call won the claim.
	ClaimReportExport(ctx context.Context, workspaceID uint, runID uint, record ReportExportRecord) (bool, error)
}

// ReportRunDedupStore reports which subjects of a batch already have a
// reporting run inside the window. The scheduler uses it to stay idempotent
// across restarts with ONE query per subject-type batch instead of one
// query per subject.
type ReportRunDedupStore interface {
	HasReportRun(ctx context.Context, query ReportRunDedupQuery) (bool, error)
	ListReportRunSubjectIDs(ctx context.Context, query ReportRunDedupBatchQuery) (map[uint]bool, error)
}

type ReportRunDedupBatchQuery struct {
	WorkspaceID uint
	Capability  string
	SubjectType string
	SubjectIDs  []uint
	WindowStart time.Time
	WindowEnd   time.Time
}

type ReportRunDedupQuery struct {
	WorkspaceID uint
	Capability  string
	SubjectType string
	SubjectID   uint
	WindowStart time.Time
	WindowEnd   time.Time
}

type ReportRunDedupService struct{ store ReportRunDedupStore }

func NewReportRunDedupService(store ReportRunDedupStore) (*ReportRunDedupService, error) {
	if store == nil {
		return nil, ErrReportRunInvalid
	}
	return &ReportRunDedupService{store: store}, nil
}

func (service *ReportRunDedupService) ListReportRunSubjectIDs(
	ctx context.Context,
	query ReportRunDedupBatchQuery,
) (map[uint]bool, error) {
	if query.WorkspaceID == 0 || len(query.SubjectIDs) == 0 ||
		!IsReportRunSubject(query.SubjectType) || !IsReportCapability(query.Capability) ||
		!query.WindowStart.Before(query.WindowEnd) {
		return map[uint]bool{}, ErrReportRunInvalid
	}
	return service.store.ListReportRunSubjectIDs(ctx, query)
}

func (service *ReportRunDedupService) HasReportRun(ctx context.Context, query ReportRunDedupQuery) (bool, error) {
	if query.WorkspaceID == 0 || query.SubjectID == 0 ||
		!IsReportRunSubject(query.SubjectType) || !IsReportCapability(query.Capability) ||
		!query.WindowStart.Before(query.WindowEnd) {
		return false, ErrReportRunInvalid
	}
	return service.store.HasReportRun(ctx, query)
}

type ReportRunReadService struct{ store ReportRunReadStore }

func NewReportRunReadService(store ReportRunReadStore) (*ReportRunReadService, error) {
	if store == nil {
		return nil, ErrReportRunInvalid
	}
	return &ReportRunReadService{store: store}, nil
}

func (service *ReportRunReadService) FindReportRun(
	ctx context.Context,
	workspaceID uint,
	runID uint,
) (ReportRunResult, error) {
	if workspaceID == 0 || runID == 0 {
		return ReportRunResult{}, ErrReportRunInvalid
	}
	return service.store.FindReportRun(ctx, workspaceID, runID)
}

func (service *ReportRunReadService) FindReportRunWithInput(
	ctx context.Context,
	workspaceID uint,
	runID uint,
) (ReportRunResult, error) {
	if workspaceID == 0 || runID == 0 {
		return ReportRunResult{}, ErrReportRunInvalid
	}
	return service.store.FindReportRunWithInput(ctx, workspaceID, runID)
}

// FindLatestReportRun resolves the subject's newest run for one exact view.
// A missing run is the normal "nothing generated for this view yet" case.
func (service *ReportRunReadService) FindLatestReportRun(
	ctx context.Context,
	query ReportRunLatestQuery,
) (ReportRunResult, error) {
	if service == nil || service.store == nil || query.WorkspaceID == 0 || query.SubjectID == 0 ||
		!IsReportRunSubject(query.SubjectType) || !IsReportCapability(query.Capability) || query.AnchorDate == "" {
		return ReportRunResult{}, ErrReportRunInvalid
	}
	return service.store.FindLatestReportRun(ctx, query)
}
