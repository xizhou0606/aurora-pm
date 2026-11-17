package resourceaccesshttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type policyApplicationStub struct {
	getCalls      int
	lastPut       resourceaccess.ReplacePolicyInput
	authorized    []documentlibrary.Operation
	lastPrincipal resourceaccess.Principal
	err           error
}

func (stub *policyApplicationStub) AuthorizeOperation(_ context.Context, principal resourceaccess.Principal, _ documentlibrary.ResourceID, operation documentlibrary.Operation) error {
	stub.authorized = append(stub.authorized, operation)
	stub.lastPrincipal = principal
	return stub.err
}

func (stub *policyApplicationStub) GetPolicy(_ context.Context, _ resourceaccess.Principal, id documentlibrary.ResourceID) (resourceaccess.PolicyView, error) {
	stub.getCalls++
	return resourceaccess.PolicyView{Policy: resourceaccess.Policy{ResourceID: id, Mode: resourceaccess.ModePrivate, Rules: []resourceaccess.Rule{}, Revision: 1}}, stub.err
}
func (stub *policyApplicationStub) ReplacePolicy(_ context.Context, _ resourceaccess.Principal, id documentlibrary.ResourceID, input resourceaccess.ReplacePolicyInput) (resourceaccess.PolicyView, error) {
	stub.lastPut = input
	return resourceaccess.PolicyView{Policy: resourceaccess.Policy{ResourceID: id, Mode: input.Mode, Rules: []resourceaccess.Rule{}, Revision: 2}}, stub.err
}

type linkApplicationStub struct {
	createInput resourceaccess.CreateShareLinkInput
}

func (stub *linkApplicationStub) Create(_ context.Context, _ resourceaccess.Principal, id documentlibrary.ResourceID, input resourceaccess.CreateShareLinkInput) (resourceaccess.CreatedShareLink, error) {
	stub.createInput = input
	return resourceaccess.CreatedShareLink{ShareLink: resourceaccess.ShareLink{ID: "lnk_1", ResourceID: id, Level: input.Level}, Secret: "once"}, nil
}
func (stub *linkApplicationStub) List(context.Context, resourceaccess.Principal, documentlibrary.ResourceID) ([]resourceaccess.ShareLink, error) {
	return []resourceaccess.ShareLink{{ID: "lnk_1"}}, nil
}
func (stub *linkApplicationStub) Revoke(context.Context, resourceaccess.Principal, documentlibrary.ResourceID, string) error {
	return nil
}
func (stub *linkApplicationStub) Rotate(_ context.Context, _ resourceaccess.Principal, id documentlibrary.ResourceID, linkID string) (resourceaccess.CreatedShareLink, error) {
	return resourceaccess.CreatedShareLink{ShareLink: resourceaccess.ShareLink{ID: linkID, ResourceID: id}, Secret: "rotated"}, nil
}
func (stub *linkApplicationStub) Exchange(context.Context, resourceaccess.Principal, string) (resourceaccess.ExchangeResult, error) {
	return resourceaccess.ExchangeResult{ResourceID: "dr_1", AccessSecret: "access"}, nil
}

type subjectDirectoryStub struct{}

func (subjectDirectoryStub) SearchSubjects(_ context.Context, _ uint, search identity.SubjectSearch) (identity.SubjectPage, error) {
	return identity.SubjectPage{Items: []identity.SubjectCandidate{{Kind: search.Kind, ID: "7", DisplayName: "User"}}, Page: 1, PageSize: 20, Total: 1}, nil
}
func (subjectDirectoryStub) ResolveSubjects(context.Context, uint, []identity.SubjectRef) ([]identity.SubjectCandidate, error) {
	return nil, nil
}
func (subjectDirectoryStub) MatchSubjects(context.Context, identity.SubjectMatchRequest) (identity.SubjectMatchResult, error) {
	return identity.SubjectMatchResult{}, nil
}
func (subjectDirectoryStub) SubjectCapabilities() map[identity.SubjectKind]identity.SubjectKindCapability {
	return map[identity.SubjectKind]identity.SubjectKindCapability{
		identity.SubjectUser: {Available: true}, identity.SubjectOrganizationUnit: {Available: false},
	}
}

