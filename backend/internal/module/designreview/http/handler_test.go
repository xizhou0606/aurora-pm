package designreviewhttp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type handlerRepository struct {
	designreview.Repository
	listWorkspace uint
	detail        designreview.DesignReviewDetail
	detailErr     error
	authHash      string
}

func (repository *handlerRepository) ListReviews(_ context.Context, workspaceID uint, _ designreview.ReviewListFilter) ([]designreview.DesignReviewSummary, int64, error) {
	repository.listWorkspace = workspaceID
	return nil, 0, nil
}

func (repository *handlerRepository) GetReview(context.Context, uint, string) (designreview.DesignReviewDetail, error) {
	return repository.detail, repository.detailErr
}

func (repository *handlerRepository) AuthenticateCollector(_ context.Context, tokenHash string) (designreview.MobileCollector, error) {
	repository.authHash = tokenHash
	return designreview.MobileCollector{ID: "collector", WorkspaceID: 7, UserID: 42}, nil
}

func (*handlerRepository) ClaimCaptureDraft(context.Context, string) (*designreview.MobileCaptureDraft, error) {
	return nil, nil
}

type handlerProjectAccess struct {
	allowed bool
	admin   bool
}

func (access handlerProjectAccess) CanAccessProject(context.Context, uint, uint, uint) (bool, error) {
	return access.allowed, nil
}

func (access handlerProjectAccess) IsAccessAdministrator(context.Context, uint, uint) (bool, error) {
	return access.admin, nil
}

type handlerActionReader struct {
	codes []string
	users *[]uint
}

func (reader handlerActionReader) EffectiveActionCodes(_ context.Context, userID uint) ([]string, error) {
	if reader.users != nil {
		*reader.users = append(*reader.users, userID)
	}
	return append([]string(nil), reader.codes...), nil
}

func newHandlerEngine(t *testing.T, repository *handlerRepository, actions []string, projects handlerProjectAccess, authenticated bool) *gin.Engine {
	return newHandlerEngineWithIdentity(t, repository, handlerActionReader{codes: actions}, projects, authenticated, 42, 42)
}

func newHandlerEngineWithIdentity(t *testing.T, repository *handlerRepository, actions handlerActionReader, projects handlerProjectAccess, authenticated bool, principalUserID, factUserID uint) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service, err := designreview.NewService(repository, projects, actions, nil, nil, nil, designreview.Availability{})
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	if authenticated {
		engine.Use(func(c *gin.Context) {
			c.Set(pmauth.WorkspaceContextKey, identitygorm.Workspace{ID: 7})
			c.Set(pmauth.ActorFactUserIDContextKey, factUserID)
			c.Set(pmauth.PrincipalContextKey, pmauth.PMPrincipal(principalUserID, "user", "PM User", ""))
			c.Next()
		})
	}
	handler := NewHandler(service)
	handler.RegisterRoutes(engine.Group("/api/v1/platform"))
	handler.RegisterCollectorRoutes(engine.Group("/api/v1/platform/design-review-mobile-captures"))
	return engine
}

