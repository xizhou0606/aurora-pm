package toolruntimehttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOAuthRegistrationJSONAcceptsGooseAndIgnoresUnknownRFC7591Metadata(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/tool-auth/register", strings.NewReader(`{
		"client_name":"goose",
		"redirect_uris":["http://127.0.0.1:4567/oauth_callback"],
		"grant_types":["authorization_code","refresh_token"],
		"response_types":["code"],
		"token_endpoint_auth_method":"none",
		"client_uri":"https://goose-docs.ai",
		"software_id":"goose",
		"software_version":"1.35"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")

	var body struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if !oauthRegistrationJSON(context, &body) {
		t.Fatalf("registration metadata should be accepted: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if body.ClientName != "goose" || len(body.RedirectURIs) != 1 ||
		body.RedirectURIs[0] != "http://127.0.0.1:4567/oauth_callback" {
		t.Fatalf("unexpected decoded metadata: %#v", body)
	}
}

func TestOAuthRegistrationJSONRejectsTrailingDocument(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/tool-auth/register", strings.NewReader(`{} {}`))

	var body map[string]any
	if oauthRegistrationJSON(context, &body) {
		t.Fatal("multiple JSON documents must be rejected")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}
