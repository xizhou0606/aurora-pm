package feishu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

const (
	documentCoverExportPollInterval  = time.Second
	documentCoverExportTimeout       = 20 * time.Second
	documentCoverExportStartInterval = time.Minute / 100
)

var (
	ErrDocumentCoverExportFailed        = errors.New("feishu document cover export failed")
	ErrDocumentCoverExportTimeout       = errors.New("feishu document cover export timed out")
	ErrDocumentCoverRendererUnavailable = errors.New("feishu document cover renderer unavailable")

	globalDocumentCoverExportCoordinator = newDocumentCoverExportCoordinator(
		documentCoverExportStartInterval,
	)
)

type documentCoverExportCoordinator struct {
	slot          chan struct{}
	mu            sync.Mutex
	nextStart     time.Time
	startInterval time.Duration
}

func newDocumentCoverExportCoordinator(startInterval time.Duration) *documentCoverExportCoordinator {
	return &documentCoverExportCoordinator{
		slot: make(chan struct{}, 1), startInterval: max(0, startInterval),
	}
}

func (c *documentCoverExportCoordinator) acquire(ctx context.Context) error {
	select {
	case c.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *documentCoverExportCoordinator) waitForStart(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	startAt := now
	if c.nextStart.After(now) {
		startAt = c.nextStart
	}
	if delay := time.Until(startAt); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.nextStart = time.Now().Add(c.startInterval)
	return nil
}

func (c *documentCoverExportCoordinator) release() {
	<-c.slot
}

func (r *SDKReader) exportDocumentCover(ctx context.Context, documentID string) (DocumentCover, error) {
	coordinator := r.documentCoverExportCoordinatorValue()
	if err := coordinator.acquire(ctx); err != nil {
		return DocumentCover{}, err
	}
	defer coordinator.release()
	exportCtx, cancel := context.WithTimeout(ctx, r.documentCoverExportTimeoutValue())
	defer cancel()
	if err := coordinator.waitForStart(exportCtx); err != nil {
		return DocumentCover{}, documentCoverExportContextError(err)
	}

	created, err := r.client.Drive.V1.ExportTask.Create(exportCtx,
		larkdrive.NewCreateExportTaskReqBuilder().ExportTask(
			larkdrive.NewExportTaskBuilder().FileExtension("pdf").Token(documentID).Type("docx").Build(),
		).Build())
	if err != nil {
		return DocumentCover{}, documentCoverExportCallError(exportCtx, fmt.Errorf("create document export: %w", err))
	}
	if created == nil || !created.Success() || created.Data == nil || created.Data.Ticket == nil ||
		strings.TrimSpace(*created.Data.Ticket) == "" {
		return DocumentCover{}, fmt.Errorf("create document export failed")
	}

	ticker := time.NewTicker(r.documentCoverExportPollIntervalValue())
	defer ticker.Stop()
	for {
		select {
		case <-exportCtx.Done():
			return DocumentCover{}, documentCoverExportContextError(exportCtx.Err())
		case <-ticker.C:
		}
		result, pollErr := r.client.Drive.V1.ExportTask.Get(exportCtx,
			larkdrive.NewGetExportTaskReqBuilder().Ticket(strings.TrimSpace(*created.Data.Ticket)).
				Token(documentID).Build())
		if pollErr != nil {
			return DocumentCover{}, documentCoverExportCallError(exportCtx, fmt.Errorf("poll document export: %w", pollErr))
		}
		if result == nil || !result.Success() || result.Data == nil || result.Data.Result == nil {
			return DocumentCover{}, fmt.Errorf("poll document export failed")
		}
		export := result.Data.Result
		if export.JobStatus == nil {
			continue
		}
		switch *export.JobStatus {
		case 0:
			return r.downloadDocumentCoverExport(exportCtx, export)
		case 1, 2:
			continue
		default:
			return DocumentCover{}, ErrDocumentCoverExportFailed
		}
	}
}

func (r *SDKReader) downloadDocumentCoverExport(
	ctx context.Context,
	export *larkdrive.ExportTask,
) (DocumentCover, error) {
	if export.FileToken == nil || strings.TrimSpace(*export.FileToken) == "" ||
		(export.FileSize != nil && (*export.FileSize < 0 || int64(*export.FileSize) > documentCoverMaxBytes)) {
		return DocumentCover{}, ErrDocumentCoverExportFailed
	}
	download, err := r.client.Drive.V1.ExportTask.Download(ctx,
		larkdrive.NewDownloadExportTaskReqBuilder().FileToken(strings.TrimSpace(*export.FileToken)).Build())
	if err != nil {
		return DocumentCover{}, documentCoverExportCallError(ctx, fmt.Errorf("download document export: %w", err))
	}
	if download == nil || !download.Success() || download.File == nil {
		return DocumentCover{}, fmt.Errorf("download document export failed")
	}
	content, err := io.ReadAll(io.LimitReader(download.File, documentCoverMaxBytes+1))
	if err != nil {
		return DocumentCover{}, fmt.Errorf("read document export: %w", err)
	}
	if len(content) == 0 || int64(len(content)) > documentCoverMaxBytes ||
		strings.ToLower(http.DetectContentType(content)) != "application/pdf" {
		return DocumentCover{}, fmt.Errorf("document export content is invalid")
	}
	renderer := r.documentCoverRendererValue()
	if renderer == nil {
		return DocumentCover{}, ErrDocumentCoverRendererUnavailable
	}
	firstPage, err := renderer.RenderFirstPage(ctx, content)
	if err != nil {
		return DocumentCover{}, fmt.Errorf("render document cover page: %w", err)
	}
	if err := validateRenderedDocumentCover(firstPage); err != nil {
		return DocumentCover{}, err
	}
	return DocumentCover{Content: firstPage, ContentType: "image/png"}, nil
}

func (r *SDKReader) documentCoverRendererValue() DocumentCoverRenderer {
	return r.coverRenderer
}

func (r *SDKReader) documentCoverExportCoordinatorValue() *documentCoverExportCoordinator {
	if r.coverExportCoordinator != nil {
		return r.coverExportCoordinator
	}
	return globalDocumentCoverExportCoordinator
}

func (r *SDKReader) documentCoverExportPollIntervalValue() time.Duration {
	if r.coverExportPollInterval > 0 {
		return r.coverExportPollInterval
	}
	return documentCoverExportPollInterval
}

func (r *SDKReader) documentCoverExportTimeoutValue() time.Duration {
	if r.coverExportTimeout > 0 {
		return r.coverExportTimeout
	}
	return documentCoverExportTimeout
}

func documentCoverExportContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: deadline exceeded", ErrDocumentCoverExportTimeout)
	}
	return err
}

func documentCoverExportCallError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return documentCoverExportContextError(ctxErr)
	}
	return documentCoverExportContextError(err)
}
