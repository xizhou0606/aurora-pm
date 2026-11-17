// Package airuntime implements internal, HMAC-authenticated AI runtime ports.
package airuntime

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

type DesignReviewClient struct {
	baseURL string
	secret  string
	client  *http.Client
	now     func() time.Time
}

func NewDesignReviewClient(baseURL, secret string, timeout time.Duration) *DesignReviewClient {
	if strings.TrimSpace(baseURL) == "" || len(strings.TrimSpace(secret)) < 32 || timeout <= 0 {
		return nil
	}
	return &DesignReviewClient{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), secret: strings.TrimSpace(secret), client: &http.Client{Timeout: timeout}, now: func() time.Time { return time.Now().UTC() }}
}

func (client *DesignReviewClient) Plan(ctx context.Context, input designreview.DOMNormalizationRequest) (designreview.DOMNormalizationPlan, error) {
	if client == nil || input.WorkspaceID == 0 || len(input.FigmaData) == 0 {
		return designreview.DOMNormalizationPlan{}, designreview.ErrUnavailable
	}
	payload := map[string]any{
		"manifest":  map[string]any{"version": "1.0", "workspace_id": input.WorkspaceID, "model": input.Model, "reasoning_effort": input.ReasoningEffort, "image_detail": input.ImageDetail},
		"figma":     map[string]any{"content_type": input.FigmaContentType, "sha256": input.FigmaSHA256, "base64": base64.StdEncoding.EncodeToString(input.FigmaData)},
		"inventory": input.Inventory,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return designreview.DOMNormalizationPlan{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/internal/v1/design-review/dom-normalize", bytes.NewReader(raw))
	if err != nil {
		return designreview.DOMNormalizationPlan{}, err
	}
	timestamp := strconv.FormatInt(client.now().UnixMilli(), 10)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-PM-Timestamp", timestamp)
	request.Header.Set("X-PM-Signature", runtimeSignature(raw, timestamp, client.secret))
	response, err := client.client.Do(request)
	if err != nil {
		return designreview.DOMNormalizationPlan{}, designreview.ErrUnavailable
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return designreview.DOMNormalizationPlan{}, err
	}
	if response.StatusCode == http.StatusServiceUnavailable || response.StatusCode >= 500 {
		return designreview.DOMNormalizationPlan{}, designreview.ErrUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return designreview.DOMNormalizationPlan{}, designreview.ErrInvalid
	}
	var envelope struct {
		Data struct {
			InventoryToken string                            `json:"inventory_token"`
			Model          string                            `json:"model"`
			PromptVersion  string                            `json:"prompt_version"`
			Output         designreview.DOMNormalizationPlan `json:"output"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return designreview.DOMNormalizationPlan{}, fmt.Errorf("decode normalization response")
	}
	plan := envelope.Data.Output
	plan.InventoryToken, plan.Model, plan.PromptVersion = envelope.Data.InventoryToken, envelope.Data.Model, envelope.Data.PromptVersion
	return plan, nil
}

func runtimeSignature(raw []byte, timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}
