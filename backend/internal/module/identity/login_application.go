package identity

import "context"

// FeishuLoginApplicationCredentials are used only by PM's standalone login
// integration. They are deliberately separate from document-reader and
// notification Feishu applications.
type FeishuLoginApplicationCredentials struct {
	AppID     string
	AppSecret string
}

type FeishuLoginApplicationRepository interface {
	FindFeishuLoginApplication(context.Context) (FeishuLoginApplicationCredentials, error)
}
