package figma

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(status int, contentType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func nodePayload(imageURL string) []byte {
	payload := map[string]any{
		"nodes": map[string]any{"1:2": map[string]any{"document": map[string]any{
			"id": "1:2", "name": "Checkout", "type": "FRAME",
			"absoluteBoundingBox": map[string]any{"x": 0, "y": 0, "width": 390, "height": 844},
		}}},
	}
	if imageURL != "" {
		payload = map[string]any{"images": map[string]string{"1:2": imageURL}}
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	canvas := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	canvas.Set(0, 0, color.NRGBA{R: 24, G: 96, B: 180, A: 255})
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestPreviewUsesHeaderTokenAndReturnsValidatedStructure(t *testing.T) {
	client := New("https://api.figma.com", "figma-secret", time.Second)
	requests := 0
	client.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Header.Get("X-Figma-Token") != "figma-secret" || strings.Contains(request.URL.RawQuery, "figma-secret") || request.Header.Get("Authorization") != "" {
			t.Fatalf("token was not confined to X-Figma-Token: %+v", request)
		}
		if strings.Contains(request.URL.Path, "/nodes") {
			return response(http.StatusOK, "application/json", nodePayload("")), nil
		}
		return response(http.StatusOK, "application/json", nodePayload("https://cdn.figmausercontent.com/checkout.png")), nil
	})
	preview, err := client.Preview(context.Background(), "file-key", "1:2")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || preview.NodeName != "Checkout" || preview.Width != 390 || preview.Height != 844 {
		t.Fatalf("unexpected preview: requests=%d preview=%+v", requests, preview)
	}
	if err := designreview.ValidateDesignStructure(preview.Structure); err != nil {
		t.Fatalf("preview structure did not satisfy the domain contract: %v", err)
	}
}

func TestSanitizeFigmaNodeBoundsDepthChildrenAndText(t *testing.T) {
	longText := strings.Repeat("界", 501)
	root := figmaNode{ID: "1:1", Name: "Root", Type: "FRAME", AbsoluteBoundingBox: &figmaBounds{Width: 100, Height: 100}, Characters: &longText}
	root.Children = make([]figmaNode, 201)
	for index := range root.Children {
		root.Children[index] = figmaNode{ID: "2:" + string(rune(index+1)), Name: "Child", Type: "FRAME", AbsoluteBoundingBox: &figmaBounds{Width: 1, Height: 1}}
	}
	deep := figmaNode{ID: "deep", Name: "Deep", Type: "FRAME", Children: []figmaNode{{ID: "hidden", Name: "Hidden", Type: "FRAME"}}}
	result := sanitizeFigmaNode(root, 0)
	if len(result.Children) != 200 || !result.ChildrenTruncated || result.Text == nil || len([]rune(*result.Text)) != 500 {
		t.Fatalf("sanitization limits not applied: children=%d truncated=%v text=%d", len(result.Children), result.ChildrenTruncated, len([]rune(*result.Text)))
	}
	deepResult := sanitizeFigmaNode(deep, 12)
	if !deepResult.Truncated || len(deepResult.Children) != 0 {
		t.Fatalf("depth limit not applied: %+v", deepResult)
	}
}

func TestPreviewMapsFigmaHTTPFailures(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{status: http.StatusNotFound, want: designreview.ErrNotFound},
		{status: http.StatusTooManyRequests, want: designreview.ErrUnavailable},
		{status: http.StatusBadGateway, want: designreview.ErrUnavailable},
		{status: http.StatusForbidden, want: designreview.ErrInvalid},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			client := New("https://api.figma.com", "figma-secret", time.Second)
			client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(test.status, "application/json", []byte(`{}`)), nil
			})
			_, err := client.Preview(context.Background(), "file-key", "1:2")
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v, want %v", err, test.want)
			}
		})
	}
}

func TestEvidenceDownloadsOnlyValidBoundedImages(t *testing.T) {
	imageBytes := testPNG(t)
	newClient := func(asset []byte, contentType string) *Client {
		client := New("https://api.figma.com", "figma-secret", time.Second)
		client.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(request.URL.Path, "/nodes"):
				return response(http.StatusOK, "application/json", nodePayload("")), nil
			case strings.Contains(request.URL.Path, "/images/"):
				return response(http.StatusOK, "application/json", nodePayload("https://cdn.figmausercontent.com/checkout.png")), nil
			default:
				if request.Header.Get("X-Figma-Token") != "" {
					t.Fatal("Figma token leaked to the asset host")
				}
				return response(http.StatusOK, contentType, asset), nil
			}
		})
		return client
	}
	preview, raw, contentType, err := newClient(imageBytes, "image/png").Evidence(context.Background(), "file-key", "1:2")
	if err != nil || preview.NodeID != "1:2" || contentType != "image/png" || !bytes.Equal(raw, imageBytes) {
		t.Fatalf("valid evidence rejected: preview=%+v type=%s err=%v", preview, contentType, err)
	}
	if _, _, _, err := newClient([]byte("not an image"), "text/plain").Evidence(context.Background(), "file-key", "1:2"); !errors.Is(err, designreview.ErrInvalid) {
		t.Fatalf("non-image error=%v, want invalid", err)
	}
	oversized := make([]byte, maxFigmaEvidenceBytes+1)
	copy(oversized, imageBytes)
	if _, _, _, err := newClient(oversized, "image/png").Evidence(context.Background(), "file-key", "1:2"); !errors.Is(err, designreview.ErrInvalid) {
		t.Fatalf("oversized image error=%v, want invalid", err)
	}
}

func TestAllowedImageURLAndRedirectPolicy(t *testing.T) {
	allowed := []string{"https://figma.com/a.png", "https://cdn.figma.com/a.png", "https://cdn.figmausercontent.com/a.png", "https://bucket.amazonaws.com/a.png"}
	denied := []string{"http://cdn.figma.com/a.png", "https://localhost/a.png", "https://127.0.0.1/a.png", "https://figma.com:8443/a.png", "https://user@figma.com/a.png", "https://figma.com.evil.test/a.png"}
	for _, raw := range allowed {
		parsed, _ := url.Parse(raw)
		if !allowedImageURL(parsed) {
			t.Errorf("allowed URL rejected: %s", raw)
		}
	}
	for _, raw := range denied {
		parsed, _ := url.Parse(raw)
		if allowedImageURL(parsed) {
			t.Errorf("denied URL accepted: %s", raw)
		}
	}
	client := New("https://api.figma.com", "figma-secret", time.Second)
	evil, _ := http.NewRequest(http.MethodGet, "https://localhost/a.png", nil)
	if !errors.Is(client.http.CheckRedirect(evil, nil), designreview.ErrInvalid) {
		t.Fatal("unsafe redirect was accepted")
	}
}
