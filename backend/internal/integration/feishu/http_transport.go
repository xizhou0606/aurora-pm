package feishu

import (
	"io"
	"net/http"
	"strings"
)

type boundedFeishuResponseTransport struct {
	base     http.RoundTripper
	maxBytes int64
}

type boundedFeishuResponseBody struct {
	io.Reader
	io.Closer
}

func newBoundedFeishuHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	bounded := *client
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	bounded.Transport = &boundedFeishuResponseTransport{
		base: base, maxBytes: documentCoverMaxBytes + 1,
	}
	return &bounded
}

func (t *boundedFeishuResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil || response == nil || response.Body == nil || !isFeishuCoverDownloadPath(request.URL.Path) {
		return response, err
	}
	response.Body = &boundedFeishuResponseBody{
		Reader: io.LimitReader(response.Body, t.maxBytes), Closer: response.Body,
	}
	return response, nil
}

func isFeishuCoverDownloadPath(path string) bool {
	return strings.HasSuffix(path, "/download") &&
		(strings.HasPrefix(path, "/open-apis/drive/v1/medias/") ||
			strings.HasPrefix(path, "/open-apis/drive/v1/export_tasks/file/"))
}
