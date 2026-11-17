package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestReadCoverExportsDocxWithoutNativeCoverToPNG(t *testing.T) {
	pdf := testMultiPagePDF(t)
	rendered := testSolidPNG(t, color.RGBA{R: 255, A: 255}, documentCoverRenderWidth, documentCoverRenderHeight)
	var renderedInput []byte
	var paths []string
	var polls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		case "/open-apis/docx/v1/documents/docx-token":
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "data": map[string]any{"document": map[string]any{
					"document_id": "docx-token", "revision_id": 7,
				}},
			})
		case "/open-apis/drive/v1/export_tasks":
			if request.Method != http.MethodPost {
				t.Errorf("create method=%s", request.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode export body: %v", err)
			}
			if body["file_extension"] != "pdf" || body["token"] != "docx-token" || body["type"] != "docx" {
				t.Errorf("export body=%v", body)
			}
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "data": map[string]any{"ticket": "export-ticket"},
			})
		case "/open-apis/drive/v1/export_tasks/export-ticket":
			if request.URL.Query().Get("token") != "docx-token" {
				t.Errorf("poll token=%q", request.URL.Query().Get("token"))
			}
			polls++
			result := map[string]any{"job_status": 2}
			if polls == 2 {
				result = map[string]any{
					"job_status": 0, "job_error_msg": "success",
					"file_token": "pdf-token", "file_size": len(pdf),
				}
			}
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "data": map[string]any{"result": result},
			})
		case "/open-apis/drive/v1/export_tasks/file/pdf-token/download":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(pdf)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	reader := newCoverExportTestReader(t, server, 5*time.Millisecond, time.Second)
	reader.coverRenderer = documentCoverRendererFunc(func(_ context.Context, content []byte) ([]byte, error) {
		renderedInput = append([]byte(nil), content...)
		return rendered, nil
	})
	cover, err := reader.ReadCover(context.Background(), "https://acme.feishu.cn/docx/docx-token")
	if err != nil {
		t.Fatal(err)
	}
	if cover.ContentType != "image/png" || !bytes.Equal(cover.Content, rendered) ||
		cover.OffsetRatioX != 0 || cover.OffsetRatioY != 0 {
		t.Fatalf("cover type=%q bytes=%d offsets=%v,%v",
			cover.ContentType, len(cover.Content), cover.OffsetRatioX, cover.OffsetRatioY)
	}
	if !bytes.Equal(renderedInput, pdf) {
		t.Fatalf("renderer input bytes=%d want=%d", len(renderedInput), len(pdf))
	}
	wantPaths := []string{
		"/open-apis/auth/v3/tenant_access_token/internal",
		"/open-apis/docx/v1/documents/docx-token",
		"/open-apis/drive/v1/export_tasks",
		"/open-apis/drive/v1/export_tasks/export-ticket",
		"/open-apis/drive/v1/export_tasks/export-ticket",
		"/open-apis/drive/v1/export_tasks/file/pdf-token/download",
	}
	if !equalStrings(paths, wantPaths) {
		t.Fatalf("request paths=%v want=%v", paths, wantPaths)
	}
}

func TestReadCoverReturnsExportTaskFailure(t *testing.T) {
	server := newCoverExportStatusServer(t, 3)
	defer server.Close()
	reader := newCoverExportTestReader(t, server, time.Millisecond, time.Second)
	_, err := reader.ReadCover(context.Background(), "https://acme.feishu.cn/docx/docx-token")
	if !errors.Is(err, ErrDocumentCoverExportFailed) {
		t.Fatalf("error=%v, want export failure", err)
	}
}

func TestReadCoverTimesOutWhileExportTaskIsProcessing(t *testing.T) {
	server := newCoverExportStatusServer(t, 2)
	defer server.Close()
	reader := newCoverExportTestReader(t, server, 5*time.Millisecond, 30*time.Millisecond)
	_, err := reader.ReadCover(context.Background(), "https://acme.feishu.cn/docx/docx-token")
	if !errors.Is(err, ErrDocumentCoverExportTimeout) {
		t.Fatalf("error=%v, want export timeout", err)
	}
}

func TestReadCoverDoesNotFallBackToLocalRenderer(t *testing.T) {
	pdf := testMultiPagePDF(t)
	server := newImmediateCoverExportServer(pdf, make(chan struct{}, 1))
	defer server.Close()
	reader := newCoverExportTestReader(t, server, time.Millisecond, time.Second)
	reader.coverRenderer = nil
	_, err := reader.ReadCover(context.Background(), "https://acme.feishu.cn/docx/docx-token")
	if !errors.Is(err, ErrDocumentCoverRendererUnavailable) {
		t.Fatalf("missing remote renderer error=%v", err)
	}
}

func newCoverExportStatusServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeCoverExportJSON(writer, map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		case "/open-apis/docx/v1/documents/docx-token":
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
					"job_status": status, "job_error_msg": "export failed",
				}},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
}

func newCoverExportTestReader(
	t *testing.T,
	server *httptest.Server,
	pollInterval time.Duration,
	timeout time.Duration,
) *SDKReader {
	t.Helper()
	httpClient := newBoundedFeishuHTTPClient(server.Client())
	return &SDKReader{
		cfg: config.Feishu{DomainAllowlist: []string{"feishu.cn"}},
		client: lark.NewClient("cover-export-"+server.URL, "app-secret",
			lark.WithOpenBaseUrl(server.URL), lark.WithOAuthBaseUrl(server.URL),
			lark.WithHttpClient(httpClient)),
		httpClient: httpClient, coverExportCoordinator: newDocumentCoverExportCoordinator(0),
		coverMediaCoordinator: newDocumentCoverMediaCoordinator(1, 0),
		coverRenderer: documentCoverRendererFunc(func(context.Context, []byte) ([]byte, error) {
			return testSolidPNG(t, color.RGBA{R: 255, A: 255}, documentCoverRenderWidth, documentCoverRenderHeight), nil
		}),
		coverExportPollInterval: pollInterval, coverExportTimeout: timeout,
	}
}

func writeCoverExportJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}
