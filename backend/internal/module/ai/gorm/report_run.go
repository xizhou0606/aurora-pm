package aigorm

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
)

// ReportRunRepository persists and reads AI reporting runs. It is fail-closed:
// run creation requires an enabled AI capability and a configured runtime
// model, and reads only ever surface report-prefixed runs of one workspace.
type ReportRunRepository struct {
	db      *gorm.DB
	enabled bool
	model   string
}

func NewReportRunRepository(db *gorm.DB, enabled bool, model string) *ReportRunRepository {
	return &ReportRunRepository{db: db, enabled: enabled, model: strings.TrimSpace(model)}
}

func (repository *ReportRunRepository) StartReportRun(
	ctx context.Context,
	command ai.ReportRunStartCommand,
) (ai.ReportRunStartResult, error) {
	if repository == nil || repository.db == nil || !repository.enabled || repository.model == "" {
		return ai.ReportRunStartResult{}, ai.ErrReportRunUnavailable
	}
	run := AIRun{
		WorkspaceID: command.WorkspaceID, Capability: command.Capability,
		SubjectType: command.SubjectType, SubjectID: command.SubjectID, SubjectVersion: 1,
		Status: "queued", Model: repository.model, InputJSON: string(command.Input),
		RequestedByUser: command.PrincipalID,
	}
	if err := repository.db.WithContext(ctx).Create(&run).Error; err != nil {
		return ai.ReportRunStartResult{}, err
	}
	return ai.ReportRunStartResult{RunID: run.ID, Status: run.Status, Enabled: true}, nil
}

// FindReportRun is the polling read: it deliberately skips the large
// input/output columns so status polling never loads report payloads.
func (repository *ReportRunRepository) FindReportRun(
	ctx context.Context,
	workspaceID uint,
	runID uint,
) (ai.ReportRunResult, error) {
	var run AIRun
	err := repository.db.WithContext(ctx).
		// output_json 随轮询返回：叙述与代码质量评估都在里面，单行代价可接受。
		Select("id", "workspace_id", "capability", "subject_type", "subject_id", "status", "error_message", "created_at", "completed_at", "output_json").
		Where("workspace_id = ? AND id = ? AND capability LIKE ?", workspaceID, runID, ai.ReportCapabilityPrefix+"%").
		Take(&run).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ai.ReportRunResult{}, ai.ErrReportRunNotFound
		}
		return ai.ReportRunResult{}, err
	}
	return reportRunResult(run), nil
}

// FindReportRunWithInput additionally loads input_json/output_json for
// export paths (Feishu document title/content) that genuinely need them.
func (repository *ReportRunRepository) FindReportRunWithInput(
	ctx context.Context,
	workspaceID uint,
	runID uint,
) (ai.ReportRunResult, error) {
	var run AIRun
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ? AND capability LIKE ?", workspaceID, runID, ai.ReportCapabilityPrefix+"%").
		Take(&run).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ai.ReportRunResult{}, ai.ErrReportRunNotFound
		}
		return ai.ReportRunResult{}, err
	}
	return reportRunResult(run), nil
}

// reportRunResult extracts the narrative summary from the run output. The
// reporting skill contract mirrors the shared runtime envelope: the output
// object must carry a non-empty "summary" once the run completes.
func reportRunResult(run AIRun) ai.ReportRunResult {
	summary := ""
	if strings.TrimSpace(run.OutputJSON) != "" {
		var output struct {
			Summary string `json:"summary"`
		}
		if json.Unmarshal([]byte(run.OutputJSON), &output) == nil {
			summary = output.Summary
		}
	}
	overallComment := ""
	if strings.TrimSpace(run.OutputJSON) != "" {
		var output struct {
			OverallComment string `json:"overall_comment"`
		}
		if json.Unmarshal([]byte(run.OutputJSON), &output) == nil {
			overallComment = output.OverallComment
		}
	}
	return ai.ReportRunResult{
		ID: run.ID, Status: run.Status, Capability: run.Capability,
		SubjectType: run.SubjectType, SubjectID: run.SubjectID,
		Summary: summary, ErrorMessage: run.ErrorMessage, CreatedAt: run.CreatedAt, CompletedAt: run.CompletedAt,
		InputJSON:         run.InputJSON,
		CommitQuality:     commitQualityFromOutput(run.OutputJSON),
		OverallComment:    overallComment,
		OverallScore:      overallScoreFromOutput(run.OutputJSON),
		DimensionComments: dimensionCommentsFromOutput(run.OutputJSON),
	}
}

// overallScoreFromOutput extracts the narrative skill's 0-100 comprehensive
// score from the stored run output envelope; runs without it (old runs) yield
// nil so the API field stays omitted. Out-of-range values are clamped.
func overallScoreFromOutput(raw string) *int {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var output struct {
		OverallScore *int `json:"overall_score"`
	}
	if json.Unmarshal([]byte(raw), &output) != nil || output.OverallScore == nil {
		return nil
	}
	score := *output.OverallScore
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return &score
}

