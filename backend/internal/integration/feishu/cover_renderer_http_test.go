package feishu

import (
	"bytes"
	"context"
	"image/color"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const testDocumentCoverRendererToken = "renderer-token-that-is-at-least-32-characters"

func TestRemoteDocumentCoverRendererHTTPContract(t *testing.T) {
	pdf := testMultiPagePDF(t)
	png := testSolidPNG(t, color.RGBA{R: 255, A: 255},
		documentCoverRenderWidth, documentCoverRenderHeight)
	var renderedInput []byte
	handler := NewDocumentCoverRendererHandler(testDocumentCoverRendererToken,
		documentCoverRendererFunc(func(_ context.Context, content []byte) ([]byte, error) {
			renderedInput = append([]byte(nil), content...)
			return png, nil
		}))
	server := httptest.NewServer(handler)
	defer server.Close()
	renderer, err := NewRemoteDocumentCoverRenderer(
		server.URL, testDocumentCoverRendererToken, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cover, err := renderer.RenderFirstPage(context.Background(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(renderedInput, pdf) || !bytes.Equal(cover, png) {
		t.Fatalf("renderer input=%d output=%d", len(renderedInput), len(cover))
	}
}

func TestDocumentCoverRendererHTTPFailsClosed(t *testing.T) {
	pdf := testMultiPagePDF(t)
	validPNG := testSolidPNG(t, color.RGBA{R: 255, A: 255},
		documentCoverRenderWidth, documentCoverRenderHeight)
	var calls atomic.Int32
	handler := NewDocumentCoverRendererHandler(testDocumentCoverRendererToken,
		documentCoverRendererFunc(func(_ context.Context, _ []byte) ([]byte, error) {
			calls.Add(1)
			return validPNG, nil
		}))
	tests := []struct {
		name        string
		token       string
		contentType string
		content     []byte
		length      int64
		status      int
	}{
		{name: "missing token", contentType: "application/pdf", content: pdf, status: http.StatusUnauthorized},
		{name: "wrong token", token: "wrong", contentType: "application/pdf", content: pdf, status: http.StatusUnauthorized},
		{name: "wrong media type", token: testDocumentCoverRendererToken,
			contentType: "text/plain", content: pdf, status: http.StatusUnsupportedMediaType},
		{name: "spoofed PDF media type", token: testDocumentCoverRendererToken,
			contentType: "application/pdf", content: []byte("not a PDF"), status: http.StatusUnsupportedMediaType},
		{name: "oversized", token: testDocumentCoverRendererToken, contentType: "application/pdf",
			content: nil, length: documentCoverMaxBytes + 1, status: http.StatusRequestEntityTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, documentCoverRendererPath, bytes.NewReader(test.content))
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set(documentCoverRendererTokenHeader, test.token)
			if test.length > 0 {
				request.ContentLength = test.length
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid requests invoked renderer %d times", calls.Load())
	}
}

func TestRemoteDocumentCoverRendererRejectsInvalidOutputAndRedirects(t *testing.T) {
	invalid := testSolidPNG(t, color.RGBA{B: 255, A: 255}, 32, 32)
	invalidServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(invalid)
	}))
	defer invalidServer.Close()
	renderer, err := NewRemoteDocumentCoverRenderer(
		invalidServer.URL, testDocumentCoverRendererToken, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.RenderFirstPage(context.Background(), testMultiPagePDF(t)); err == nil {
		t.Fatal("invalid renderer output was accepted")
	}

	var redirectTargetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectTargetCalls.Add(1)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	renderer, err = NewRemoteDocumentCoverRenderer(
		redirect.URL, testDocumentCoverRendererToken, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.RenderFirstPage(context.Background(), testMultiPagePDF(t)); err == nil ||
		redirectTargetCalls.Load() != 0 {
		t.Fatalf("redirect followed or accepted: target calls=%d err=%v", redirectTargetCalls.Load(), err)
	}
}
