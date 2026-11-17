package resourceaccesshttp

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"net/http"
	"net/http/httptest"
	"testing"
)

type requestsStub struct {
	principal resourceaccess.Principal
	notify    bool
	read      *bool
	decision  string
	err       error
}

func (s *requestsStub) Context(_ context.Context, p resourceaccess.Principal, id documentlibrary.ResourceID) (resourceaccess.RequestContext, error) {
	s.principal = p
	return resourceaccess.RequestContext{ResourceID: id, Items: []resourceaccess.AccessRequest{}}, s.err
}
func (s *requestsStub) BitableContext(ctx context.Context, p resourceaccess.Principal, _ string) (resourceaccess.RequestContext, error) {
	return s.Context(ctx, p, "dr_1")
}
func (s *requestsStub) DocumentContext(ctx context.Context, p resourceaccess.Principal, _ uint) (resourceaccess.RequestContext, error) {
	return s.Context(ctx, p, "dr_1")
}
func (s *requestsStub) Create(_ context.Context, p resourceaccess.Principal, _ documentlibrary.ResourceID, _ string, notify bool) (resourceaccess.AccessRequest, error) {
	s.principal = p
	s.notify = notify
	return resourceaccess.AccessRequest{ID: 1}, nil
}
func (s *requestsStub) Update(_ context.Context, p resourceaccess.Principal, _ documentlibrary.ResourceID, _ uint, input resourceaccess.RequestReview) (resourceaccess.AccessRequest, error) {
	s.principal = p
	s.read = input.Read
	s.decision = string(input.Decision)
	return resourceaccess.AccessRequest{ID: 1}, nil
}
func TestRequestHTTPContractAndObjectPolicyIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policies := &policyApplicationStub{}
	h, _ := NewHandler(policies, &linkApplicationStub{}, subjectDirectoryStub{})
	requests := &requestsStub{}
	h.ConfigureRequests(requests)
	router := authenticatedRouter(h)
	for _, test := range []struct {
		method, path, body string
		operation          documentlibrary.Operation
	}{
		{"GET", "/document-resources/dr_1/access-requests", "", documentlibrary.OperationView},
		{"GET", "/documents/14/access-request-context", "", documentlibrary.OperationView},
		{"POST", "/document-resources/dr_1/access-requests", `{"reason":"edit"}`, documentlibrary.OperationView},
		{"PATCH", "/document-resources/dr_1/access-requests/1", `{"decision":"approved","read":false}`, documentlibrary.OperationManageAccess},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body)))
		if recorder.Code != http.StatusOK || requests.principal.WorkspaceID == 0 || requests.principal.UserID == 0 {
			t.Fatalf("%s code=%d principal=%+v", test.method, recorder.Code, requests.principal)
		}
		if test.method == "PATCH" && policies.authorized[len(policies.authorized)-1] != test.operation {
			t.Fatal("wrong object operation")
		}
		policies.err = resourceaccess.ErrNotFound
		requests.err = resourceaccess.ErrNotFound
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body)))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("inaccessible scope status=%d", recorder.Code)
		}
		policies.err = nil
		requests.err = nil
	}
	if !requests.notify || requests.read == nil || *requests.read || requests.decision != "approved" {
		t.Fatalf("default notify/read contract: %+v", requests)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("POST", "/document-resources/dr_1/access-requests", bytes.NewBufferString(`{"reason":"edit","workspace_id":2}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("client tenant injection accepted: %d", recorder.Code)
	}
}
