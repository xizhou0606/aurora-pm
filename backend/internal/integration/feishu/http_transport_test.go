package feishu

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type trackedLargeResponseBody struct {
	mu        sync.Mutex
	remaining int64
	read      int64
	closed    bool
}

func (b *trackedLargeResponseBody) Read(buffer []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining == 0 {
		return 0, io.EOF
	}
	count := int64(len(buffer))
	if count > b.remaining {
		count = b.remaining
	}
	for index := int64(0); index < count; index++ {
		buffer[index] = 'x'
	}
	b.remaining -= count
	b.read += count
	return int(count), nil
}

func (b *trackedLargeResponseBody) Close() error {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	return nil
}

func TestFeishuSDKResponseBodyIsTruncatedAndOversizedCoverRejected(t *testing.T) {
	largeBody := &trackedLargeResponseBody{remaining: documentCoverMaxBytes + 4096}
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body io.ReadCloser
		header := make(http.Header)
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			header.Set("Content-Type", "application/json")
			body = io.NopCloser(strings.NewReader(
				`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/docx/v1/documents/docx-token":
			header.Set("Content-Type", "application/json")
			body = io.NopCloser(strings.NewReader(
				`{"code":0,"data":{"document":{"document_id":"docx-token","cover":{"token":"cover-token"}}}}`))
		case "/open-apis/drive/v1/medias/cover-token/download":
			header.Set("Content-Type", "application/octet-stream")
			body = largeBody
		default:
			return &http.Response{StatusCode: http.StatusNotFound,
				Header: make(http.Header), Body: io.NopCloser(strings.NewReader("not found")), Request: request}, nil
		}
		return &http.Response{StatusCode: http.StatusOK,
			Header: header, Body: body, Request: request}, nil
	})
	httpClient := newBoundedFeishuHTTPClient(&http.Client{Transport: base})
	reader := &SDKReader{
		cfg: config.Feishu{DomainAllowlist: []string{"feishu.cn"}},
		client: lark.NewClient("bounded-cover-client", "app-secret",
			lark.WithOpenBaseUrl("https://open.test"), lark.WithOAuthBaseUrl("https://open.test"),
			lark.WithHttpClient(httpClient)),
		httpClient: httpClient, coverMediaCoordinator: newDocumentCoverMediaCoordinator(1, 0),
	}
	_, err := reader.ReadCover(context.Background(), "https://acme.feishu.cn/docx/docx-token")
	if err == nil || !strings.Contains(err.Error(), "size is invalid") {
		t.Fatalf("oversized cover error=%v", err)
	}
	largeBody.mu.Lock()
	defer largeBody.mu.Unlock()
	if largeBody.read != documentCoverMaxBytes+1 || !largeBody.closed {
		t.Fatalf("SDK media bytes read=%d closed=%v", largeBody.read, largeBody.closed)
	}
}
