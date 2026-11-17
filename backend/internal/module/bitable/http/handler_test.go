package bitablehttp

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"net/http/httptest"
	"strings"
	"testing"
)

type memoryStore struct{ calls int }

func (s *memoryStore) Get(context.Context, uint, string) (bitable.Resource, error) {
	s.calls++
	return bitable.Resource{ID: "550e8400-e29b-41d4-a716-446655440000", WorkspaceID: 1, Tables: []bitable.Table{{ID: "table"}}}, nil
}
func (s *memoryStore) List(context.Context, uint) ([]bitable.Resource, error) {
	s.calls++
	return []bitable.Resource{}, nil
}
func (s *memoryStore) Create(context.Context, bitable.Principal, string, string) (bitable.Resource, error) {
	s.calls++
	return bitable.Resource{}, nil
}

type allow struct{}

func (allow) RequireAction(context.Context, bitable.Principal, accesscatalog.ActionCode) error {
	return nil
}

type dataProvider struct{}

type resourceAccess struct{ principal *bitable.Principal }

func (gate resourceAccess) RequireBitableResource(_ context.Context, principal bitable.Principal, id documentlibrary.ResourceID, _ documentlibrary.Operation) (bitable.ResourceBinding, error) {
	if gate.principal != nil {
		*gate.principal = principal
	}
	nativeID := string(id)
	if id == "dr_bitable" {
		nativeID = "550e8400-e29b-41d4-a716-446655440000"
	}
	return bitable.ResourceBinding{ResourceID: id, NativeResourceID: nativeID, OwnerUserID: 1}, nil
}
func (resourceAccess) FilterBitableResources(_ context.Context, _ bitable.Principal, ids []string, _ documentlibrary.Operation) (map[string]bitable.ResourceBinding, error) {
	result := map[string]bitable.ResourceBinding{}
	for _, id := range ids {
		result[id] = bitable.ResourceBinding{ResourceID: documentlibrary.ResourceID(id), NativeResourceID: id, OwnerUserID: 1}
	}
	return result, nil
}
func (resourceAccess) AuthorizeBitableResource(context.Context, bitable.Principal, bitable.ResourceBinding, documentlibrary.Operation) error {
	return nil
}
func (resourceAccess) BatchAuthorizeBitableResources(_ context.Context, _ bitable.Principal, bindings []bitable.ResourceBinding, operations []documentlibrary.Operation) (map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool, error) {
	result := map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool{}
	for _, binding := range bindings {
		result[binding.ResourceID] = map[documentlibrary.Operation]bool{}
		for _, operation := range operations {
			result[binding.ResourceID][operation] = true
		}
	}
	return result, nil
}
func (resourceAccess) RequireBitableCreate(context.Context, bitable.Principal) error { return nil }

func (dataProvider) Bootstrap(context.Context, bitable.Scope) (bitable.Bootstrap, error) {
	return bitable.Bootstrap{}, nil
}
func (dataProvider) Query(context.Context, bitable.Scope, bitable.Query) (bitable.RecordPage, error) {
	return bitable.RecordPage{Records: []bitable.Record{}}, nil
}
func (dataProvider) Search(context.Context, bitable.Scope, bitable.Search) (bitable.SearchPage, error) {
	return bitable.SearchPage{}, nil
}
func (dataProvider) Execute(context.Context, bitable.Scope, bitable.CommandBatch) (bitable.Operation, error) {
	return bitable.Operation{}, nil
}
func (dataProvider) Operation(context.Context, bitable.Scope, string) (bitable.Operation, error) {
	return bitable.Operation{}, nil
}
func (dataProvider) Export(context.Context, bitable.Scope, bitable.ExportQuery) (bitable.ExportSnapshot, error) {
	return bitable.ExportSnapshot{}, nil
}
func router(auth bool) (*gin.Engine, *memoryStore) {
	return routerWithAccess(auth, resourceAccess{})
}

func routerWithAccess(auth bool, access resourceAccess) (*gin.Engine, *memoryStore) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if auth {
		r.Use(func(c *gin.Context) {
			c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 1})
			c.Set(pmauth.WorkspaceIDContextKey, uint(1))
			c.Next()
		})
	}
	store := &memoryStore{}
	s, _ := bitable.NewService(store, store, allow{}, dataProvider{})
	s.ConfigureUnifiedResourceAccess(access, nil)
	h, _ := NewHandler(s)
	h.RegisterRoutes(r.Group("/api/v1/platform"))
	return r, store
}

