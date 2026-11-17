package pmtools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type accessToolResolver struct {
	documentlibrary.Resolver
	owner uint
}

func (s *accessToolResolver) RequireTypeOperation(_ context.Context, w uint, id documentlibrary.ResourceID, _ documentlibrary.Operation) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	if w != 1 || id != "r" {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, resourceaccess.ErrNotFound
	}
	return documentlibrary.Resource{ID: id, WorkspaceID: w, OwnerUserID: s.owner, TypeKey: documentlibrary.TypeRichText}, documentlibrary.ContentBinding{}, nil
}

type accessToolSubjects struct{ identity.SubjectDirectory }

func (accessToolSubjects) MatchSubjects(_ context.Context, r identity.SubjectMatchRequest) (identity.SubjectMatchResult, error) {
	result := identity.SubjectMatchResult{}
	for _, subject := range r.Subjects {
		result.Matches = append(result.Matches, identity.SubjectMatch{Subject: subject, Matched: true})
	}
	return result, nil
}

type accessToolPolicyStore struct{ resourceaccess.PolicyStore }

func (accessToolPolicyStore) ListPolicies(_ context.Context, w uint, ids []documentlibrary.ResourceID) (map[documentlibrary.ResourceID]resourceaccess.Policy, error) {
	return map[documentlibrary.ResourceID]resourceaccess.Policy{"r": {WorkspaceID: w, ResourceID: "r", Mode: resourceaccess.ModePrivate, Revision: 1}}, nil
}
func (accessToolPolicyStore) ResolveLinkAccess(context.Context, resourceaccess.Principal, []documentlibrary.ResourceID, string, time.Time) (map[documentlibrary.ResourceID]resourceaccess.LinkAccessDecision, error) {
	return nil, nil
}

type accessToolActions struct {
	allowed   bool
	operation documentlibrary.Operation
}

func (s *accessToolActions) AuthorizeDocumentResourceOperation(_ context.Context, _ resourceaccess.Principal, _ documentlibrary.Resource, op documentlibrary.Operation) (bool, error) {
	s.operation = op
	return s.allowed, nil
}

func TestAccessMutationObjectGateMatchesOwnerActionAndWorkspace(t *testing.T) {
	resolver, actions := &accessToolResolver{owner: 7}, &accessToolActions{allowed: true}
	policies, err := resourceaccess.NewPolicyService(resolver, accessToolSubjects{}, actions, nil, accessToolPolicyStore{}, resourceaccess.SystemClock{})
	if err != nil {
		t.Fatal(err)
	}
	s := &DocumentAccessTools{Users: documentToolUsers{}, Mutations: &resourceaccess.ToolMutationService{Policies: policies, Requests: resourceaccess.NewRequestService(policies, nil)}}
	for _, tc := range []struct {
		name string
		op   documentlibrary.Operation
	}{
		{"document.resource.policy.update", documentlibrary.OperationManageAccess},
		{"document.access_request.decide", documentlibrary.OperationManageAccess},
		{"document.share_link.revoke", documentlibrary.OperationManageLinks},
	} {
		r := toolruntime.ObjectActionRequest{ToolName: tc.name, PolicyCode: "document.resource-access", Input: json.RawMessage(`{"resource_id":"r"}`), Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 1, PrincipalID: 7}}
		check := func(want bool) {
			t.Helper()
			allowed, err := s.AuthorizeObjectAction(context.Background(), r)
			if err != nil || allowed != want {
				t.Fatalf("%s allowed=%v want=%v err=%v", tc.name, allowed, want, err)
			}
		}
		check(true)
		if actions.operation != tc.op {
			t.Fatalf("%s used wrong operation %s", tc.name, actions.operation)
		}
		actions.allowed = false
		check(false)
		actions.allowed = true
		resolver.owner = 8
		check(false)
		resolver.owner = 7
		r.Authorization.WorkspaceID = 2
		check(false)
		r.Authorization.WorkspaceID = 1
		r.Authorization.PrincipalID = 8
		check(false)
	}
}
