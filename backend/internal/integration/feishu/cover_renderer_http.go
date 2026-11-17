package feishu

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

const (
	documentCoverRendererPath        = "/internal/v1/render/first-page"
	documentCoverRendererTokenHeader = "X-PM-Cover-Renderer-Token"
)

type documentCoverRendererReadiness interface {
	CheckReady() error
}

type documentCoverRendererHTTPHandler struct {
	token    string
	renderer DocumentCoverRenderer
}

func NewDocumentCoverRendererHandler(token string, renderer DocumentCoverRenderer) http.Handler {
	handler := &documentCoverRendererHTTPHandler{
		token: strings.TrimSpace(token), renderer: renderer,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", handler.live)
	mux.HandleFunc("GET /health/ready", handler.ready)
	mux.HandleFunc("POST "+documentCoverRendererPath, handler.render)
	return mux
}

func (h *documentCoverRendererHTTPHandler) live(writer http.ResponseWriter, _ *http.Request) {
	writeDocumentCoverRendererText(writer, http.StatusOK, "ok")
}

func (h *documentCoverRendererHTTPHandler) ready(writer http.ResponseWriter, _ *http.Request) {
	if h.renderer == nil {
		writeDocumentCoverRendererText(writer, http.StatusServiceUnavailable, "unavailable")
		return
	}
	if readiness, ok := h.renderer.(documentCoverRendererReadiness); ok {
		if err := readiness.CheckReady(); err != nil {
			writeDocumentCoverRendererText(writer, http.StatusServiceUnavailable, "unavailable")
			return
		}
	}
	writeDocumentCoverRendererText(writer, http.StatusOK, "ok")
}

func (h *documentCoverRendererHTTPHandler) render(writer http.ResponseWriter, request *http.Request) {
	if !documentCoverRendererTokenMatches(h.token, request.Header.Get(documentCoverRendererTokenHeader)) {
		writeDocumentCoverRendererText(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || strings.ToLower(mediaType) != "application/pdf" {
		writeDocumentCoverRendererText(writer, http.StatusUnsupportedMediaType, "PDF required")
		return
	}
	if request.ContentLength > documentCoverMaxBytes {
		writeDocumentCoverRendererText(writer, http.StatusRequestEntityTooLarge, "request too large")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, documentCoverMaxBytes)
	content, err := io.ReadAll(request.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeDocumentCoverRendererText(writer, http.StatusRequestEntityTooLarge, "request too large")
			return
		}
		writeDocumentCoverRendererText(writer, http.StatusBadRequest, "invalid request")
		return
	}
	if len(content) == 0 || strings.ToLower(http.DetectContentType(content)) != "application/pdf" {
		writeDocumentCoverRendererText(writer, http.StatusUnsupportedMediaType, "PDF required")
		return
	}
	if h.renderer == nil {
		writeDocumentCoverRendererText(writer, http.StatusServiceUnavailable, "unavailable")
		return
	}
	cover, err := h.renderer.RenderFirstPage(request.Context(), content)
	if err != nil || validateRenderedDocumentCover(cover) != nil {
		writeDocumentCoverRendererText(writer, http.StatusUnprocessableEntity, "render failed")
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Length", strconv.Itoa(len(cover)))
	writer.Header().Set("Content-Type", "image/png")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(cover)
}

func documentCoverRendererTokenMatches(expected, actual string) bool {
	expectedHash := sha256.Sum256([]byte(expected))
	actualHash := sha256.Sum256([]byte(strings.TrimSpace(actual)))
	return expected != "" && subtle.ConstantTimeCompare(expectedHash[:], actualHash[:]) == 1
}

func writeDocumentCoverRendererText(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(message))
}
