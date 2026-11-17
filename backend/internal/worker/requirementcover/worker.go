package requirementcover

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	requirementcovermodule "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const (
	thumbnailWidth       = 640
	thumbnailHeight      = 320
	thumbnailMaxBytes    = 2 * 1024 * 1024
	jobClaimTTL          = 2 * time.Minute
	jobPollInterval      = time.Second
	coverReadTimeout     = 35 * time.Second
	maxCoverRetryAttempt = 5
)

type objectStore interface {
	PutObject(context.Context, string, io.Reader, int64, string) error
	RemoveObject(context.Context, string) error
}

type Worker struct {
	db       *gorm.DB
	cfg      config.Config
	reader   feishu.CoverReader
	store    objectStore
	workerID string
	now      func() time.Time
}

func NewWorker(db *gorm.DB, cfg config.Config, reader feishu.CoverReader, store objectStore) (*Worker, error) {
	if db == nil || reader == nil || store == nil {
		return nil, errors.New("requirement cover worker dependencies are unavailable")
	}
	if !cfg.Feishu.Enabled || !cfg.ObjectStorage.Enabled {
		return nil, errors.New("requirement cover worker requires Feishu and object storage")
	}
	identifier := make([]byte, 8)
	if _, err := rand.Read(identifier); err != nil {
		return nil, fmt.Errorf("create worker id: %w", err)
	}
	return &Worker{
		db: db, cfg: cfg, reader: reader, store: store,
		workerID: "requirement-cover-" + hex.EncodeToString(identifier),
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()
	for {
		processed, err := w.RunOnce(ctx)
		if err != nil {
			fmt.Printf("requirement cover worker: %v\n", err)
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	job, ok, err := w.claimNext()
	if err != nil || !ok {
		return false, err
	}
	if job.SourceHash == "" || job.Status != requirementcovermodule.StatusProcessing {
		return true, w.completeDisabled(job)
	}
	return true, w.generate(ctx, job)
}

func (w *Worker) claimNext() (requirementgorm.RequirementCover, bool, error) {
	now := w.now()
	staleBefore := now.Add(-jobClaimTTL)
	var job requirementgorm.RequirementCover
	err := w.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where(
			"status IN ? AND next_attempt_at <= ? AND (claimed_at IS NULL OR claimed_at < ?)",
			[]string{requirementcovermodule.StatusPending, requirementcovermodule.StatusProcessing}, now, staleBefore,
		).Order("next_attempt_at, id").Limit(1)
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := query.First(&job).Error; err != nil {
			return err
		}
		result := tx.Model(&requirementgorm.RequirementCover{}).Where(
			"id = ? AND status IN ? AND (claimed_at IS NULL OR claimed_at < ?)",
			job.ID, []string{requirementcovermodule.StatusPending, requirementcovermodule.StatusProcessing}, staleBefore,
		).Updates(map[string]any{
			"status": requirementcovermodule.StatusProcessing, "claimed_at": now, "claimed_by": w.workerID,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		job.Status, job.ClaimedAt, job.ClaimedBy = requirementcovermodule.StatusProcessing, &now, w.workerID
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return requirementgorm.RequirementCover{}, false, nil
	}
	return job, err == nil, err
}

func (w *Worker) generate(ctx context.Context, job requirementgorm.RequirementCover) error {
	var requirement requirementgorm.Requirement
	if err := w.db.Where("id = ? AND workspace_id = ?", job.RequirementID, job.WorkspaceID).
		First(&requirement).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return w.completeDisabled(job)
		}
		return w.retry(job, err)
	}
	normalizedURL, eligible := requirementcovermodule.EligibleURL(requirement.ProductDocURL, w.cfg)
	if !eligible || requirementcovermodule.SourceHash(normalizedURL) != job.SourceHash {
		return w.discardStale(job)
	}

	readCtx, cancel := context.WithTimeout(ctx, coverReadTimeout)
	cover, err := w.reader.ReadCover(readCtx, normalizedURL)
	cancel()
	if err != nil {
		return w.retry(job, err)
	}
	thumbnail, err := makeThumbnail(cover.Content)
	if err != nil {
		return w.retry(job, err)
	}
	objectKey := buildObjectKey(w.cfg.ObjectStorage, job)
	if err := w.store.PutObject(ctx, objectKey, bytes.NewReader(thumbnail), int64(len(thumbnail)), "image/jpeg"); err != nil {
		return w.retry(job, fmt.Errorf("store thumbnail: %w", err))
	}
	if err := w.publish(job, objectKey, int64(len(thumbnail))); err != nil {
		_ = w.store.RemoveObject(context.WithoutCancel(ctx), objectKey)
		return err
	}
	if job.ObjectKey != "" && job.ObjectKey != objectKey {
		_ = w.store.RemoveObject(context.WithoutCancel(ctx), job.ObjectKey)
	}
	return nil
}

func (w *Worker) publish(job requirementgorm.RequirementCover, objectKey string, size int64) error {
	now := w.now()
	result := w.db.Model(&requirementgorm.RequirementCover{}).Where(
		"id = ? AND workspace_id = ? AND generation = ? AND source_hash = ? AND status = ? AND claimed_by = ?",
		job.ID, job.WorkspaceID, job.Generation, job.SourceHash, requirementcovermodule.StatusProcessing, w.workerID,
	).Updates(map[string]any{
		"status": requirementcovermodule.StatusReady, "object_key": objectKey, "content_type": "image/jpeg", "size": size,
		"width": thumbnailWidth, "height": thumbnailHeight, "generated_at": now,
		"claimed_at": nil, "claimed_by": "", "last_error": "", "next_attempt_at": now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("requirement cover job changed before publish")
	}
	return nil
}

func (w *Worker) retry(job requirementgorm.RequirementCover, cause error) error {
	attempts := job.Attempts + 1
	status := requirementcovermodule.StatusPending
	delay := retryDelay(attempts)
	if attempts >= maxCoverRetryAttempt {
		status, delay = requirementcovermodule.StatusFailed, 0
	}
	now := w.now()
	result := w.db.Model(&requirementgorm.RequirementCover{}).Where(
		"id = ? AND generation = ? AND source_hash = ? AND status = ? AND claimed_by = ?",
		job.ID, job.Generation, job.SourceHash, requirementcovermodule.StatusProcessing, w.workerID,
	).Updates(map[string]any{
		"status": status, "attempts": attempts, "next_attempt_at": now.Add(delay),
		"claimed_at": nil, "claimed_by": "", "last_error": safeError(cause),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 && status == requirementcovermodule.StatusFailed && job.ObjectKey != "" {
		_ = w.store.RemoveObject(context.Background(), job.ObjectKey)
	}
	return nil
}

func (w *Worker) discardStale(job requirementgorm.RequirementCover) error {
	return w.db.Model(&requirementgorm.RequirementCover{}).Where(
		"id = ? AND generation = ? AND status = ? AND claimed_by = ?",
		job.ID, job.Generation, requirementcovermodule.StatusProcessing, w.workerID,
	).Updates(map[string]any{"status": requirementcovermodule.StatusPending, "claimed_at": nil, "claimed_by": ""}).Error
}

func (w *Worker) completeDisabled(job requirementgorm.RequirementCover) error {
	if job.ObjectKey != "" {
		_ = w.store.RemoveObject(context.Background(), job.ObjectKey)
	}
	return w.db.Model(&requirementgorm.RequirementCover{}).Where(
		"id = ? AND generation = ? AND source_hash = ? AND status = ? AND claimed_by = ?",
		job.ID, job.Generation, "", requirementcovermodule.StatusProcessing, w.workerID,
	).Updates(map[string]any{
		"status": requirementcovermodule.StatusDisabled, "claimed_at": nil, "claimed_by": "", "object_key": "",
		"content_type": "", "size": 0, "width": 0, "height": 0,
	}).Error
}

func makeThumbnail(content []byte) ([]byte, error) {
	if len(content) == 0 || len(content) > 8*1024*1024 {
		return nil, errors.New("document cover image is invalid")
	}
	source, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("decode document cover: %w", err)
	}
	if source.Bounds().Dx() != 1280 || source.Bounds().Dy() != 640 {
		return nil, errors.New("document cover dimensions are invalid")
	}
	target := image.NewRGBA(image.Rect(0, 0, thumbnailWidth, thumbnailHeight))
	xdraw.CatmullRom.Scale(target, target.Bounds(), source, source.Bounds(), xdraw.Src, nil)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, target, &jpeg.Options{Quality: 82}); err != nil {
		return nil, fmt.Errorf("encode document cover thumbnail: %w", err)
	}
	if output.Len() == 0 || output.Len() > thumbnailMaxBytes {
		return nil, errors.New("document cover thumbnail is invalid")
	}
	return output.Bytes(), nil
}

func buildObjectKey(cfg config.ObjectStorage, job requirementgorm.RequirementCover) string {
	parts := make([]string, 0, 6)
	if prefix := strings.Trim(strings.TrimSpace(cfg.GlobalPrefix), "/"); prefix != "" {
		parts = append(parts, prefix)
	}
	parts = append(parts, fmt.Sprintf("workspace/%d", job.WorkspaceID), "requirement-covers",
		fmt.Sprintf("requirement/%d", job.RequirementID), fmt.Sprintf("%s-g%d.jpg", job.SourceHash[:16], job.Generation))
	return filepath.ToSlash(strings.Join(parts, "/"))
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 5 * time.Second
	case 2:
		return 30 * time.Second
	case 3:
		return 2 * time.Minute
	default:
		return 10 * time.Minute
	}
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	return "cover generation failed"
}
