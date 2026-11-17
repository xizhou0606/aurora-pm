// Package figma implements the server-side, token-confined Figma preview port.
package figma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string, timeout time.Duration) *Client {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return &Client{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), token: strings.TrimSpace(token), http: &http.Client{Timeout: timeout, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !allowedImageURL(request.URL) {
			return designreview.ErrInvalid
		}
		return nil
	}}}
}

const (
	maxFigmaEvidenceBytes = 20 << 20
	maxFigmaResponseBytes = 8 << 20
)

func (client *Client) Preview(ctx context.Context, fileKey, nodeID string) (designreview.FigmaPreview, error) {
	if client == nil {
		return designreview.FigmaPreview{}, designreview.ErrUnavailable
	}
	nodeURL := client.baseURL + "/v1/files/" + url.PathEscape(fileKey) + "/nodes?ids=" + url.QueryEscape(nodeID)
	var nodeResponse struct {
		Nodes map[string]struct {
			Document figmaNode `json:"document"`
		} `json:"nodes"`
	}
	if err := client.getJSON(ctx, nodeURL, &nodeResponse); err != nil {
		return designreview.FigmaPreview{}, err
	}
	node, ok := nodeResponse.Nodes[nodeID]
	if !ok || node.Document.AbsoluteBoundingBox == nil {
		return designreview.FigmaPreview{}, designreview.ErrNotFound
	}
	imageURL := client.baseURL + "/v1/images/" + url.PathEscape(fileKey) + "?ids=" + url.QueryEscape(nodeID) + "&format=png&scale=1"
	var imageResponse struct {
		Images map[string]string `json:"images"`
	}
	if err := client.getJSON(ctx, imageURL, &imageResponse); err != nil {
		return designreview.FigmaPreview{}, err
	}
	previewURL := strings.TrimSpace(imageResponse.Images[nodeID])
	if previewURL == "" {
		return designreview.FigmaPreview{}, designreview.ErrNotFound
	}
	structure, err := json.Marshal(sanitizeFigmaNode(node.Document, 0))
	if err != nil || len(structure) == 0 || len(structure) > maxFigmaEvidenceBytes {
		return designreview.FigmaPreview{}, designreview.ErrInvalid
	}
	return designreview.FigmaPreview{FileKey: fileKey, NodeID: nodeID, NodeName: node.Document.Name, ImageURL: previewURL, Width: int(node.Document.AbsoluteBoundingBox.Width), Height: int(node.Document.AbsoluteBoundingBox.Height), Structure: structure}, nil
}

func (client *Client) Evidence(ctx context.Context, fileKey, nodeID string) (designreview.FigmaPreview, []byte, string, error) {
	preview, err := client.Preview(ctx, fileKey, nodeID)
	if err != nil {
		return designreview.FigmaPreview{}, nil, "", err
	}
	endpoint, err := url.Parse(preview.ImageURL)
	if err != nil || !allowedImageURL(endpoint) {
		return designreview.FigmaPreview{}, nil, "", designreview.ErrInvalid
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return designreview.FigmaPreview{}, nil, "", err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return designreview.FigmaPreview{}, nil, "", designreview.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return designreview.FigmaPreview{}, nil, "", designreview.ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxFigmaEvidenceBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxFigmaEvidenceBytes {
		return designreview.FigmaPreview{}, nil, "", designreview.ErrInvalid
	}
	contentType := http.DetectContentType(raw)
	if contentType != "image/png" && contentType != "image/jpeg" {
		return designreview.FigmaPreview{}, nil, "", designreview.ErrInvalid
	}
	if _, _, err := image.Decode(bytes.NewReader(raw)); err != nil {
		return designreview.FigmaPreview{}, nil, "", designreview.ErrInvalid
	}
	return preview, raw, contentType, nil
}

func allowedImageURL(value *url.URL) bool {
	if value == nil || value.Scheme != "https" || value.User != nil || value.Port() != "" {
		return false
	}
	host := strings.ToLower(value.Hostname())
	return host == "figma.com" || strings.HasSuffix(host, ".figma.com") ||
		strings.HasSuffix(host, ".figmausercontent.com") || strings.HasSuffix(host, ".amazonaws.com")
}

func (client *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-Figma-Token", client.token)
	response, err := client.http.Do(request)
	if err != nil {
		return designreview.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return designreview.ErrNotFound
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
		return designreview.ErrUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return designreview.ErrInvalid
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxFigmaResponseBytes+1))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Figma response: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return designreview.ErrInvalid
	}
	return nil
}
