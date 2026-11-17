package googlelogin

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func TestClientAuthorizationURLCarriesStateNonceAndOIDCScopes(t *testing.T) {
	client := &Client{
		oauth: oauth2.Config{
			ClientID: "client-id", RedirectURL: "https://pm.example.com/api/v1/core/auth/google/callback",
			Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth"},
			Scopes:   []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: noopVerifier{}, httpClient: http.DefaultClient,
	}
	location, err := client.AuthorizationURL("state-value", "nonce-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("state") != "state-value" || query.Get("nonce") != "nonce-value" ||
		query.Get("response_type") != "code" || query.Get("client_id") != "client-id" ||
		query.Get("scope") != "openid email profile" {
		t.Fatalf("authorization query=%v", query)
	}
}

func TestClientExchangeReturnsOnlyCryptographicallyVerifiedIDTokenClaims(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	validToken := signedIDToken(t, privateKey, jwt.MapClaims{
		"iss": googleIssuer, "sub": "google-subject", "aud": "client-id",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Add(-time.Minute).Unix(),
		"email": "verified@example.com", "email_verified": true, "nonce": "nonce-value",
	})
	server := tokenServer(t, validToken)
	defer server.Close()
	client := testExchangeClient(server, &privateKey.PublicKey)

	got, err := client.Exchange(context.Background(), "authorization-code")
	if err != nil {
		t.Fatal(err)
	}
	if got.Issuer != googleIssuer || got.Subject != "google-subject" || got.Email != "verified@example.com" || !got.EmailVerified || got.Nonce != "nonce-value" {
		t.Fatalf("identity=%+v", got)
	}
}

func TestClientExchangeRejectsInvalidAudienceSignatureAndExpiredTokens(t *testing.T) {
	trustedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	untrustedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{
		"audience": signedIDToken(t, trustedKey, jwt.MapClaims{
			"iss": googleIssuer, "sub": "subject", "aud": "another-client", "exp": time.Now().Add(time.Hour).Unix(),
			"email": "a@example.com", "email_verified": true, "nonce": "nonce",
		}),
		"signature": signedIDToken(t, untrustedKey, jwt.MapClaims{
			"iss": googleIssuer, "sub": "subject", "aud": "client-id", "exp": time.Now().Add(time.Hour).Unix(),
			"email": "a@example.com", "email_verified": true, "nonce": "nonce",
		}),
		"expiry": signedIDToken(t, trustedKey, jwt.MapClaims{
			"iss": googleIssuer, "sub": "subject", "aud": "client-id", "exp": time.Now().Add(-time.Hour).Unix(),
			"email": "a@example.com", "email_verified": true, "nonce": "nonce",
		}),
	} {
		t.Run(name, func(t *testing.T) {
			server := tokenServer(t, token)
			defer server.Close()
			_, err := testExchangeClient(server, &trustedKey.PublicKey).Exchange(context.Background(), "code")
			if err != ErrRejected {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

type noopVerifier struct{}

func (noopVerifier) Verify(context.Context, string) (*oidc.IDToken, error) { return nil, nil }

func signedIDToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func tokenServer(t *testing.T, rawIDToken string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil || request.Form.Get("code") == "" {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"access_token": "access-token", "token_type": "Bearer", "expires_in": 3600, "id_token": rawIDToken,
		})
	}))
}

func testExchangeClient(server *httptest.Server, publicKey *rsa.PublicKey) *Client {
	return &Client{
		oauth: oauth2.Config{
			ClientID: "client-id", ClientSecret: "client-secret", RedirectURL: "https://pm.example.com/callback",
			Endpoint: oauth2.Endpoint{TokenURL: server.URL, AuthStyle: oauth2.AuthStyleInHeader},
		},
		verifier:   oidc.NewVerifier(googleIssuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{publicKey}}, &oidc.Config{ClientID: "client-id"}),
		httpClient: server.Client(),
	}
}

var _ identity.GoogleLoginProvider = (*Client)(nil)
