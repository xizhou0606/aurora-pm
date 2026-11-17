package pmtools

import (
	"context"
	"encoding/json"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"testing"
)

type documentRequestQueriesStub struct {
	p   resourceaccess.Principal
	err error
}

func (s *documentRequestQueriesStub) Context(_ context.Context, p resourceaccess.Principal, id documentlibrary.ResourceID) (resourceaccess.RequestContext, error) {
	s.p = p
	return resourceaccess.RequestContext{ResourceID: id, ResourceType: documentlibrary.TypeRichText, HasAccess: false, CanRequest: true}, s.err
}
func (s *documentRequestQueriesStub) DocumentContext(ctx context.Context, p resourceaccess.Principal, id uint) (resourceaccess.RequestContext, error) {
	return s.Context(ctx, p, "private-document")
}
func (s *documentRequestQueriesStub) BitableContext(ctx context.Context, p resourceaccess.Principal, id string) (resourceaccess.RequestContext, error) {
	return s.Context(ctx, p, documentlibrary.ResourceID(id))
}
func TestDocumentRequestToolDoesNotRequireBodyAccess(t *testing.T) {
	requests := &documentRequestQueriesStub{}
	service := &DocumentAccessTools{Requests: requests, Users: documentToolUsers{}}
	h := documentAccessReadHandler{service: service, name: "document.access_request.context"}
	auth := toolruntime.ExecutionAuthorization{PrincipalID: 5, WorkspaceID: 7}
	output, err := h.Invoke(context.Background(), toolruntime.ToolCall{Authorization: auth, Input: json.RawMessage(`{"id":11}`)})
	if err != nil {
		t.Fatal(err)
	}
	var view resourceaccess.RequestContext
	if json.Unmarshal(output, &view) != nil || view.HasAccess || !view.CanRequest || requests.p.UserID != 5 || requests.p.WorkspaceID != 7 || requests.p.ActorFactUserID != 21 {
		t.Fatalf("view=%s principal=%+v", output, requests.p)
	}
	request := toolruntime.ObjectActionRequest{PolicyCode: "document.resource-access", ToolName: h.name, Authorization: auth, Input: json.RawMessage(`{"id":11}`)}
	if allowed, err := service.AuthorizeObjectAction(context.Background(), request); err != nil || !allowed {
		t.Fatalf("request context should preserve domain policy: %v", err)
	}
	requests.err = resourceaccess.ErrNotFound
	if allowed, err := service.AuthorizeObjectAction(context.Background(), request); err != nil || allowed {
		t.Fatal("object denial bypassed")
	}
	requests.err = nil
	request.PolicyCode = "task.creator"
	if allowed, _ := service.AuthorizeObjectAction(context.Background(), request); allowed {
		t.Fatal("unrelated object policy accepted")
	}
	request.PolicyCode = "document.resource-access"
	request.ToolName = "document.owner.transfer"
	if allowed, _ := service.AuthorizeObjectAction(context.Background(), request); allowed {
		t.Fatal("query policy expanded to mutation")
	}
}
