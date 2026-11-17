package saassso

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

func TestAuthorizationURLUsesConfiguredClientAndRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	client, err := New(config.SSO{Enabled: true, AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL + "/token", Issuer: "one2all", ClientID: "pm", ClientSecret: "secret", RedirectURL: "https://pm.example.test/callback"})
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
	if parsed.Query().Get("client_id") != "pm" || parsed.Query().Get("redirect_uri") != "https://pm.example.test/callback" || parsed.Query().Get("state") != "state-value" || strings.Contains(redirect, "secret") {
		t.Fatalf("redirect=%q", redirect)
	}
}

func TestExchangeValidatesIssuerAndSubject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("request=%s content-type=%q", r.Method, r.Header.Get("Content-Type"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": "one2all", "subject": "one2all|77", "username": "alice", "display_name": "Alice", "email": "alice@example.com"})
	}))
	defer server.Close()
	client, err := New(config.SSO{Enabled: true, AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL + "/token", Issuer: "one2all", ClientID: "pm", ClientSecret: "secret", RedirectURL: "https://pm.example.test/callback"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := client.Exchange(context.Background(), "sso-code")
	if err != nil || identity.Subject != "one2all|77" || identity.Username != "alice" {
		t.Fatalf("identity=%+v err=%v", identity, err)
	}
}

func TestExchangeRejectsIssuerMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": "other", "subject": "77"})
	}))
	defer server.Close()
	client, err := New(config.SSO{Enabled: true, AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL + "/token", Issuer: "one2all", ClientID: "pm", ClientSecret: "secret", RedirectURL: "https://pm.example.test/callback"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Exchange(context.Background(), "sso-code"); err != ErrMalformed {
		t.Fatalf("err=%v want=%v", err, ErrMalformed)
	}
}
