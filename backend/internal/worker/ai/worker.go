package aiworker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const (
	maxRuntimeResponseBytes          = 4 << 20
	maxRuntimeRequestBytes           = 7 << 20
	maxQATestCaseRuntimeRequestBytes = 7 << 20
	feishuSourceLoadTimeout          = 30 * time.Second
	qaSourceFreshnessTimeout         = 20 * time.Second
	runStatusPollInterval            = 250 * time.Millisecond
	aiRunClaimTimePrecision          = time.Millisecond
)

type Worker struct {
	documentSources        func(*gorm.DB) (*docsource.Service, error)
	db                     *gorm.DB
	cfg                    config.Config
	client                 *http.Client
	reader                 feishu.Reader
	now                    func() time.Time
	delivery               *qatestcase.Materializer
	deliveryMu             sync.Mutex
	deliveryBackoff        map[uint]deliveryRetry
	deliveryScanBeforeID   uint
	deliveryScanNextAt     time.Time
	deliveryRunsSinceScan  int
	deliveryYieldToQueue   bool
	sourceLoadTimeout      time.Duration
	sourceFreshnessTimeout time.Duration
}

func New(db *gorm.DB, cfg config.Config, reader feishu.Reader) *Worker {
	store, storeErr := objectstore.New(cfg.ObjectStorage)
	return &Worker{
		db: db, cfg: cfg, client: &http.Client{Timeout: cfg.AI.RequestTimeout}, reader: reader,
		now:                    func() time.Time { return time.Now().UTC() },
		delivery:               qatestcase.NewMaterializer(db, cfg.ObjectStorage, store, storeErr),
		deliveryBackoff:        map[uint]deliveryRetry{},
		sourceLoadTimeout:      feishuSourceLoadTimeout,
		sourceFreshnessTimeout: qaSourceFreshnessTimeout,
	}
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	return w.RunOnceRole(ctx, true)
}

// RunOnceRole claims and executes at most one queued run. The lead consumer
// additionally sweeps stale-run recovery and test-case delivery compensation;
// followers skip those scans so they stay single-flight. A lost claim (another
// consumer won the row) reports processed=true so the caller retries
// immediately instead of waiting a full poll interval.
func (w *Worker) RunOnceRole(ctx context.Context, lead bool) (bool, error) {
	if !w.cfg.AI.Enabled {
		return false, nil
	}
	recovered := false
	if lead {
		var err error
		recovered, err = w.recoverStaleAIRuns()
		if err != nil {
			return false, err
		}
		if w.deliveryScanScheduled() {
			processed, scanErr := w.compensateTestCaseDelivery(ctx)
			if scanErr != nil || processed {
				return processed, scanErr
			}
		}
	}
	var run aigorm.AIRun
	queued := w.db.Where("status = ?", "queued").Order("id").Limit(1).Find(&run)
	if queued.Error != nil {
		return false, queued.Error
	}
	if queued.RowsAffected == 0 {
		return recovered, nil
	}
	claimedAt := nextAIRunClaimTime(w.now(), run.UpdatedAt, run.StartedAt)
	claimed := queuedRunScope(w.db.Model(&aigorm.AIRun{}), &run).
		Updates(map[string]any{"status": "running", "started_at": claimedAt})
	if claimed.Error != nil {
		return false, claimed.Error
	}
	if claimed.RowsAffected == 0 {
		// Another consumer won the row; retry with the next queued run right away.
		return true, nil
	}
	run.StartedAt = &claimedAt
	w.noteGenerationRunProcessed()
	if err := w.execute(ctx, &run); err != nil {
		if errors.Is(err, errRunNotRunning) {
			return true, nil
		}
		if failErr := w.failRunClaim(&run, err); failErr != nil {
			if errors.Is(failErr, errRunNotRunning) {
				return true, nil
			}
			return true, failErr
		}
		return true, err
	}
	w.tryTestCaseDelivery(ctx, run)
	return true, nil
}

