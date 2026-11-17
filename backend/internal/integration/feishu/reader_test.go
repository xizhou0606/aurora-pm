package feishu

import (
	"context"
	"encoding/json"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestNewConfiguresBoundedSDKRequestTimeout(t *testing.T) {
	reader := New(config.Feishu{})
	if reader.httpClient == nil || reader.httpClient.Timeout != sdkRequestTimeout {
		t.Fatalf("Feishu HTTP timeout=%v, want %v", reader.httpClient, sdkRequestTimeout)
	}
}

func TestReadCoverResolvesWikiAndDownloadsDocumentCover(t *testing.T) {
	png := testSolidPNG(t, color.RGBA{R: 255, A: 255}, 64, 32)
	var paths []string
	var wikiToken string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		case "/open-apis/wiki/v2/spaces/get_node":
			wikiToken = request.URL.Query().Get("token")
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"code": 0, "data": map[string]any{"node": map[string]any{
					"obj_token": "docx-token", "obj_type": "docx",
				}},
			})
		case "/open-apis/docx/v1/documents/docx-token":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"code": 0, "data": map[string]any{"document": map[string]any{
					"document_id": "docx-token", "revision_id": 7,
					"cover": map[string]any{
						"token": "cover-token", "offset_ratio_x": 3, "offset_ratio_y": -2,
					},
				}},
			})
		case "/open-apis/drive/v1/medias/cover-token/download":
			writer.Header().Set("Content-Type", "application/octet-stream")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(png)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	httpClient := server.Client()
	reader := &SDKReader{
		cfg: config.Feishu{DomainAllowlist: []string{"feishu.cn"}},
		client: lark.NewClient("app-id", "app-secret",
			lark.WithOpenBaseUrl(server.URL), lark.WithOAuthBaseUrl(server.URL),
			lark.WithHttpClient(httpClient)),
		httpClient: httpClient,
	}
	cover, err := reader.ReadCover(context.Background(), "https://acme.feishu.cn/wiki/wiki-token")
	if err != nil {
		t.Fatal(err)
	}
	if cover.ContentType != "image/png" || validateRenderedDocumentCover(cover.Content) != nil ||
		cover.OffsetRatioX != 0 || cover.OffsetRatioY != 0 {
		t.Fatalf("cover type=%q bytes=%d offsets=%v,%v",
			cover.ContentType, len(cover.Content), cover.OffsetRatioX, cover.OffsetRatioY)
	}
	if wikiToken != "wiki-token" {
		t.Fatalf("wiki token=%q", wikiToken)
	}
	wantPaths := []string{
		"/open-apis/auth/v3/tenant_access_token/internal",
		"/open-apis/wiki/v2/spaces/get_node",
		"/open-apis/docx/v1/documents/docx-token",
		"/open-apis/drive/v1/medias/cover-token/download",
	}
	if !equalStrings(paths, wantPaths) {
		t.Fatalf("request paths=%v want=%v", paths, wantPaths)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestParseDocumentURL(t *testing.T) {
	tests := []struct {
		raw         string
		ok          bool
		kind, token string
	}{
		{"https://acme.feishu.cn/wiki/wikcnABC", true, "wiki", "wikcnABC"},
		{"https://acme.feishu.cn/docx/doxcnABC", true, "docx", "doxcnABC"},
		{"https://tcnhujxo5o2j.feishu.cn/wiki/TWG3wH2SsivKKBkiezpcfeeDn3e?from=from_copylink", true, "wiki", "TWG3wH2SsivKKBkiezpcfeeDn3e"},
		{"https://acme.feishu.cn/wiki/wikcnABC#heading", true, "wiki", "wikcnABC"},
		{"https://feishu.cn.evil.test/wiki/token", false, "", ""},
		{"http://acme.feishu.cn/wiki/token", false, "", ""},
		{"https://acme.feishu.cn/sheets/token", false, "", ""},
		{"https://acme.feishu.cn:8443/wiki/token", false, "", ""},
		{"https://acme.feishu.cn/wiki/token/extra", false, "", ""},
		{"https://user:pass@acme.feishu.cn/wiki/token", false, "", ""},
	}
	for _, test := range tests {
		kind, token, normalized, err := parseDocumentURL(test.raw, []string{".feishu.cn"})
		if (err == nil) != test.ok || kind != test.kind || token != test.token {
			t.Fatalf("parse %q kind=%q token=%q err=%v", test.raw, kind, token, err)
		}
		if test.ok && (strings.Contains(normalized, "?") || strings.Contains(normalized, "#")) {
			t.Fatalf("normalized %q still has query or fragment", normalized)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("需求文档ABC", 4); got != "需求文档" {
		t.Fatalf("truncate=%q", got)
	}
	content, originalChars, truncated := truncateRunesWithMetadata("需求文档ABC", 4)
	if content != "需求文档" || originalChars != 7 || !truncated {
		t.Fatalf("content=%q original=%d truncated=%v", content, originalChars, truncated)
	}
	content, originalChars, truncated = truncateRunesWithMetadata("完整", 2)
	if content != "完整" || originalChars != 2 || truncated {
		t.Fatalf("content=%q original=%d truncated=%v", content, originalChars, truncated)
	}
}
