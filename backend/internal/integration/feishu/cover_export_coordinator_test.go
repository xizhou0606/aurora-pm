package feishu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDocumentCoverExportCoordinatorAllowsOnlyOneActiveExport(t *testing.T) {
	coordinator := newDocumentCoverExportCoordinator(0)
	if err := coordinator.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	var secondStarted atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := coordinator.acquire(context.Background()); err == nil {
			secondStarted.Store(true)
			coordinator.release()
		}
	}()
	time.Sleep(10 * time.Millisecond)
	if secondStarted.Load() {
		t.Fatal("second export started before the first export released its slot")
	}
	coordinator.release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second export did not start after release")
	}
	if !secondStarted.Load() {
		t.Fatal("second export never started")
	}
}

func TestDocumentCoverExportLimiterCancellationDoesNotCreateDebt(t *testing.T) {
	coordinator := newDocumentCoverExportCoordinator(time.Second)
	initialNextStart := time.Now().Add(time.Second)
	coordinator.nextStart = initialNextStart
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := coordinator.waitForStart(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait error=%v", err)
	}
	if !coordinator.nextStart.Equal(initialNextStart) {
		t.Fatalf("cancelled wait changed next start from %v to %v",
			initialNextStart, coordinator.nextStart)
	}
}

func TestDocumentCoverExportTimeoutStartsAfterSlotAcquisition(t *testing.T) {
	pdf := testMultiPagePDF(t)
	metadataRead := make(chan struct{})
	server := newImmediateCoverExportServer(pdf, metadataRead)
	defer server.Close()
	reader := newCoverExportTestReader(t, server, time.Millisecond, 200*time.Millisecond)
	if err := reader.coverExportCoordinator.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := reader.ReadCover(context.Background(), "https://acme.feishu.cn/docx/docx-token")
		result <- err
	}()
	select {
	case <-metadataRead:
	case <-time.After(time.Second):
		t.Fatal("document metadata was not read")
	}
	time.Sleep(250 * time.Millisecond)
	reader.coverExportCoordinator.release()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("queued export failed after slot release: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued export did not finish")
	}
}

func newImmediateCoverExportServer(pdf []byte, metadataRead chan<- struct{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		case "/open-apis/docx/v1/documents/docx-token":
			select {
			case metadataRead <- struct{}{}:
			default:
			}
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "data": map[string]any{"document": map[string]any{
					"document_id": "docx-token",
				}},
			})
		case "/open-apis/drive/v1/export_tasks":
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "data": map[string]any{"ticket": "export-ticket"},
			})
		case "/open-apis/drive/v1/export_tasks/export-ticket":
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "data": map[string]any{"result": map[string]any{
					"job_status": 0, "file_token": "pdf-token", "file_size": len(pdf),
				}},
			})
		case "/open-apis/drive/v1/export_tasks/file/pdf-token/download":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(pdf)
		default:
			http.NotFound(writer, request)
		}
	}))
}
