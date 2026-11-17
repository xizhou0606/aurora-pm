// Package designreview consumes visual and deep design-review jobs inside the
// existing pm-worker process.
package designreview

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	domain "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
	designreviewgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type Worker struct {
	repository *designreviewgorm.Repository
	objects    domain.ObjectStore
	cfg        config.Config
	client     *http.Client
	owner      string
	now        func() time.Time
}

func New(db *gorm.DB, cfg config.Config) *Worker {
	objects, _ := objectstore.New(cfg.ObjectStorage)
	return &Worker{repository: designreviewgorm.NewRepository(db), objects: objects, cfg: cfg, client: &http.Client{Timeout: cfg.AI.RequestTimeout}, owner: "pm-worker-design-review-" + strconv.FormatInt(time.Now().UnixNano(), 36), now: func() time.Time { return time.Now().UTC() }}
}

func (worker *Worker) Run(ctx context.Context, pollInterval time.Duration) {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = worker.RunOnce(ctx)
		}
	}
}

func (worker *Worker) RunOnce(ctx context.Context) (bool, error) {
	maintenance := false
	if worker.objects != nil {
		cleanup, err := worker.repository.ClaimObjectCleanup(ctx, worker.owner, time.Minute)
		if err != nil {
			return false, err
		}
		if cleanup != nil {
			if err := worker.objects.RemoveObject(ctx, cleanup.ObjectKey); err != nil {
				retryAt := worker.now().Add(objectCleanupRetryDelay(cleanup.Attempts))
				_ = worker.repository.FailObjectCleanup(context.WithoutCancel(ctx), cleanup.ID, worker.owner, err.Error(), retryAt)
				return true, err
			}
			if err := worker.repository.CompleteObjectCleanup(ctx, cleanup.WorkspaceID, cleanup.ObjectKey); err != nil {
				return true, err
			}
			maintenance = true
		}
	}
	expiredObjects, err := worker.repository.ExpireCaptureDrafts(ctx, worker.now())
	if err != nil {
		return false, err
	}
	for _, expired := range expiredObjects {
		if worker.objects == nil {
			break
		}
		if err := worker.objects.RemoveObject(ctx, expired.ObjectKey); err != nil {
			return true, err
		}
		if err := worker.repository.CompleteCaptureObjectCleanup(ctx, expired.DraftID, worker.now()); err != nil {
			return true, err
		}
		maintenance = true
	}
	if err := worker.repository.PurgeExpiredCaptureState(ctx, worker.now()); err != nil {
		return maintenance, err
	}
	deletion, err := worker.repository.NextDeletion(ctx)
	if err != nil {
		return maintenance, err
	}
	if deletion != nil {
		if worker.objects == nil && len(deletion.ObjectKeys) > 0 {
			return maintenance, nil
		}
		for _, key := range deletion.ObjectKeys {
			if err := worker.objects.RemoveObject(ctx, key); err != nil {
				return true, err
			}
		}
		return true, worker.repository.PurgeDeletion(ctx, *deletion)
	}
	if !worker.cfg.AI.Enabled || strings.TrimSpace(worker.cfg.AI.DesignReviewModel) == "" {
		return maintenance, nil
	}
	job, err := worker.repository.ClaimJob(ctx, worker.owner, 2*worker.cfg.AI.RequestTimeout)
	if err != nil || job == nil {
		return maintenance, err
	}
	if err := worker.execute(ctx, *job); err != nil {
		_ = worker.repository.FailJob(context.WithoutCancel(ctx), *job, err, retryableJobError(err))
		return true, err
	}
	return true, nil
}

type runtimeEvidence struct {
	Kind        string `json:"kind"`
	ContentType string `json:"contentType"`
	SHA256      string `json:"sha256"`
	Base64      string `json:"base64"`
	raw         []byte
}

type runtimeDifference struct {
	Category             string         `json:"category"`
	Severity             string         `json:"severity"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	Recommendation       string         `json:"recommendation"`
	Confidence           float64        `json:"confidence"`
	DesignRegion         *domain.Region `json:"designRegion"`
	ImplementationRegion *domain.Region `json:"implementationRegion"`
	EvidenceNotes        string         `json:"evidenceNotes"`
}

type runtimeHTTPError struct {
	status int
	code   string
	fields []string
}

func (err runtimeHTTPError) Error() string {
	message := fmt.Sprintf("design review runtime returned %d", err.status)
	if err.code != "" {
		message += ": " + err.code
	}
	if len(err.fields) > 0 {
		message += " (fields: " + strings.Join(err.fields, ", ") + ")"
	}
	return message
}

func newRuntimeHTTPError(status int, raw []byte) runtimeHTTPError {
	err := runtimeHTTPError{status: status}
	var envelope struct {
		Error   string `json:"error"`
		Details struct {
			FieldErrors map[string][]string `json:"fieldErrors"`
		} `json:"details"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return err
	}
	if code := strings.TrimSpace(envelope.Error); len(code) <= 160 {
		err.code = code
	}
	for field := range envelope.Details.FieldErrors {
		if field != "" && len(field) <= 80 {
			err.fields = append(err.fields, field)
		}
	}
	sort.Strings(err.fields)
	return err
}

