package feishulogin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestAuthorizationURLDoesNotContainSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	client, err := New(config.FeishuLogin{AppID: "app", AppSecret: "secret", BaseURL: server.URL, RedirectURL: "https://pm.example.test/callback"})
	if err != nil {
		t.Fatal(err)
	}
	redirect, err := client.AuthorizationURL("state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(redirect)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("state") != "state-value" || parsed.Query().Get("app_id") != "app" ||
		strings.Contains(redirect, "secret") {
		t.Fatalf("unexpected redirect %q", redirect)
	}
}

func TestExchangeResolvesTenantIdentityWithoutReturningProviderToken(t *testing.T) {
	var sawToken bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/authen/v2/oauth/token":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["client_secret"] != "secret" || body["code"] != "provider-code" {
				t.Fatalf("exchange body=%v err=%v", body, err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"access_token": "provider-token"}})
		case "/open-apis/authen/v1/user_info":
			if r.Header.Get("Authorization") != "Bearer provider-token" {
				t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
			}
			sawToken = true
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{
				"open_id": "ou_123", "tenant_key": "tenant_123", "name": "Alice", "avatar_url": "https://avatar.test/alice.png",
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(config.FeishuLogin{AppID: "app", AppSecret: "secret", BaseURL: server.URL, RedirectURL: "https://pm.example.test/callback"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := client.Exchange(context.Background(), "provider-code")
	if err != nil || identity.OpenID != "ou_123" || identity.TenantKey != "tenant_123" ||
		identity.DisplayName != "Alice" || identity.AvatarURL != "https://avatar.test/alice.png" || !sawToken {
		t.Fatalf("identity=%+v err=%v sawToken=%v", identity, err, sawToken)
	}
}

func TestExchangeRejectsProfileWithoutTenantKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/authen/v2/oauth/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"access_token": "provider-token"}})
		case "/open-apis/authen/v1/user_info":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"open_id": "ou_123"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(config.FeishuLogin{AppID: "app", AppSecret: "secret", BaseURL: server.URL, RedirectURL: "https://pm.example.test/callback"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Exchange(context.Background(), "provider-code"); err != ErrMalformed {
		t.Fatalf("err=%v want=%v", err, ErrMalformed)
	}
}

func TestExchangeRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxResponseBytes+1)))
	}))
	defer server.Close()
	client, err := New(config.FeishuLogin{AppID: "app", AppSecret: "secret", BaseURL: server.URL, RedirectURL: "https://pm.example.test/callback"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Exchange(context.Background(), "provider-code"); err != ErrMalformed {
		t.Fatalf("err=%v want=%v", err, ErrMalformed)
	}
}
