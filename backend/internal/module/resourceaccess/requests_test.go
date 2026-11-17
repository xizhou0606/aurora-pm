package resourceaccess

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"testing"
	"time"
)

type emptyRequestStore struct{ RequestStore }

func (emptyRequestStore) ListAccessRequests(context.Context, Principal, documentlibrary.ResourceID, bool) ([]AccessRequest, error) {
	return []AccessRequest{}, nil
}
func (emptyRequestStore) CountPendingAccessRequests(context.Context, Principal, documentlibrary.ResourceID, bool, time.Time) (int, error) {
	return 0, nil
}

func TestRequestContextOwnerDisplayDoesNotRequireACLPickerMembership(t *testing.T) {
	id := documentlibrary.ResourceID("admin_owned")
	resource := documentlibrary.Resource{ID: id, WorkspaceID: 2, OwnerUserID: 110, TypeKey: documentlibrary.TypeRichText}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "110"}: true}, resolveErr: errors.New("administrator is not in member picker")}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationView: true, documentlibrary.OperationViewAccess: true, documentlibrary.OperationEdit: true, documentlibrary.OperationManageAccess: true}}
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{id: {ResourceID: id, Mode: ModePrivate, Revision: 1}}}
	policies, err := NewPolicyService(&resolverStub{resource: resource}, directory, authorizer, nil, store, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	service := NewRequestService(policies, emptyRequestStore{})
	service.ConfigureParticipantNames(func(_ context.Context, ids []uint) (map[uint]string, error) {
		if len(ids) != 1 || ids[0] != 110 {
			t.Fatalf("unscoped display lookup: %v", ids)
		}
		return map[uint]string{110: "sysadmin"}, nil
	})
	view, err := service.Context(context.Background(), Principal{WorkspaceID: 2, UserID: 110}, id)
	if err != nil || !view.CanReview || view.OwnerName != "sysadmin" || view.PendingCount != 0 {
		t.Fatalf("context=%+v err=%v", view, err)
	}
}

type requestGrantsStub map[documentlibrary.ResourceID]bool

func TestDeniedDocumentRequestMetadataIsWorkspaceScoped(t *testing.T) {
	id := documentlibrary.ResourceID("private_document")
	resolver := &resolverStub{resource: documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 9, TypeKey: documentlibrary.TypeRichText}}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationViewAccess: true, documentlibrary.OperationView: true, documentlibrary.OperationEdit: true}}
	policies, err := NewPolicyService(resolver, directory, authorizer, nil, &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{id: {ResourceID: id, Mode: ModePrivate, Revision: 1}}}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	service := NewRequestService(policies, emptyRequestStore{})
	service.ConfigureParticipantNames(func(context.Context, []uint) (map[uint]string, error) { return map[uint]string{9: "Owner"}, nil })
	p := Principal{WorkspaceID: 1, UserID: 7}
	view, err := service.DocumentContext(context.Background(), p, 14)
	if err != nil || view.HasAccess || !view.CanRequest || view.CanReview || view.OwnerName != "Owner" {
		t.Fatalf("private request context=%+v err=%v", view, err)
	}
	authorizer.allowed[documentlibrary.OperationEdit] = false
	view, err = service.DocumentContext(context.Background(), p, 14)
	if err != nil || view.CanRequest {
		t.Fatalf("global edit bypass=%+v err=%v", view, err)
	}
	p.WorkspaceID = 2
	if _, err = service.DocumentContext(context.Background(), p, 14); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross workspace=%v", err)
	}
	p.WorkspaceID = 1
	resolver.resource.Lifecycle = "archived"
	if _, err = service.DocumentContext(context.Background(), p, 14); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archived=%v", err)
	}
	resolver.resource.Lifecycle = ""
	directory.matches = map[identity.SubjectRef]bool{}
	if _, err = service.DocumentContext(context.Background(), p, 14); !errors.Is(err, ErrNotFound) {
		t.Fatalf("nonmember=%v", err)
	}
}

func (s requestGrantsStub) ApprovedRequestResources(context.Context, Principal, []documentlibrary.ResourceID) (map[documentlibrary.ResourceID][]RequestGrant, error) {
	result := map[documentlibrary.ResourceID][]RequestGrant{}
	for id, allowed := range s {
		if allowed {
			result[id] = []RequestGrant{{Level: LevelEditor}}
		}
	}
	return result, nil
}
func TestApprovedRequestsUnionWithPolicyAndPreserveGlobalGate(t *testing.T) {
	id := documentlibrary.ResourceID("dr_approved")
	viewer := LevelViewer
	resource := documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 99, TypeKey: documentlibrary.TypeRichText}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationViewAccess: true, documentlibrary.OperationView: true, documentlibrary.OperationEdit: true, documentlibrary.OperationManageAccess: true}}
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{id: {ResourceID: id, Mode: ModeWorkspace, WorkspaceLevel: &viewer, Revision: 1}}, linkLevels: map[documentlibrary.ResourceID]Level{id: LevelViewer}}
	service, err := NewPolicyService(&resolverStub{resource: resource}, directory, authorizer, nil, store, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	grants := requestGrantsStub{id: true}
	service.ConfigureApprovedRequests(grants)
	p := Principal{WorkspaceID: 1, UserID: 7}
	evaluate := func(op documentlibrary.Operation) Evaluation {
		t.Helper()
		value, err := service.Evaluate(context.Background(), p, EvaluationRequest{ResourceID: id, Operation: op})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	value := evaluate(documentlibrary.OperationEdit)
	if !value.Allowed || value.EffectiveLevel == nil || *value.EffectiveLevel != LevelEditor || value.ExpiresAt != nil {
		t.Fatalf("approved grant=%+v", value)
	}
	if evaluate(documentlibrary.OperationManageAccess).Allowed {
		t.Fatal("grant became owner")
	}
	authorizer.allowed[documentlibrary.OperationEdit] = false
	if evaluate(documentlibrary.OperationEdit).Allowed {
		t.Fatal("global action bypass")
	}
	authorizer.allowed[documentlibrary.OperationEdit] = true
	delete(grants, id)
	if evaluate(documentlibrary.OperationEdit).Allowed || !evaluate(documentlibrary.OperationView).Allowed {
		t.Fatal("revocation removed unrelated viewer source")
	}
	editor := LevelEditor
	policy := store.policies[id]
	policy.WorkspaceLevel = &editor
	store.policies[id] = policy
	if !evaluate(documentlibrary.OperationEdit).Allowed {
		t.Fatal("revocation removed unrelated editor source")
	}
}