// dimensionCommentsFromOutput extracts the per-ranking AI commentary from the
// stored run output envelope; runs without it (old runs, member reports)
// yield nil so the API field stays omitted.
func dimensionCommentsFromOutput(raw string) *ai.ReportDimensionComments {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var output struct {
		DimensionComments *ai.ReportDimensionComments `json:"dimension_comments"`
	}
	if json.Unmarshal([]byte(raw), &output) != nil || output.DimensionComments == nil {
		return nil
	}
	comments := output.DimensionComments
	if comments.Tasks == "" && comments.Quality == "" && comments.Code == "" {
		return nil
	}
	return comments
}

// commitQualityFromOutput extracts the code-quality skill assessment from the
// stored run output envelope; runs without the skill yield nil.
func commitQualityFromOutput(raw string) *ai.CommitQualitySummary {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var output struct {
		CommitQuality *ai.CommitQualitySummary `json:"commit_quality"`
	}
	if json.Unmarshal([]byte(raw), &output) != nil || output.CommitQuality == nil {
		return nil
	}
	return output.CommitQuality
}

// latestRunScanLimit bounds how many recent runs of one view are checked for
// an anchor match; dedup keeps the per-subject run count small, so the newest
// match is virtually always within the first few rows.
const latestRunScanLimit = 30

// FindLatestReportRun returns the subject's newest run whose input period
// anchor matches the requested view. 定时生成的汇报没有前端回执，预览接口
// 靠它把最近一次评语/正文自动带出来。anchor 从输入的头部片段提取：JSON 按
// 结构体字段序写出，subject 的 period 紧随 target，anchor_date 必落在片段
// 内——不传输整份输入载荷，方言也不依赖具体 JSON 操作符。
func (repository *ReportRunRepository) FindLatestReportRun(
	ctx context.Context,
	query ai.ReportRunLatestQuery,
) (ai.ReportRunResult, error) {
	if repository == nil || repository.db == nil {
		return ai.ReportRunResult{}, ai.ErrReportRunUnavailable
	}
	var rows []AIRun
	err := repository.db.WithContext(ctx).
		Select("id", "workspace_id", "capability", "subject_type", "subject_id", "status", "error_message", "created_at", "completed_at", "output_json",
			"substr(input_json, 1, 4096) AS input_json").
		Where("workspace_id = ? AND capability = ? AND subject_type = ? AND subject_id = ?",
			query.WorkspaceID, query.Capability, query.SubjectType, query.SubjectID).
		Order("id DESC").Limit(latestRunScanLimit).
		Find(&rows).Error
	if err != nil {
		return ai.ReportRunResult{}, err
	}
	for _, run := range rows {
		if runInputAnchorDate(run.InputJSON) != query.AnchorDate {
			continue
		}
		return reportRunResult(run), nil
	}
	return ai.ReportRunResult{}, ai.ErrReportRunNotFound
}

// runAnchorDatePattern matches the JSON key inside input.subject.period; the
// interpolated date is a plain calendar string, never escaped.
var runAnchorDatePattern = regexp.MustCompile(`"anchor_date":"([0-9]{4}-[0-9]{2}-[0-9]{2})"`)

func runInputAnchorDate(inputFragment string) string {
	match := runAnchorDatePattern.FindStringSubmatch(inputFragment)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func (repository *ReportRunRepository) HasReportRun(
	ctx context.Context,
	query ai.ReportRunDedupQuery,
) (bool, error) {
	if repository == nil || repository.db == nil {
		return false, ai.ErrReportRunUnavailable
	}
	var count int64
	err := repository.db.WithContext(ctx).Model(&AIRun{}).
		Where("workspace_id = ? AND capability = ? AND subject_type = ? AND subject_id = ?"+
			" AND created_at >= ? AND created_at < ?",
			query.WorkspaceID, query.Capability, query.SubjectType, query.SubjectID,
			query.WindowStart, query.WindowEnd).
		Limit(1).Count(&count).Error
	return count > 0, err
}

func (repository *ReportRunRepository) ListReportRunSubjectIDs(
	ctx context.Context,
	query ai.ReportRunDedupBatchQuery,
) (map[uint]bool, error) {
	if repository == nil || repository.db == nil || len(query.SubjectIDs) == 0 {
		return map[uint]bool{}, nil
	}
	var subjectIDs []uint
	err := repository.db.WithContext(ctx).Model(&AIRun{}).
		Where("workspace_id = ? AND capability = ? AND subject_type = ?"+
			" AND subject_id IN ? AND created_at >= ? AND created_at < ?",
			query.WorkspaceID, query.Capability, query.SubjectType, query.SubjectIDs,
			query.WindowStart, query.WindowEnd).
		Distinct("subject_id").
		Pluck("subject_id", &subjectIDs).Error
	if err != nil {
		return nil, err
	}
	existing := make(map[uint]bool, len(subjectIDs))
	for _, id := range subjectIDs {
		existing[id] = true
	}
	return existing, nil
}

var (
	_ ai.ReportRunStartStore = (*ReportRunRepository)(nil)
	_ ai.ReportRunReadStore  = (*ReportRunRepository)(nil)
	_ ai.ReportRunDedupStore = (*ReportRunRepository)(nil)
)
