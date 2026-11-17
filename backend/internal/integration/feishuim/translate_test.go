package feishuim

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestTranslateToCurrentRewritesPreviousApplicationOpenID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			raw, _ := io.ReadAll(r.Body)
			var body struct {
				AppID string `json:"app_id"`
			}
			_ = json.Unmarshal(raw, &body)
			token := "token-" + body.AppID
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": token})
		case r.URL.Path == "/open-apis/contact/v3/users/ou_old" && r.URL.Query().Get("user_id_type") == "open_id":
			if got := r.Header.Get("Authorization"); got != "Bearer token-cli_old" {
				t.Fatalf("previous app token=%q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "data": map[string]any{"user": map[string]any{"union_id": "on_bridge", "open_id": "ou_old"}},
			})
		case r.URL.Path == "/open-apis/contact/v3/users/on_bridge" && r.URL.Query().Get("user_id_type") == "union_id":
			if got := r.Header.Get("Authorization"); got != "Bearer token-cli_new" {
				t.Fatalf("current app token=%q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "data": map[string]any{"user": map[string]any{"open_id": "ou_new"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	translator, err := NewOpenIDTranslator(config.FeishuLogin{
		AppID: "cli_new", AppSecret: "new-secret",
		PreviousAppID: "cli_old", PreviousAppSecret: "old-secret",
		BaseURL: server.URL,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	openID, err := translator.TranslateToCurrent(context.Background(), "ou_old")
	if err != nil || openID != "ou_new" {
		t.Fatalf("open_id=%q err=%v", openID, err)
	}
}

func TestNewOpenIDTranslatorSkipsWhenPreviousApplicationIsAbsentOrSame(t *testing.T) {
	none, err := NewOpenIDTranslator(config.FeishuLogin{AppID: "cli_new", AppSecret: "secret"}, time.Second)
	if err != nil || none != nil {
		t.Fatalf("absent previous=%v err=%v", none, err)
	}
	same, err := NewOpenIDTranslator(config.FeishuLogin{
		AppID: "cli_same", AppSecret: "secret",
		PreviousAppID: "cli_same", PreviousAppSecret: "other",
	}, time.Second)
	if err != nil || same != nil {
		t.Fatalf("same application=%v err=%v", same, err)
	}
}
