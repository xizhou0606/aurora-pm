package toolruntime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

type effectiveActionReaderStub struct {
	codes []string
	err   error
	calls int
}

func (stub *effectiveActionReaderStub) EffectiveActionCodes(context.Context, uint) ([]string, error) {
	stub.calls++
	return append([]string(nil), stub.codes...), stub.err
}

type objectActionAuthorizerStub struct {
	request ObjectActionRequest
	allowed bool
}

func (stub *objectActionAuthorizerStub) AuthorizeObjectAction(_ context.Context, request ObjectActionRequest) (bool, error) {
	stub.request = request
	return stub.allowed, nil
}

func TestCatalogBusinessActionAuthorizerUsesOnePersonnelReadAndCreatorFallback(t *testing.T) {
	actions := &effectiveActionReaderStub{}
	objects := &objectActionAuthorizerStub{allowed: true}
	authorizer, err := NewCatalogBusinessActionAuthorizer(actions, objects)
	if err != nil {
		t.Fatal(err)
	}
	authorization := ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}
	decisions, err := authorizer.PreflightBusinessActions(
		context.Background(), []string{"task.update", "task.delete", "user.me"}, authorization,
	)
	if err != nil || actions.calls != 1 {
		t.Fatalf("decisions=%+v calls=%d err=%v", decisions, actions.calls, err)
	}
	if decisions["task.update"] != BusinessActionConditional || decisions["task.delete"] != BusinessActionConditional ||
		decisions["user.me"] != BusinessActionAllowed {
		t.Fatalf("decisions=%+v", decisions)
	}
	decision, err := authorizer.AuthorizeBusinessAction(context.Background(), BusinessActionRequest{
		ToolName: "task.update", Authorization: authorization, Input: json.RawMessage(`{"id":11,"title":"owned"}`),
	})
	if err != nil || decision != BusinessActionAllowed || objects.request.PolicyCode != "task.creator" ||
		objects.request.ActionCode != string(accesscatalog.ActionWorkItemsTasksUpdate) {
		t.Fatalf("decision=%s request=%+v err=%v", decision, objects.request, err)
	}
}

func TestCatalogBusinessActionAuthorizerPrefersGlobalAction(t *testing.T) {
	actions := &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionWorkItemsTasksTransitionStatus)}}
	objects := &objectActionAuthorizerStub{}
	authorizer, err := NewCatalogBusinessActionAuthorizer(actions, objects)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := authorizer.AuthorizeBusinessAction(context.Background(), BusinessActionRequest{
		ToolName: "task.status.update", Authorization: ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input: json.RawMessage(`{"id":11,"status":"in_dev"}`),
	})
	if err != nil || decision != BusinessActionAllowed || objects.request.PolicyCode != "" {
		t.Fatalf("decision=%s object=%+v err=%v", decision, objects.request, err)
	}
}

func TestCatalogBusinessActionAuthorizerPreservesBroadQualityUpdateGrantForStatusTool(t *testing.T) {
	actions := &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionQualityBugsUpdate)}}
	authorizer, err := NewCatalogBusinessActionAuthorizer(actions, nil)
	if err != nil {
		t.Fatal(err)
	}
	authorization := ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}
	decision, err := authorizer.AuthorizeBusinessAction(context.Background(), BusinessActionRequest{
		ToolName: "quality.bug.status.update", Authorization: authorization,
		Input: json.RawMessage(`{"id":11,"status":"review"}`),
	})
	if err != nil || decision != BusinessActionAllowed {
		t.Fatalf("decision=%s err=%v", decision, err)
	}

	actions.codes = []string{string(accesscatalog.ActionQualityBugsTransitionStatus)}
	decision, err = authorizer.AuthorizeBusinessAction(context.Background(), BusinessActionRequest{
		ToolName: "quality.bug.update", Authorization: authorization,
		Input: json.RawMessage(`{"id":11,"severity_key":"p0"}`),
	})
	if err != nil || decision != BusinessActionDenied {
		t.Fatalf("transition-only broad update decision=%s err=%v", decision, err)
	}
}