func (w *Worker) execute(ctx context.Context, run *aigorm.AIRun) error {
	if run == nil || run.StartedAt == nil {
		return errRunNotRunning
	}
	requestCtx, stopWatching := w.contextWhileRunIsActive(ctx, run)
	defer stopWatching()
	var input map[string]any
	if err := json.Unmarshal([]byte(run.InputJSON), &input); err != nil {
		return fmt.Errorf("decode run input: %w", err)
	}
	if err := w.ensureRunClaimActive(run); err != nil {
		return err
	}
	sourceTimeout := "读取飞书需求来源超时，本次 AI 运行已停止；请重试"
	if _, modern := input[docsource.ManifestKey]; modern {
		sourceTimeout = "读取需求文档来源超时，本次 AI 运行已停止；请重试"
	}
	if err := runWithSourceDeadline(requestCtx, w.sourceLoadTimeout, sourceTimeout, func(sourceCtx context.Context) error {
		return w.attachSources(sourceCtx, run, input)
	}); err != nil {
		return err
	}
	qatestcase.RemoveTarget(input)
	// 报告运行携带逐提交证据时，先用专用代码质量审查技能产出逐条结论，
	// 叙述与导出即可引用；失败只降级，不阻断报告。
	w.enrichReportWithCommitQuality(requestCtx, run, input)
	payload := map[string]any{"runId": run.ID, "workspaceId": run.WorkspaceID, "capability": run.Capability, "input": input}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := validateRuntimeRequestSize(run.Capability, len(raw)); err != nil {
		return err
	}
	responseStatus, responseRaw, err := w.callRuntime(requestCtx, raw)
	if err != nil {
		active, statusErr := w.runIsActive(run)
		if statusErr == nil && !active {
			return errRunNotRunning
		}
		return fmt.Errorf("call AI runtime: %w", err)
	}
	if responseStatus != http.StatusOK {
		var envelope struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(responseRaw, &envelope)
		detail := strings.TrimSpace(envelope.Message)
		if detail == "" {
			detail = strings.TrimSpace(envelope.Error)
		}
		if detail != "" {
			return fmt.Errorf("AI runtime returned status %d: %s", responseStatus, detail)
		}
		return fmt.Errorf("AI runtime returned status %d", responseStatus)
	}
	var envelope struct {
		Data struct {
			Output json.RawMessage `json:"output"`
			Usage  json.RawMessage `json:"usage"`
		} `json:"data"`
	}
	if json.Unmarshal(responseRaw, &envelope) != nil || len(envelope.Data.Output) == 0 {
		return fmt.Errorf("AI runtime returned invalid structured output")
	}
	var output struct {
		Summary string `json:"summary"`
		Changes []struct {
			Field string `json:"field"`
		} `json:"changes"`
	}
	if json.Unmarshal(envelope.Data.Output, &output) != nil || output.Summary == "" {
		return fmt.Errorf("AI runtime output schema mismatch")
	}
	if err := validateQATestCaseSkillProvenance(run.Capability, envelope.Data.Output); err != nil {
		return err
	}
	if err := validateChanges(run.Capability, output.Changes); err != nil {
		return err
	}
	if err := runWithSourceDeadline(requestCtx, w.sourceFreshnessTimeout, "生成后复核飞书需求来源超时，测试用例不会保存；请重试", func(sourceCtx context.Context) error {
		if _, present := input["sources"].([]docsource.Snapshot); present {
			return w.verifyDocumentSources(sourceCtx, w.db, run, input, false)
		}
		return w.verifyQATestCaseSourceFreshness(sourceCtx, run)
	}); err != nil {
		return err
	}
	if _, err := taskplan.ParseForSubject(run.Capability, run.SubjectType, run.SubjectID, envelope.Data.Output); err != nil {
		return err
	}
	envelope.Data.Output = docsource.DiscloseUnexpanded(envelope.Data.Output, input)
	return w.completeRunClaim(run, output.Summary, envelope.Data.Output, envelope.Data.Usage, w.now(), input)
}

