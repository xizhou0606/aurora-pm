package feishu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

const (
	documentCoverMediaStartInterval = time.Second / 5
	documentCoverMediaMaxConcurrent = 4
)

var globalDocumentCoverMediaCoordinator = newDocumentCoverMediaCoordinator(
	documentCoverMediaMaxConcurrent,
	documentCoverMediaStartInterval,
)

type documentCoverMediaCoordinator struct {
	slot          chan struct{}
	startOnce     sync.Once
	startGate     chan struct{}
	nextStart     time.Time
	startInterval time.Duration
}

func newDocumentCoverMediaCoordinator(maxConcurrent int, startInterval time.Duration) *documentCoverMediaCoordinator {
	return &documentCoverMediaCoordinator{
		slot: make(chan struct{}, max(1, maxConcurrent)), startGate: make(chan struct{}, 1),
		startInterval: max(0, startInterval),
	}
}

func (c *documentCoverMediaCoordinator) acquire(ctx context.Context) error {
	select {
	case c.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *documentCoverMediaCoordinator) waitForStart(ctx context.Context) error {
	gate := c.startGateValue()
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if delay := time.Until(c.nextStart); delay > 0 {
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

func (c *documentCoverMediaCoordinator) startGateValue() chan struct{} {
	c.startOnce.Do(func() {
		if c.startGate == nil {
			c.startGate = make(chan struct{}, 1)
		}
	})
	return c.startGate
}

func (c *documentCoverMediaCoordinator) release() {
	<-c.slot
}

func (r *SDKReader) downloadDocumentNativeCover(
	ctx context.Context,
	token string,
	offsetX *float64,
	offsetY *float64,
) (DocumentCover, error) {
	coordinator := r.documentCoverMediaCoordinatorValue()
	if err := coordinator.acquire(ctx); err != nil {
		return DocumentCover{}, err
	}
	defer coordinator.release()
	if err := coordinator.waitForStart(ctx); err != nil {
		return DocumentCover{}, err
	}
	download, err := r.client.Drive.V1.Media.Download(ctx, larkdrive.NewDownloadMediaReqBuilder().
		FileToken(strings.TrimSpace(token)).Build())
	if err != nil {
		return DocumentCover{}, fmt.Errorf("download document cover: %w", err)
	}
	if download == nil || !download.Success() || download.File == nil {
		return DocumentCover{}, fmt.Errorf("download document cover failed")
	}
	content, err := io.ReadAll(io.LimitReader(download.File, documentCoverMaxBytes+1))
	if err != nil {
		return DocumentCover{}, fmt.Errorf("read document cover: %w", err)
	}
	if len(content) == 0 || int64(len(content)) > documentCoverMaxBytes {
		return DocumentCover{}, fmt.Errorf("document cover size is invalid")
	}
	contentType := strings.ToLower(http.DetectContentType(content))
	if !validDocumentCoverContentType(contentType) {
		return DocumentCover{}, fmt.Errorf("document cover media type is invalid")
	}
	return normalizeNativeDocumentCover(content, contentType,
		clampDocumentCoverOffset(offsetX), clampDocumentCoverOffset(offsetY))
}

func (r *SDKReader) documentCoverMediaCoordinatorValue() *documentCoverMediaCoordinator {
	if r.coverMediaCoordinator != nil {
		return r.coverMediaCoordinator
	}
	return globalDocumentCoverMediaCoordinator
}
