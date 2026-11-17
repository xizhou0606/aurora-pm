// Package feishuim delivers PM-authored Feishu IM cards using the dedicated
// PM login application. Recipients are addressed by that app's open_id, or by
// email when the open_id has not been bound yet.
package feishuim

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const maxIdempotencyKey = 50

type Client struct {
	client *lark.Client
}

func New(cfg config.FeishuLogin, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, errors.New("Feishu IM delivery requires the dedicated login application")
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	options := []lark.ClientOptionFunc{lark.WithHttpClient(httpClient)}
	if base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"); base != "" {
		options = append(options, lark.WithOpenBaseUrl(base), lark.WithOAuthBaseUrl(base))
	}
	return &Client{client: lark.NewClient(strings.TrimSpace(cfg.AppID), strings.TrimSpace(cfg.AppSecret), options...)}, nil
}

func (c *Client) SendInteractive(ctx context.Context, idempotencyKey, receiveIDType, receiveID, cardJSON string) error {
	return c.send(ctx, idempotencyKey, receiveIDType, receiveID, "interactive", cardJSON)
}

func (c *Client) send(ctx context.Context, idempotencyKey, receiveIDType, receiveID, msgType, content string) error {
	if c == nil || c.client == nil {
		return errors.New("Feishu IM client is not configured")
	}
	receiveIDType = strings.TrimSpace(receiveIDType)
	receiveID = strings.TrimSpace(receiveID)
	msgType = strings.TrimSpace(msgType)
	content = strings.TrimSpace(content)
	if receiveID == "" || msgType == "" || content == "" {
		return errors.New("Feishu IM delivery requires a receive id and message body")
	}
	if receiveIDType != "open_id" && receiveIDType != "email" {
		return fmt.Errorf("Feishu IM delivery receive_id_type %q is not supported", receiveIDType)
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(msgType).
			Content(content).
			Uuid(clipIdempotencyKey(idempotencyKey)).
			Build()).
		Build()
	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return err
	}
	if resp == nil || !resp.Success() {
		code, status, msg := 0, 0, "empty response"
		if resp != nil {
			code, status, msg = resp.Code, resp.StatusCode, strings.TrimSpace(resp.Msg)
		}
		return fmt.Errorf("feishu im http %d: {\"code\":%d,\"msg\":%q}", status, code, msg)
	}
	return nil
}

func clipIdempotencyKey(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxIdempotencyKey {
		return value
	}
	return value[:maxIdempotencyKey]
}