func (w *Worker) completeRunClaim(
	run *aigorm.AIRun, title string, output json.RawMessage, usage json.RawMessage, completed time.Time,
	enrichedInput map[string]any,
) error {
	completed = completionTimeAtLeastClaim(run, completed)
	isReport := strings.HasPrefix(strings.TrimSpace(run.Capability), "report.")
	if isReport {
		title = reportArtifactTitle(run, title)
		// 注入必须读取富化后的 input(技能结论在执行期写入内存 map),原始
		// input_json 里没有评估块——读它等于永远注入空。
		output = injectCommitQualityIntoOutput(enrichedInput, output)
	}
	var options []*sql.TxOptions
	if strings.Contains(run.InputJSON, `"_document_sources"`) {
		options = append(options, &sql.TxOptions{Isolation: sql.LevelSerializable})
	}
	ctx, cancel := context.WithTimeout(context.Background(), w.sourceFreshnessTimeout)
	defer cancel()
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := w.verifyDocumentSources(ctx, tx, run, enrichedInput, true); err != nil {
			return err
		}
		artifact := aigorm.AIArtifact{WorkspaceID: run.WorkspaceID, RunID: run.ID, Kind: "structured_result", Title: title, ContentJSON: string(output)}
		if err := tx.Create(&artifact).Error; err != nil {
			return err
		}
		claim, err := claimedRunScope(tx.Model(&aigorm.AIRun{}), run)
		if err != nil {
			return err
		}
		updates := map[string]any{"status": "completed", "output_json": string(output), "completed_at": completed, "error_message": ""}
		// 报告运行把富化后的输入回写:逐提交 AI 结论与综合评估随输入持久化,
		// 导出与页面回溯才能看到完整证据链。
		if isReport && enrichedInput != nil {
			if enriched, err := json.Marshal(enrichedInput); err == nil {
				updates["input_json"] = string(enriched)
			}
		}
		updated := claim.Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errRunNotRunning
		}
		return recordUsage(tx, run, usage, completed)
	}, options...)
}

func validateRuntimeRequestSize(capability string, size int) error {
	requestLimit := maxRuntimeRequestBytes
	if capability == "qa.test_cases" {
		requestLimit = maxQATestCaseRuntimeRequestBytes
	}
	if size > requestLimit {
		return fmt.Errorf("AI runtime request exceeds the supported %d MiB limit; split the source material into smaller runs", requestLimit>>20)
	}
	return nil
}

func readRuntimeResponse(body io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxRuntimeResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxRuntimeResponseBytes {
		return nil, fmt.Errorf("AI runtime response exceeds the 4 MiB limit")
	}
	return raw, nil
}

func validateQATestCaseSkillProvenance(capability string, raw json.RawMessage) error {
	return qatestcase.ValidateProvenance(capability, raw)
}

// A runtime restart can accept the TCP connection and then close it before
// sending HTTP response headers. Retry that narrow class of transport failure
// once; provider errors and HTTP responses remain visible to the caller. This
// function owns the response body so its deadline remains active through EOF.
func (w *Worker) callRuntime(ctx context.Context, raw []byte) (int, []byte, error) {
	if w.cfg.AI.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.cfg.AI.RequestTimeout)
		defer cancel()
	}
	for attempt := 0; ; attempt++ {
		timestamp := strconv.FormatInt(w.now().UnixMilli(), 10)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.AI.RuntimeBaseURL+"/internal/v1/generate", bytes.NewReader(raw))
		if err != nil {
			return 0, nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-PM-Timestamp", timestamp)
		request.Header.Set("X-PM-Signature", sign(raw, timestamp, w.cfg.AI.InternalSecret))
		response, err := w.client.Do(request)
		if err == nil {
			responseRaw, readErr := readRuntimeResponse(response.Body)
			_ = response.Body.Close()
			if readErr != nil {
				return response.StatusCode, nil, readErr
			}
			return response.StatusCode, responseRaw, nil
		}
		if attempt > 0 || !retryableRuntimeTransportError(err) {
			return 0, nil, err
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return 0, nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func retryableRuntimeTransportError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection reset") ||
		strings.Contains(message, "connection refused") ||
		strings.Contains(message, "broken pipe")
}

// A cancelled run should release the single worker immediately instead of
// waiting for the upstream model timeout before the next queued run can start.
func (w *Worker) contextWhileRunIsActive(ctx context.Context, run *aigorm.AIRun) (context.Context, func()) {
	requestCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(runStatusPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-requestCtx.Done():
				return
			case <-ticker.C:
				active, err := w.runIsActive(run)
				if err != nil {
					continue
				}
				if !active {
					cancel()
					return
				}
			}
		}
	}()
	return requestCtx, func() {
		close(done)
		cancel()
	}
}

