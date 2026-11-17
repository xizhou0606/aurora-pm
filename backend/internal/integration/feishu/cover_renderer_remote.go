package feishu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type remoteDocumentCoverRenderer struct {
	endpoint *url.URL
	token    string
	client   *http.Client
}

func NewRemoteDocumentCoverRenderer(
	baseURL string,
	token string,
	timeout time.Duration,
) (DocumentCoverRenderer, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("document cover renderer base URL is invalid")
	}
	if strings.TrimSpace(token) == "" || timeout <= 0 {
		return nil, fmt.Errorf("document cover renderer credentials or timeout are invalid")
	}
	endpoint := *parsed
	endpoint.Path = documentCoverRendererPath
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxConnsPerHost = 2
	transport.MaxIdleConnsPerHost = 2
	return &remoteDocumentCoverRenderer{
		endpoint: &endpoint, token: strings.TrimSpace(token),
		client: &http.Client{
			Timeout: timeout, Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (r *remoteDocumentCoverRenderer) RenderFirstPage(ctx context.Context, content []byte) ([]byte, error) {
	if len(content) == 0 || int64(len(content)) > documentCoverMaxBytes ||
		strings.ToLower(http.DetectContentType(content)) != "application/pdf" {
		return nil, fmt.Errorf("document cover PDF input is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint.String(), bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("create document cover renderer request: %w", err)
	}
	request.Header.Set("Accept", "image/png")
	request.Header.Set("Content-Type", "application/pdf")
	request.Header.Set(documentCoverRendererTokenHeader, r.token)
	response, err := r.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call document cover renderer: %w", err)
	}
	defer response.Body.Close()
	if response.ContentLength > documentCoverRenderMaxBytes {
		return nil, fmt.Errorf("document cover renderer response is too large")
	}
	cover, err := io.ReadAll(io.LimitReader(response.Body, documentCoverRenderMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read document cover renderer response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("document cover renderer status=%d", response.StatusCode)
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr != nil || strings.ToLower(mediaType) != "image/png" ||
		validateRenderedDocumentCover(cover) != nil {
		return nil, fmt.Errorf("document cover renderer response is invalid")
	}
	return cover, nil
}
