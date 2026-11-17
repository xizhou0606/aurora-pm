package collaborationclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

// ReplacementClient talks only to the configured private collaboration runtime.
// The service token is never sent through the browser/public websocket URL.
type ReplacementClient struct{ URL, ServiceToken string }

func (client ReplacementClient) Replace(ctx context.Context, workspaceID, resourceID, ticket string, input document.ContentReplacement) (document.ContentReplacementResult, error) {
	if client.URL == "" || client.ServiceToken == "" {
		return document.ContentReplacementResult{}, document.ErrEditingStateUnavailable
	}
	body, err := json.Marshal(map[string]any{"room": map[string]string{"workspaceId": workspaceID, "resourceId": resourceID}, "ticket": ticket, "connectionId": input.ConnectionID, "operationId": input.OperationID, "expectedRevision": input.ExpectedRevision, "content": input.Content})
	if err != nil {
		return document.ContentReplacementResult{}, document.ErrInvalidInput
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.URL, "/")+"/internal/content-replacement", bytes.NewReader(body))
	if err != nil {
		return document.ContentReplacementResult{}, document.ErrEditingStateUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+client.ServiceToken)
	request.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := httpClient.Do(request)
	if err != nil {
		return document.ContentReplacementResult{}, document.ErrReplacementResultUnknown
	}
	defer response.Body.Close()
	var result struct {
		Revision    uint64 `json:"revision"`
		ContentHash string `json:"content_hash"`
		Code        string `json:"code"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&result) != nil {
		return document.ContentReplacementResult{}, document.ErrReplacementResultUnknown
	}
	if response.StatusCode != 200 {
		switch result.Code {
		case "DOCUMENT_EDITING_IN_PROGRESS":
			return document.ContentReplacementResult{}, document.ErrEditingInProgress
		case "DOCUMENT_REPLACE_IN_PROGRESS":
			return document.ContentReplacementResult{}, document.ErrReplaceInProgress
		case "DOCUMENT_VERSION_CONFLICT":
			return document.ContentReplacementResult{}, document.ErrRevisionConflict
		case "DOCUMENT_FORBIDDEN":
			return document.ContentReplacementResult{}, document.ErrForbidden
		case "DOCUMENT_INPUT_INVALID":
			return document.ContentReplacementResult{}, document.ErrInvalidInput
		}
		return document.ContentReplacementResult{}, document.ErrReplacementResultUnknown
	}
	if result.Revision == 0 || len(result.ContentHash) != 64 {
		return document.ContentReplacementResult{}, document.ErrReplacementResultUnknown
	}
	return document.ContentReplacementResult{Revision: result.Revision, ContentHash: result.ContentHash}, nil
}
