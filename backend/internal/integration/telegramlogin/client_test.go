package telegramlogin

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

const testVerifier = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestClientAuthorizationURLUsesMinimalScopesNonceAndPKCES256(t *testing.T) {
	challengeBytes := sha256.Sum256([]byte(testVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])
	client := &Client{
		oauth: oauth2.Config{
			ClientID: "123456789", RedirectURL: "https://pm.example.com/api/v1/core/auth/telegram/callback",
			Endpoint: oauth2.Endpoint{AuthURL: telegramIssuer + "/auth"}, Scopes: []string{oidc.ScopeOpenID, "profile"},
		},
		verifier: telegramNoopVerifier{}, httpClient: http.DefaultClient,
	}
	location, err := client.AuthorizationURL("state-value", "nonce-value", challenge)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("state") != "state-value" || query.Get("nonce") != "nonce-value" ||
		query.Get("scope") != "openid profile" || query.Get("response_type") != "code" ||
		query.Get("code_challenge") != challenge || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("query=%v", query)
	}
	if strings.Contains(query.Get("scope"), "phone") || strings.Contains(query.Get("scope"), "telegram:bot_access") {
		t.Fatalf("scope was not minimal: %q", query.Get("scope"))
	}
}

func TestClientExchangeUsesBasicAuthVerifierAndVerifiedIDTokenOnly(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rawToken := signedTelegramToken(t, privateKey, jwt.MapClaims{
		"iss": telegramIssuer, "sub": "telegram-subject", "aud": "123456789",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "nonce": "nonce-value",
		"id": int64(987654321), "name": "Telegram User", "preferred_username": "tg_user", "picture": "https://t.me/i/userpic/photo.jpg",
	})
	server := telegramTokenServer(t, rawToken)
	defer server.Close()
	client := telegramExchangeClient(server, &privateKey.PublicKey)
	got, err := client.Exchange(context.Background(), "authorization-code", testVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if got.Issuer != telegramIssuer || got.Subject != "telegram-subject" || got.TelegramUserID != 987654321 ||
		got.Name != "Telegram User" || got.PreferredUsername != "tg_user" || got.Nonce != "nonce-value" {
		t.Fatalf("identity=%+v", got)
	}
}

func TestClientExchangeRejectsInvalidAudienceSignatureAndExpiry(t *testing.T) {
	trusted, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	untrusted, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	base := jwt.MapClaims{"iss": telegramIssuer, "sub": "subject", "aud": "123456789", "exp": time.Now().Add(time.Hour).Unix(), "nonce": "nonce", "id": 1}
	wrongAudience := cloneClaims(base)
	wrongAudience["aud"] = "other-client"
	expired := cloneClaims(base)
	expired["exp"] = time.Now().Add(-time.Hour).Unix()
	for name, rawToken := range map[string]string{
		"audience":  signedTelegramToken(t, trusted, wrongAudience),
		"signature": signedTelegramToken(t, untrusted, base),
		"expiry":    signedTelegramToken(t, trusted, expired),
	} {
		t.Run(name, func(t *testing.T) {
			server := telegramTokenServer(t, rawToken)
			defer server.Close()
			if _, err := telegramExchangeClient(server, &trusted.PublicKey).Exchange(context.Background(), "code", testVerifier); err != ErrRejected {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

type telegramNoopVerifier struct{}

func (telegramNoopVerifier) Verify(context.Context, string) (*oidc.IDToken, error) { return nil, nil }

func telegramTokenServer(t *testing.T, rawIDToken string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		clientID, clientSecret, ok := request.BasicAuth()
		if !ok || clientID != "123456789" || clientSecret != "client-secret" {
			t.Errorf("missing Telegram Basic client authentication")
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := request.ParseForm(); err != nil || request.Form.Get("grant_type") != "authorization_code" ||
			request.Form.Get("code") == "" || request.Form.Get("redirect_uri") != "https://pm.example.com/callback" ||
			request.Form.Get("code_verifier") != testVerifier || request.Form.Get("client_id") != "123456789" {
			t.Errorf("token form=%v err=%v", request.Form, err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"access_token": "unused-access-token", "token_type": "Bearer", "expires_in": 3600, "id_token": rawIDToken,
		})
	}))
}

func telegramExchangeClient(server *httptest.Server, publicKey *rsa.PublicKey) *Client {
	return &Client{
		oauth: oauth2.Config{
			ClientID: "123456789", ClientSecret: "client-secret", RedirectURL: "https://pm.example.com/callback",
			Endpoint: oauth2.Endpoint{TokenURL: server.URL, AuthStyle: oauth2.AuthStyleInHeader},
		},
		verifier:   oidc.NewVerifier(telegramIssuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{publicKey}}, &oidc.Config{ClientID: "123456789"}),
		httpClient: server.Client(),
	}
}

func signedTelegramToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	raw, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneClaims(source jwt.MapClaims) jwt.MapClaims {
	copy := make(jwt.MapClaims, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
