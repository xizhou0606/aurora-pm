package feishuim

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestSendInteractivePostsPMCardThroughLoginApplication(t *testing.T) {
	cardJSON := `{"header":{"title":{"content":"【PM】通知"}}}`
	var gotReceiveID, gotContent, gotUUID, gotReceiveType, gotMsgType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		case "/open-apis/im/v1/messages":
			gotReceiveType = r.URL.Query().Get("receive_id_type")
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				ReceiveID string `json:"receive_id"`
				MsgType   string `json:"msg_type"`
				Content   string `json:"content"`
				UUID      string `json:"uuid"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			gotReceiveID, gotContent, gotUUID, gotMsgType = payload.ReceiveID, payload.Content, payload.UUID, payload.MsgType
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{client: lark.NewClient("app-id", "app-secret",
		lark.WithOpenBaseUrl(server.URL), lark.WithOAuthBaseUrl(server.URL),
		lark.WithHttpClient(server.Client()))}
	if err := client.SendInteractive(context.Background(), "pm-outbox-42-card", "open_id", "ou_alice", cardJSON); err != nil {
		t.Fatal(err)
	}
	if gotReceiveType != "open_id" || gotReceiveID != "ou_alice" || gotUUID != "pm-outbox-42-card" ||
		gotMsgType != "interactive" || gotContent != cardJSON {
		t.Fatalf("receive_id_type=%q receive_id=%q uuid=%q msg_type=%q content=%q",
			gotReceiveType, gotReceiveID, gotUUID, gotMsgType, gotContent)
	}
}

func TestSendInteractivePostsPMCardThroughEmail(t *testing.T) {
	cardJSON := `{"header":{"title":{"content":"【PM】通知"}}}`
	var gotReceiveID, gotReceiveType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		case "/open-apis/im/v1/messages":
			gotReceiveType = r.URL.Query().Get("receive_id_type")
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				ReceiveID string `json:"receive_id"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			gotReceiveID = payload.ReceiveID
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{client: lark.NewClient("app-id", "app-secret",
		lark.WithOpenBaseUrl(server.URL), lark.WithOAuthBaseUrl(server.URL),
		lark.WithHttpClient(server.Client()))}
	if err := client.SendInteractive(context.Background(), "pm-outbox-48-card", "email", "alice@example.com", cardJSON); err != nil {
		t.Fatal(err)
	}
	if gotReceiveType != "email" || gotReceiveID != "alice@example.com" {
		t.Fatalf("receive_id_type=%q receive_id=%q", gotReceiveType, gotReceiveID)
	}
}

func TestSendInteractiveSurfacesPermanentFeishuIdentityErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		case "/open-apis/im/v1/messages":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 230013, "msg": "Bot has NO availability to this user."})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{client: lark.NewClient("app-id", "app-secret",
		lark.WithOpenBaseUrl(server.URL), lark.WithOAuthBaseUrl(server.URL),
		lark.WithHttpClient(server.Client()))}
	err := client.SendInteractive(context.Background(), "pm-outbox-43-card", "open_id", "ou_missing", `{"header":{"title":{"content":"【PM】通知"}}}`)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "bot has no availability to this user") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewRequiresLoginApplication(t *testing.T) {
	if _, err := New(config.FeishuLogin{}, time.Second); err == nil {
		t.Fatal("expected missing application error")
	}
}
