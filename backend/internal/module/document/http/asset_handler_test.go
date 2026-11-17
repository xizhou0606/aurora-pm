package documenthttp

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func TestAssetContentHeadersOnlyInlineSafeImageTypesAndDisableCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		contentType string
		disposition string
	}{
		{contentType: "image/png", disposition: "inline"},
		{contentType: "image/jpeg", disposition: "inline"},
		{contentType: "image/webp", disposition: "inline"},
		{contentType: "image/gif", disposition: "inline"},
		{contentType: "image/svg+xml", disposition: "attachment"},
		{contentType: "text/html", disposition: "attachment"},
		{contentType: "application/pdf", disposition: "attachment"},
		{contentType: "video/mp4", disposition: "attachment"},
	}
	for _, test := range tests {
		t.Run(test.contentType, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			setAssetContentHeaders(context, document.Asset{
				FileName: "设计稿 01.svg", ContentType: test.contentType, SizeBytes: 19,
			})
			if disposition := recorder.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, test.disposition+";") {
				t.Fatalf("content disposition=%q", disposition)
			}
			if recorder.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("cache-control=%q", recorder.Header().Get("Cache-Control"))
			}
			if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("x-content-type-options=%q", recorder.Header().Get("X-Content-Type-Options"))
			}
			if recorder.Header().Get("Content-Length") != "19" {
				t.Fatalf("content-length=%q", recorder.Header().Get("Content-Length"))
			}
		})
	}
}