func TestHTTPForwardsBoundedTrimmedLinkAccessSecret(t *testing.T) {
	var captured bitable.Principal
	r, store := routerWithAccess(true, resourceAccess{principal: &captured})
	request := httptest.NewRequest("GET", "/api/v1/platform/bitables/dr_bitable", nil)
	request.Header.Set("X-PM-Resource-Access", "  opaque-link-secret  ")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 200 || captured.LinkAccessSecret != "opaque-link-secret" {
		t.Fatalf("link secret not forwarded opaquely: %d %#v", response.Code, captured)
	}

	request = httptest.NewRequest("GET", "/api/v1/platform/bitables/dr_bitable", nil)
	request.Header.Set("X-PM-Resource-Access", strings.Repeat("x", 257))
	response = httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 400 || store.calls != 1 {
		t.Fatalf("oversized link secret reached service: %d calls=%d", response.Code, store.calls)
	}
}
func TestHTTPRequiresCanonicalPrincipal(t *testing.T) {
	r, store := router(false)
	req := httptest.NewRequest("GET", "/api/v1/platform/bitables", nil)
	req.Header.Set("X-Workspace-ID", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 || store.calls != 0 {
		t.Fatal("headers forged scope", w.Code, store.calls)
	}
}
func TestHTTPRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"name":"x","actorId":2}`, `{"name":"x"} {}`, `{"name":"x","workspaceId":2}`} {
		r, store := router(true)
		req := httptest.NewRequest("POST", "/api/v1/platform/bitables", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 || store.calls != 0 {
			t.Fatal("invalid body reached storage", w.Code, store.calls)
		}
	}
}
func TestHTTPQueryBudgetAndResponseEnvelope(t *testing.T) {
	r, _ := router(true)
	url := "/api/v1/platform/bitables/dr_bitable/tables/table/records/query"
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"viewId":"view","offset":0,"limit":201}`, 400}, {`{"viewId":"view","offset":0,"limit":10}`, 200}} {
		req := httptest.NewRequest("POST", url, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private response cached")
		}
		if tc.status == 200 && !strings.Contains(w.Body.String(), `"data"`) {
			t.Fatal("missing envelope")
		}
	}
}

func TestHTTPOldRowRevisionCellWriteIsRejected(t *testing.T) {
	r, _ := router(true)
	url := "/api/v1/platform/bitables/dr_bitable/tables/table/commands"
	oldBody := `{"operationId":"550e8400-e29b-41d4-a716-446655440000","expectedSchemaRevision":1,"atomic":true,"commands":[{"kind":"update","recordId":"record-1","expectedRevision":1,"values":{"field-1":"after"}}]}`
	request := httptest.NewRequest("POST", url, strings.NewReader(oldBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 400 || !strings.Contains(response.Body.String(), "BITABLE_INVALID_INPUT") {
		t.Fatalf("old row-CAS write was not rejected: %d %s", response.Code, response.Body.String())
	}

	newBody := `{"operationId":"550e8400-e29b-41d4-a716-446655440000","expectedSchemaRevision":1,"atomic":true,"commands":[{"kind":"update","recordId":"record-1","values":{"field-1":"after"},"cellContracts":[{"recordId":"record-1","fieldId":"field-1","baseRevision":1,"leaseId":"550e8400-e29b-41d4-a716-446655440001","fencingToken":1,"connectionId":"connection-1","editorInstanceId":"editor-1"}]}]}`
	request = httptest.NewRequest("POST", url, strings.NewReader(newBody))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("new field-base write rejected: %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPResourceProjectsUnifiedIDNotNativeUUID(t *testing.T) {
	r, _ := router(true)
	request := httptest.NewRequest("GET", "/api/v1/platform/bitables/dr_bitable", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != 200 || !strings.Contains(body, `"id":"dr_bitable"`) ||
		!strings.Contains(body, `"resource_id":"dr_bitable"`) ||
		strings.Contains(body, `"id":"550e8400-e29b-41d4-a716-446655440000"`) {
		t.Fatalf("native UUID escaped resource projection: %d %s", response.Code, body)
	}
}