func TestResourceRoutesUsePolicyMarkerAndFrozenLinkContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policies, links := &policyApplicationStub{}, &linkApplicationStub{}
	handler, err := NewHandler(policies, links, subjectDirectoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	router := authenticatedRouter(handler)
	tests := []struct {
		method string
		path   string
		body   string
		header string
	}{
		{http.MethodGet, "/document-resources/dr_1/access-policy", "", ""},
		{http.MethodPut, "/document-resources/dr_1/access-policy", `{"mode":"private","rules":[],"expected_policy_revision":1}`, "policy-key"},
		{http.MethodGet, "/document-resources/dr_1/share-links", "", ""},
		{http.MethodPost, "/document-resources/dr_1/share-links", `{"level":"viewer","expires_in":"24h"}`, "link-key"},
		{http.MethodDelete, "/document-resources/dr_1/share-links/lnk_1", "", ""},
		{http.MethodPost, "/document-resources/dr_1/share-links/lnk_1/rotate", "", ""},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
		request.Header.Set("Content-Type", "application/json")
		if test.header != "" {
			request.Header.Set("Idempotency-Key", test.header)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code < 200 || response.Code >= 300 {
			t.Fatalf("%s %s: status %d body %s", test.method, test.path, response.Code, response.Body.String())
		}
	}
	if policies.getCalls != 1 {
		t.Fatalf("only policy GET should load its projection, got %d", policies.getCalls)
	}
	wantOperations := []documentlibrary.Operation{
		documentlibrary.OperationViewAccess, documentlibrary.OperationManageAccess,
		documentlibrary.OperationViewLinks, documentlibrary.OperationManageLinks,
		documentlibrary.OperationManageLinks, documentlibrary.OperationManageLinks,
	}
	if len(policies.authorized) != len(wantOperations) {
		t.Fatalf("every resource route must call marker: got %+v", policies.authorized)
	}
	for index, want := range wantOperations {
		if policies.authorized[index] != want {
			t.Fatalf("marker operation %d = %s, want %s", index, policies.authorized[index], want)
		}
	}
	if policies.lastPut.IdempotencyKey != "policy-key" {
		t.Fatalf("PUT did not forward Idempotency-Key: %+v", policies.lastPut)
	}
	if links.createInput.Duration != resourceaccess.LinkDuration24Hours || links.createInput.IdempotencyKey != "link-key" {
		t.Fatalf("create contract did not use expires_in/header: %+v", links.createInput)
	}
	if policies.lastPrincipal.UserID != 7 || policies.lastPrincipal.ActorFactUserID != 41 {
		t.Fatalf("principal keys not preserved: %+v", policies.lastPrincipal)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/document-resources/dr_1/share-links", nil))
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) == 0 || payload.Data[0] != '[' {
		t.Fatalf("link list data must be an array, got %s", payload.Data)
	}
}

func TestSubjectSearchProjectsIdentityCapabilities(t *testing.T) {
	handler, _ := NewHandler(&policyApplicationStub{}, &linkApplicationStub{}, subjectDirectoryStub{})
	response := httptest.NewRecorder()
	authenticatedRouter(handler).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/document-resource-subjects?kind=user&limit=20&offset=0", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Capabilities map[identity.SubjectKind]identity.SubjectKindCapability `json:"subject_capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Data.Capabilities[identity.SubjectUser].Available || payload.Data.Capabilities[identity.SubjectOrganizationUnit].Available ||
		payload.Data.Capabilities[identity.SubjectRole].Available {
		t.Fatalf("subject capabilities did not fail closed: %+v", payload.Data.Capabilities)
	}
}

func TestRequestPrincipalFallsBackActorFactToPMUserID(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set(pmauth.PrincipalContextKey, pmauth.PMPrincipal(7, "user", "User", ""))
	context.Set(pmauth.WorkspaceIDContextKey, uint(1))
	principal, ok := requestPrincipal(context)
	if !ok || principal.UserID != 7 || principal.ActorFactUserID != 7 {
		t.Fatalf("missing compatibility fact must fall back to PM user ID: %+v %v", principal, ok)
	}
}

func authenticatedRouter(handler *Handler) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(pmauth.PrincipalContextKey, pmauth.PMPrincipal(7, "user", "User", ""))
		c.Set(pmauth.WorkspaceIDContextKey, uint(1))
		c.Set(pmauth.ActorFactUserIDContextKey, uint(41))
		c.Next()
	})
	handler.RegisterRoutes(router.Group(""))
	return router
}