func retryableJobError(err error) bool {
	var responseError runtimeHTTPError
	if errors.As(err, &responseError) {
		return responseError.status == http.StatusRequestTimeout || responseError.status == http.StatusTooManyRequests || responseError.status >= 500
	}
	return !errors.Is(err, domain.ErrInvalid) && !errors.Is(err, domain.ErrEvidenceRequired) &&
		!errors.Is(err, domain.ErrNotFound) && !errors.Is(err, domain.ErrConflict)
}

func (difference runtimeDifference) domainInput() domain.DifferenceInput {
	return domain.DifferenceInput{Category: difference.Category, Severity: difference.Severity, Title: difference.Title,
		Description: difference.Description, Recommendation: difference.Recommendation, Confidence: difference.Confidence,
		DesignRegion: difference.DesignRegion, ImplementationRegion: difference.ImplementationRegion, EvidenceNotes: difference.EvidenceNotes}
}

func (worker *Worker) execute(ctx context.Context, job designreviewgorm.ClaimedJob) error {
	detail, err := worker.repository.GetReview(ctx, job.WorkspaceID, job.ReviewID)
	if err != nil {
		return err
	}
	design, err := worker.loadEvidence(ctx, job.WorkspaceID, detail, "design_normalized", "design_original", "design")
	if err != nil {
		return err
	}
	implementation, err := worker.loadEvidence(ctx, job.WorkspaceID, detail, "implementation_normalized", "implementation_original", "implementation")
	if err != nil {
		return err
	}
	designStructure, err := worker.loadJSONEvidence(ctx, job.WorkspaceID, detail, "design_structure")
	if err != nil {
		return err
	}
	if err := domain.ValidateDesignStructure(designStructure); err != nil {
		return err
	}
	implementationStructure, err := worker.loadJSONEvidence(ctx, job.WorkspaceID, detail, "implementation_structure")
	if err != nil {
		return err
	}
	if _, _, err := domain.ValidateImplementationStructure(implementationStructure, detail.ImplementationKind, detail.ViewportWidth, detail.ViewportHeight); err != nil {
		return err
	}
	var normalizationManifest []byte
	if domain.DOMNormalizationEnabled(detail.DOMNormalization) {
		normalizationManifest, err = worker.loadJSONEvidence(ctx, job.WorkspaceID, detail, "dom_normalization_manifest")
		if err != nil {
			return err
		}
		if _, err := domain.ParseDOMNormalizationManifest(normalizationManifest); err != nil {
			return err
		}
	}
	if job.Kind == "full" && detail.DimensionsMatch {
		if err := worker.saveHeatmap(ctx, job, design.raw, implementation.raw); err != nil {
			return err
		}
	}
	scope := map[string]any{"enabled": detail.AnalysisScope.Enabled, "focus": ""}
	if detail.AnalysisScope.DesignRegion != nil {
		scope["designRegion"] = detail.AnalysisScope.DesignRegion
	}
	if detail.AnalysisScope.ImplementationRegion != nil {
		scope["implementationRegion"] = detail.AnalysisScope.ImplementationRegion
	}
	if job.Kind == "deep" && len(job.Payload) > 0 {
		var payload map[string]any
		if json.Unmarshal(job.Payload, &payload) == nil {
			scope["enabled"] = true
			scope["designRegion"] = payload["design_region"]
			scope["implementationRegion"] = payload["implementation_region"]
			scope["focus"] = payload["focus"]
		}
	}
	payload := map[string]any{
		"manifest": map[string]any{"version": "1.0", "reviewId": job.ReviewID, "workspaceId": job.WorkspaceID, "kind": job.Kind, "model": worker.cfg.AI.DesignReviewModel, "reasoningEffort": worker.cfg.AI.DesignReviewReasoningEffort, "imageDetail": worker.cfg.AI.DesignReviewImageDetail},
		"evidence": []runtimeEvidence{design, implementation}, "scope": scope,
		"designStructure": json.RawMessage(designStructure), "implementationStructure": json.RawMessage(implementationStructure),
	}
	if len(normalizationManifest) > 0 {
		payload["normalizationManifest"] = json.RawMessage(normalizationManifest)
	}
	var recheckSnapshot *domain.RecheckSnapshot
	if job.Kind == "full" && detail.SourceReviewID != nil {
		snapshot, snapshotErr := worker.repository.BuildRecheckSnapshot(ctx, job.WorkspaceID, job.ReviewID)
		if snapshotErr != nil {
			return snapshotErr
		}
		recheckSnapshot = &snapshot
		payload["recheck"] = snapshot
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, worker.cfg.AI.RuntimeBaseURL+"/internal/v1/design-review/analyze", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(worker.now().UnixMilli(), 10)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-PM-Timestamp", timestamp)
	request.Header.Set("X-PM-Signature", sign(raw, timestamp, worker.cfg.AI.InternalSecret))
	response, err := worker.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseRaw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return newRuntimeHTTPError(response.StatusCode, responseRaw)
	}
	var envelope struct {
		Data struct {
			Model           string          `json:"model"`
			ReasoningEffort string          `json:"reasoningEffort"`
			PromptVersion   string          `json:"promptVersion"`
			ResponseID      string          `json:"responseId"`
			DurationMS      int             `json:"durationMs"`
			Output          json.RawMessage `json:"output"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseRaw, &envelope); err != nil || len(envelope.Data.Output) == 0 {
		return domain.ErrInvalid
	}
	metadata := domain.AnalysisRunMetadata{
		Model: envelope.Data.Model, ReasoningEffort: envelope.Data.ReasoningEffort,
		PromptVersion: envelope.Data.PromptVersion, ResponseID: envelope.Data.ResponseID,
		DurationMS: envelope.Data.DurationMS,
	}
	expectedPromptVersion := "design-review-v1"
	if recheckSnapshot != nil {
		expectedPromptVersion = "design-recheck-v1"
	}
	if !metadata.Valid() || metadata.ReasoningEffort != worker.cfg.AI.DesignReviewReasoningEffort || metadata.PromptVersion != expectedPromptVersion {
		return domain.ErrInvalid
	}
	if recheckSnapshot != nil {
		var output domain.RecheckAnalysis
		if json.Unmarshal(envelope.Data.Output, &output) != nil || output.Validate(*recheckSnapshot) != nil {
			return domain.ErrInvalid
		}
		for index := range output.CurrentDifferences {
			candidate := &output.CurrentDifferences[index]
			candidate.DesignAnchor, candidate.ImplementationAnchor = domain.DeriveIdentityAnchors(designStructure, implementationStructure, candidate.DesignRegion, candidate.ImplementationRegion, detail.ViewportWidth, detail.ViewportHeight)
		}
		return worker.repository.CompleteRecheckJob(ctx, domain.ClaimedAnalysisJob{ID: job.ID, WorkspaceID: job.WorkspaceID, ReviewID: job.ReviewID}, *recheckSnapshot, output, metadata)
	}
	var output struct {
		Summary               string              `json:"summary"`
		ModelCoverageComplete bool                `json:"modelCoverageComplete"`
		Differences           []runtimeDifference `json:"differences"`
	}
	if json.Unmarshal(envelope.Data.Output, &output) != nil || strings.TrimSpace(output.Summary) == "" || len(output.Differences) > 100 {
		return domain.ErrInvalid
	}
	differences := make([]domain.AnalyzedDifference, 0, len(output.Differences))
	for _, difference := range output.Differences {
		input := difference.domainInput()
		if !input.Valid() {
			return domain.ErrInvalid
		}
		designAnchor, implementationAnchor := domain.DeriveIdentityAnchors(designStructure, implementationStructure, input.DesignRegion, input.ImplementationRegion, detail.ViewportWidth, detail.ViewportHeight)
		differences = append(differences, domain.AnalyzedDifference{Input: input, DesignAnchor: designAnchor, ImplementationAnchor: implementationAnchor})
	}
	return worker.repository.CompleteJob(ctx, job, output.Summary, output.ModelCoverageComplete, differences, metadata)
}

func (worker *Worker) loadEvidence(ctx context.Context, workspaceID uint, detail domain.DesignReviewDetail, preferred, fallback, publicKind string) (runtimeEvidence, error) {
	var selected *domain.EvidenceAsset
	for index := range detail.Assets {
		asset := &detail.Assets[index]
		if asset.Kind == preferred || (selected == nil && asset.Kind == fallback) {
			selected = asset
			if asset.Kind == preferred {
				break
			}
		}
	}
	if selected == nil {
		return runtimeEvidence{}, domain.ErrEvidenceRequired
	}
	record, err := worker.repository.GetAsset(ctx, workspaceID, detail.ID, selected.ID)
	if err != nil {
		return runtimeEvidence{}, err
	}
	reader, err := worker.objects.GetObject(ctx, record.ObjectKey)
	if err != nil {
		return runtimeEvidence{}, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, (20<<20)+1))
	if err != nil || len(raw) == 0 || len(raw) > 20<<20 {
		return runtimeEvidence{}, domain.ErrInvalid
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != record.SHA256 {
		return runtimeEvidence{}, fmt.Errorf("%w: evidence integrity mismatch", domain.ErrInvalid)
	}
	return runtimeEvidence{Kind: publicKind, ContentType: record.ContentType, SHA256: record.SHA256, Base64: base64.StdEncoding.EncodeToString(raw), raw: raw}, nil
}

func (worker *Worker) loadJSONEvidence(ctx context.Context, workspaceID uint, detail domain.DesignReviewDetail, kind string) ([]byte, error) {
	var selected *domain.EvidenceAsset
	for index := range detail.Assets {
		if detail.Assets[index].Kind == kind {
			selected = &detail.Assets[index]
			break
		}
	}
	if selected == nil {
		return nil, domain.ErrEvidenceRequired
	}
	record, err := worker.repository.GetAsset(ctx, workspaceID, detail.ID, selected.ID)
	if err != nil {
		return nil, err
	}
	if record.ContentType != "application/json" {
		return nil, domain.ErrInvalid
	}
	reader, err := worker.objects.GetObject(ctx, record.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, (20<<20)+1))
	if err != nil || len(raw) == 0 || len(raw) > 20<<20 || !json.Valid(raw) {
		return nil, domain.ErrInvalid
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != record.SHA256 {
		return nil, fmt.Errorf("%w: evidence integrity mismatch", domain.ErrInvalid)
	}
	return raw, nil
}

func (worker *Worker) saveHeatmap(ctx context.Context, job designreviewgorm.ClaimedJob, designRaw, implementationRaw []byte) error {
	designImage, _, err := image.Decode(bytes.NewReader(designRaw))
	if err != nil {
		return err
	}
	implementationImage, _, err := image.Decode(bytes.NewReader(implementationRaw))
	if err != nil {
		return err
	}
	comparison, err := domain.Heatmap(designImage, implementationImage, 12)
	if err != nil {
		return err
	}
	id := newWorkerID()
	objectKey := fmt.Sprintf("workspace/%d/design-reviews/%s/%s.png", job.WorkspaceID, job.ReviewID, id)
	if err := worker.objects.PutObject(ctx, objectKey, bytes.NewReader(comparison.PNG), int64(len(comparison.PNG)), "image/png"); err != nil {
		return err
	}
	sum := sha256.Sum256(comparison.PNG)
	width, height := designImage.Bounds().Dx(), designImage.Bounds().Dy()
	oldKey, err := worker.repository.SaveAsset(ctx, domain.AssetRecord{ID: id, WorkspaceID: job.WorkspaceID, ReviewID: job.ReviewID, Kind: "pixel_heatmap", ObjectKey: objectKey, ContentType: "image/png", ByteSize: int64(len(comparison.PNG)), SHA256: hex.EncodeToString(sum[:]), Width: &width, Height: &height, CreatedAt: worker.now()})
	if err != nil {
		_ = worker.objects.RemoveObject(context.WithoutCancel(ctx), objectKey)
		return err
	}
	if oldKey != nil && *oldKey != objectKey {
		cleanupCtx := context.WithoutCancel(ctx)
		if removeErr := worker.objects.RemoveObject(cleanupCtx, *oldKey); removeErr == nil {
			_ = worker.repository.CompleteObjectCleanup(cleanupCtx, job.WorkspaceID, *oldKey)
		}
	}
	return worker.repository.SetHeatmapRatio(ctx, job.WorkspaceID, job.ReviewID, comparison.DifferenceRatio)
}

func objectCleanupRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 6 {
		attempts = 6
	}
	return time.Duration(1<<(attempts-1)) * time.Minute
}

func newWorkerID() string {
	return uuid.NewString()
}

func sign(raw []byte, timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}