func TestCapabilitiesUsePrincipalIdentityFromRequestContext(t *testing.T) {
	var actionUsers []uint
	reader := handlerActionReader{
		codes: []string{string(accesscatalog.ActionDesignReviewsReviewsView)},
		users: &actionUsers,
	}
	engine := newHandlerEngineWithIdentity(t, &handlerRepository{}, reader, handlerProjectAccess{}, true, 109, 1)

	response := performRequest(engine, http.MethodGet, "/api/v1/platform/design-reviews/capabilities", "", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(actionUsers) != 1 || actionUsers[0] != 109 {
		t.Fatalf("RBAC action lookup users=%v want=[109]", actionUsers)
	}
	if !strings.Contains(response.Body.String(), `"can_view":true`) {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func performRequest(engine http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func TestDesignReviewRoutesRequireAuthenticatedWorkspace(t *testing.T) {
	engine := newHandlerEngine(t, &handlerRepository{}, nil, handlerProjectAccess{}, false)
	response := performRequest(engine, http.MethodGet, "/api/v1/platform/design-reviews", "", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDesignReviewListUsesContextWorkspaceAndFailsClosedWithoutAction(t *testing.T) {
	view := string(accesscatalog.ActionDesignReviewsReviewsView)
	for _, test := range []struct {
		name       string
		actions    []string
		wantStatus int
	}{
		{name: "authorized", actions: []string{view}, wantStatus: http.StatusOK},
		{name: "missing action", wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &handlerRepository{}
			engine := newHandlerEngine(t, repository, test.actions, handlerProjectAccess{}, true)
			response := performRequest(engine, http.MethodGet, "/api/v1/platform/design-reviews", "", nil)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.wantStatus == http.StatusOK && repository.listWorkspace != 7 {
				t.Fatalf("repository workspace=%d", repository.listWorkspace)
			}
		})
	}
}

func TestDesignReviewJSONContractsRejectUnknownFields(t *testing.T) {
	engine := newHandlerEngine(t, &handlerRepository{}, nil, handlerProjectAccess{}, true)
	tests := []struct {
		path string
		body string
	}{
		{path: "/api/v1/platform/design-review-differences", body: `{"review_id":"review","unknown":true}`},
		{path: "/api/v1/platform/design-review-deep-reviews", body: `{"review_id":"review","design_region":{"x":0,"y":0,"width":1,"height":1},"implementation_region":{"x":0,"y":0,"width":1,"height":1},"focus":"","retry_of_id":"forged"}`},
	}
	for _, test := range tests {
		response := performRequest(engine, http.MethodPost, test.path, test.body, nil)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

func TestReviewDetailMapsScopeAndDomainErrors(t *testing.T) {
	view := []string{string(accesscatalog.ActionDesignReviewsReviewsView)}
	tests := []struct {
		name       string
		repository *handlerRepository
		projects   handlerProjectAccess
		wantStatus int
	}{
		{name: "not found", repository: &handlerRepository{detailErr: designreview.ErrNotFound}, wantStatus: http.StatusNotFound},
		{name: "project forbidden", repository: &handlerRepository{detail: designreview.DesignReviewDetail{DesignReviewSummary: designreview.DesignReviewSummary{ProjectSetID: 99}}}, wantStatus: http.StatusForbidden},
		{name: "unavailable", repository: &handlerRepository{detailErr: designreview.ErrUnavailable}, wantStatus: http.StatusServiceUnavailable},
		{name: "unexpected", repository: &handlerRepository{detailErr: errors.New("database secret")}, wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := newHandlerEngine(t, test.repository, view, test.projects, true)
			response := performRequest(engine, http.MethodGet, "/api/v1/platform/design-reviews/11111111-1111-4111-8111-111111111111", "", nil)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.name == "unexpected" && strings.Contains(response.Body.String(), "database secret") {
				t.Fatalf("internal error leaked: %s", response.Body.String())
			}
		})
	}
}

func TestCollectorRoutesAcceptOnlyCollectorAuthorizationHeader(t *testing.T) {
	repository := &handlerRepository{}
	engine := newHandlerEngine(t, repository, nil, handlerProjectAccess{}, false)
	path := "/api/v1/platform/design-review-mobile-captures/collector/claim"
	for name, header := range map[string]string{"missing": "", "bearer": "Bearer secret"} {
		t.Run(name, func(t *testing.T) {
			response := performRequest(engine, http.MethodPost, path, "", map[string]string{"Authorization": header})
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	response := performRequest(engine, http.MethodPost, path, "", map[string]string{"Authorization": "Collector secret"})
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	digest := sha256.Sum256([]byte("secret"))
	if repository.authHash != hex.EncodeToString(digest[:]) {
		t.Fatalf("collector auth hash=%q", repository.authHash)
	}
}
