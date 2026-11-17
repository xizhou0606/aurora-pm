package toolruntimehttp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
)

type credentialHTTPGrantRepository struct{ grant toolruntime.Grant }

func (*credentialHTTPGrantRepository) Create(context.Context, *toolruntime.Grant) error { return nil }
func (repository *credentialHTTPGrantRepository) FindByID(_ context.Context, id uint) (toolruntime.Grant, error) {
	if id != repository.grant.ID {
		return toolruntime.Grant{}, toolruntime.ErrGrantInvalid
	}
	return repository.grant, nil
}
func (repository *credentialHTTPGrantRepository) Revoke(_ context.Context, _ uint, _ time.Time) (toolruntime.Grant, error) {
	return repository.grant, nil
}

type credentialHTTPRequestRepository struct {
	request toolruntime.AuthorizationRequest
}

func (*credentialHTTPRequestRepository) Create(context.Context, *toolruntime.AuthorizationRequest) error {
	return nil
}
func (repository *credentialHTTPRequestRepository) FindByID(_ context.Context, id string) (toolruntime.AuthorizationRequest, error) {
	if id != repository.request.ID {
		return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationRequestNotFound
	}
	return repository.request, nil
}
func (*credentialHTTPRequestRepository) Decide(context.Context, toolruntime.AuthorizationDecisionCommand) (toolruntime.AuthorizationRequest, error) {
	return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationDecisionInvalid
}

type credentialHTTPFixture struct {
	engine    *gin.Engine
	requestID string
	verifier  string
	challenge string
}

func newCredentialHTTPFixture(t *testing.T) credentialHTTPFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(toolruntimegorm.ExternalCredentialMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	grant, err := toolruntime.NewGrant(toolruntime.GrantRequest{
		PrincipalID: 7, WorkspaceID: 11, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
		AllowedTools: []string{"task.get"}, AllowedScopes: []string{"task:read"},
		EffectiveFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 13
	requestID := "http_authorization_request_12345"
	decidedAt := now.Add(-time.Second)
	request := toolruntime.AuthorizationRequest{
		ID: requestID, PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "PM Dev",
		ClientID: "pm-cli", ClientName: "PM CLI", Channel: toolruntime.DeliveryCLI,
		ConfigurationMode: toolruntime.WorkspaceConfigurationWindow, ConfigurationVersion: 1,
		ToolNames: grant.AllowedTools, GrantScopes: grant.AllowedScopes,
		EffectiveFrom: grant.EffectiveFrom, ExpiresAt: grant.ExpiresAt, RequestExpiresAt: now.Add(5 * time.Minute),
		Decision: toolruntime.AuthorizationAuthorize, DecidedAt: &decidedAt, GrantID: &grant.ID,
		CreatedAt: now.Add(-time.Minute), UpdatedAt: decidedAt,
	}
	service, err := toolruntime.NewCredentialService(
		&credentialHTTPGrantRepository{grant: grant}, &credentialHTTPRequestRepository{request: request},
		toolruntimegorm.NewCredentialRepository(db),
		toolruntime.WorkspaceMembershipVerifierFunc(func(context.Context, uint, uint) error { return nil }),
		toolruntime.DefaultCredentialPolicy(),
	)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewCredentialHandler(service, ManagementPrincipalResolverFunc(func(*gin.Context) (toolruntime.ManagementPrincipal, error) {
		return toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "PM Dev"}, nil
	}))
	handler.RegisterManagementRoutes(engine.Group("/api/v1/platform"))
	handler.RegisterPublicRoutes(engine.Group("/api/v1/tool-auth"))
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	digest := sha256.Sum256([]byte(verifier))
	return credentialHTTPFixture{
		engine: engine, requestID: requestID, verifier: verifier,
		challenge: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
}

func (fixture credentialHTTPFixture) request(method, target, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	fixture.engine.ServeHTTP(response, request)
	return response
}

func TestCredentialHTTPAuthorizationCodeAndTokenExchange(t *testing.T) {
	fixture := newCredentialHTTPFixture(t)
	issueBody := `{"code_challenge":"` + fixture.challenge + `","code_challenge_method":"S256","redirect_uri":"http://127.0.0.1:4321/callback"}`
	issued := fixture.request(http.MethodPost, "/api/v1/platform/external-tooling/authorization-requests/"+fixture.requestID+"/code", issueBody)
	if issued.Code != http.StatusCreated || issued.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", issued.Code, issued.Header(), issued.Body.String())
	}
	var issuePayload struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(issued.Body.Bytes(), &issuePayload); err != nil || issuePayload.Data.Code == "" {
		t.Fatalf("payload=%s err=%v", issued.Body.String(), err)
	}
	exchangeBody := `{"grant_type":"authorization_code","code":"` + issuePayload.Data.Code +
		`","code_verifier":"` + fixture.verifier +
		`","redirect_uri":"http://127.0.0.1:4321/callback","client_id":"pm-cli","channel":"cli"}`
	exchanged := fixture.request(http.MethodPost, "/api/v1/tool-auth/token", exchangeBody)
	if exchanged.Code != http.StatusOK || exchanged.Header().Get("Cache-Control") != "no-store" ||
		!strings.Contains(exchanged.Body.String(), `"token_type":"Bearer"`) {
		t.Fatalf("status=%d headers=%v body=%s", exchanged.Code, exchanged.Header(), exchanged.Body.String())
	}
}

func TestCredentialHTTPRejectsBrowserSuppliedGrantFactsAndHidesSecrets(t *testing.T) {
	fixture := newCredentialHTTPFixture(t)
	secret := strings.Repeat("s", 48)
	body := `{"code_challenge":"` + fixture.challenge +
		`","code_challenge_method":"S256","redirect_uri":"http://127.0.0.1:4321/callback","grant_id":99,"access_token":"` + secret + `"}`
	response := fixture.request(http.MethodPost, "/api/v1/platform/external-tooling/authorization-requests/"+fixture.requestID+"/code", body)
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