func (w *Worker) runIsActive(run *aigorm.AIRun) (bool, error) {
	var count int64
	claim, err := claimedRunScope(w.db.Model(&aigorm.AIRun{}), run)
	if err != nil {
		return false, err
	}
	err = claim.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

func (w *Worker) ensureRunClaimActive(run *aigorm.AIRun) error {
	active, err := w.runIsActive(run)
	if err == nil && !active {
		return errRunNotRunning
	}
	return nil
}

var errRunNotRunning = errors.New("AI run no longer belongs to this worker")

// started_at is the claim fence for a running AI job. A worker must only
// update the exact execution attempt it claimed, because a stale worker can
// outlive a restart and receive a late runtime response.
func claimedRunScope(db *gorm.DB, run *aigorm.AIRun) (*gorm.DB, error) {
	if run == nil || run.StartedAt == nil {
		return nil, errRunNotRunning
	}
	return db.Where(
		"id = ? AND workspace_id = ? AND status = ? AND started_at = ?",
		run.ID,
		run.WorkspaceID,
		"running",
		*run.StartedAt,
	), nil
}

// A queued row may have been claimed, cancelled, and requeued after it was
// read. Match its prior fence as well as its status so an old queue snapshot
// cannot overwrite a newer attempt.
func queuedRunScope(db *gorm.DB, run *aigorm.AIRun) *gorm.DB {
	claim := db.Where(
		"id = ? AND workspace_id = ? AND status = ?",
		run.ID,
		run.WorkspaceID,
		"queued",
	)
	if run.StartedAt == nil {
		return claim.Where("started_at IS NULL")
	}
	return claim.Where("started_at = ?", *run.StartedAt)
}

// GORM's default MySQL time column is DATETIME(3). Use a timestamp it can
// round-trip exactly so a worker's own fenced completion still matches there.
func normalizedAIRunClaimTime(value time.Time) time.Time {
	return value.UTC().Truncate(aiRunClaimTimePrecision)
}

// Keep started_at after a retry or recovery as the previous claim fence. The
// next claim must advance beyond every observable clock so a late worker can
// never match a newer attempt, even within one database millisecond.
func nextAIRunClaimTime(now time.Time, queuedUpdatedAt time.Time, previousStartedAt *time.Time) time.Time {
	latest := normalizedAIRunClaimTime(now)
	if queuedAt := normalizedAIRunClaimTime(queuedUpdatedAt); queuedAt.After(latest) {
		latest = queuedAt
	}
	if previousStartedAt != nil {
		if previousAt := normalizedAIRunClaimTime(*previousStartedAt); previousAt.After(latest) {
			latest = previousAt
		}
	}
	return latest.Add(aiRunClaimTimePrecision)
}

// A retry claim can intentionally be advanced by one database timestamp unit
// to fence off a previous worker attempt. A fast completion (or a clock that
// moves backwards) must still leave the run timestamps ordered.
func completionTimeAtLeastClaim(run *aigorm.AIRun, completed time.Time) time.Time {
	completed = completed.UTC()
	if run != nil && run.StartedAt != nil && completed.Before(*run.StartedAt) {
		return *run.StartedAt
	}
	return completed
}

func (w *Worker) failRunClaim(run *aigorm.AIRun, cause error) error {
	claim, err := claimedRunScope(w.db.Model(&aigorm.AIRun{}), run)
	if err != nil {
		return err
	}
	updated := claim.Updates(map[string]any{
		"status":        "failed",
		"error_message": safeError(cause),
		"completed_at":  completionTimeAtLeastClaim(run, w.now()),
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return errRunNotRunning
	}
	return nil
}

func (w *Worker) attachSources(ctx context.Context, run *aigorm.AIRun, input map[string]any) error {
	manifest, err := docsource.DecodeManifest(input)
	if err != nil {
		return err
	}
	if manifest != nil {
		return w.attachDocumentSources(ctx, run, input, manifest)
	}
	if err := w.rejectUnregisteredInternalSources(ctx, run, input); err != nil {
		return err
	}
	var rows []aigorm.AIRunSource
	if err := w.db.WithContext(ctx).Where("workspace_id = ? AND run_id = ?", run.WorkspaceID, run.ID).Find(&rows).Error; err != nil {
		return err
	}
	sources := make([]any, 0, len(rows))
	hasFeishuDocument := false
	requiresDocument := requiresFeishuDocument(run, input) || (run.Capability == "qa.test_cases" && len(rows) > 0)
	for _, row := range rows {
		if row.SourceType != "feishu_link" {
			return docsource.ErrUnavailable
		}
		if row.SourceType != "feishu_link" || !w.cfg.Feishu.Enabled || w.reader == nil {
			sources = append(sources, map[string]any{"type": row.SourceType, "url": row.SourceURL, "available": false})
			continue
		}
		document, err := w.reader.Read(ctx, row.SourceURL)
		if err != nil {
			if requiresDocument {
				return fmt.Errorf("read Feishu source: %w", err)
			}
			sources = append(sources, map[string]any{"type": row.SourceType, "url": row.SourceURL, "available": false})
			continue
		}
		if strings.TrimSpace(document.Content) == "" {
			if requiresDocument {
				return fmt.Errorf("飞书需求正文为空，无法执行本次 AI 分析")
			}
			sources = append(sources, map[string]any{"type": row.SourceType, "url": row.SourceURL, "available": false})
			continue
		}
		if document.ContentTruncated || document.BlockRefsTruncated {
			if requiresDocument {
				return fmt.Errorf("飞书需求正文或块引用超过读取上限，无法保证测试用例完整性；请拆分文档后重试")
			}
			sources = append(sources, map[string]any{
				"type": row.SourceType, "url": row.SourceURL, "available": false,
				"reason": "source_truncated",
			})
			continue
		}
		if run.Capability == "qa.test_cases" && (strings.TrimSpace(document.Revision) == "" || strings.TrimSpace(document.ContentHash) == "") {
			return fmt.Errorf("飞书需求来源缺少版本或内容哈希，无法建立生成前快照；请重试")
		}
		blockRefs, _ := json.Marshal(document.BlockRefs)
		if err := w.db.WithContext(ctx).Model(&aigorm.AIRunSource{}).Where("id = ? AND workspace_id = ?", row.ID, run.WorkspaceID).Updates(map[string]any{"revision": document.Revision, "content_hash": document.ContentHash, "block_refs": string(blockRefs)}).Error; err != nil {
			return err
		}
		sources = append(sources, map[string]any{
			"type": "feishu_document", "url": document.URL, "title": document.Title, "available": true,
			"revision": document.Revision, "content": document.Content,
			"original_content_chars": document.OriginalContentChars,
			"content_truncated":      document.ContentTruncated,
			"block_refs":             document.BlockRefs, "block_refs_truncated": document.BlockRefsTruncated,
		})
		hasFeishuDocument = true
	}
	input["sources"] = sources
	if requiresDocument && !hasFeishuDocument && w.cfg.Feishu.Enabled {
		return fmt.Errorf("飞书需求正文未读取，无法执行本次 AI 分析")
	}
	return nil
}

func requiresFeishuDocument(run *aigorm.AIRun, input map[string]any) bool {
	if run.Capability == "qa.test_cases" {
		context, _ := input["test_case_context"].(map[string]any)
		requirements, _ := context["requirements"].([]any)
		for _, value := range requirements {
			requirement, _ := value.(map[string]any)
			url, _ := requirement["product_doc_url"].(string)
			if strings.TrimSpace(url) != "" {
				return true
			}
		}
		return false
	}
	if run.Capability != "requirement.completeness" && run.Capability != "requirement.task_plan" {
		return false
	}
	subject, ok := input["subject"].(map[string]any)
	if !ok {
		return false
	}
	url, _ := subject["product_doc_url"].(string)
	return strings.TrimSpace(url) != ""
}

func sign(body []byte, timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// reportArtifactTitle derives a formal artifact title from the run's own
// subject facts (【日报】成员 日期)；report summaries are full markdown
// documents, so the narrative itself is not a usable title.
func reportArtifactTitle(run *aigorm.AIRun, fallback string) string {
	var input struct {
		Subject struct {
			Target struct {
				Name string `json:"name"`
			} `json:"target"`
			Period struct {
				AnchorDate string `json:"anchor_date"`
			} `json:"period"`
		} `json:"subject"`
	}
	if json.Unmarshal([]byte(run.InputJSON), &input) == nil && input.Subject.Target.Name != "" {
		label := "工作汇报"
		switch {
		case strings.Contains(run.Capability, "weekly"):
			label = "周报"
		case strings.Contains(run.Capability, "monthly"):
			label = "月报"
		case strings.Contains(run.Capability, "daily"):
			label = "日报"
		}
		return fmt.Sprintf("【%s】%s %s", label, input.Subject.Target.Name, input.Subject.Period.AnchorDate)
	}
	for _, line := range strings.Split(fallback, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimLeft(line, "# ")
		if len([]rune(line)) > 120 {
			line = string([]rune(line)[:120]) + "…"
		}
		return line
	}
	return "工作汇报"
}

func validateChanges(capability string, changes []struct {
	Field string `json:"field"`
}) error {
	// Reporting skills produce a narrative only; the runtime may still echo
	// input fields, and none of them are applicable PM field changes.
	if strings.HasPrefix(strings.TrimSpace(capability), "report.") {
		return nil
	}
	allowlists := map[string]map[string]bool{
		"requirement.draft":               {"title": true, "description": true, "acceptance_criteria": true},
		"requirement.triage":              {"priority": true},
		"requirement.acceptance_criteria": {"acceptance_criteria": true},
	}
	allowed := allowlists[capability]
	for _, change := range changes {
		if allowed == nil || !allowed[change.Field] {
			return fmt.Errorf("AI output attempted forbidden field %q", change.Field)
		}
	}
	return nil
}

func recordUsage(tx *gorm.DB, run *aigorm.AIRun, raw json.RawMessage, completed time.Time) error {
	var usage struct {
		InputTokens  int64 `json:"inputTokens"`
		OutputTokens int64 `json:"outputTokens"`
	}
	_ = json.Unmarshal(raw, &usage)
	day := completed.Format("2006-01-02")
	var row aigorm.AIUsageDaily
	err := tx.Where("workspace_id = ? AND usage_date = ? AND model = ?", run.WorkspaceID, day, run.Model).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return tx.Create(&aigorm.AIUsageDaily{WorkspaceID: run.WorkspaceID, UsageDate: day, Model: run.Model, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, RunCount: 1}).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&row).Updates(map[string]any{"input_tokens": gorm.Expr("input_tokens + ?", usage.InputTokens), "output_tokens": gorm.Expr("output_tokens + ?", usage.OutputTokens), "run_count": gorm.Expr("run_count + 1")}).Error
}

func safeError(err error) string {
	value := err.Error()
	if len(value) > 500 {
		return value[:500]
	}
	return value
}
