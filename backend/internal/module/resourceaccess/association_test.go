package resourceaccess

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"testing"
)

type associationSourceStub struct{ sets map[string][]uint }

func (s *associationSourceStub) ResourceLinkProjectSets(context.Context, uint, []string) (map[string][]uint, error) {
	return s.sets, nil
}
func TestAssociationConvertsPrivateWithoutCopyingRules(t *testing.T) {
	id := documentlibrary.ResourceID("doc")
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{id: {WorkspaceID: 1, ResourceID: id, Mode: ModePrivate, Revision: 1}}}
	service, _ := NewPolicyService(&resolverStub{resource: documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 7, TypeKey: documentlibrary.TypeRichText}}, &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true}}, &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationManageAccess: true}}, nil, store, fixedClock{})
	if err := service.TouchAssociation(context.Background(), Principal{WorkspaceID: 1, UserID: 7}, id, 1, "attach", true); err != nil {
		t.Fatal(err)
	}
	policy := store.policies[id]
	if policy.Mode != ModeRestricted || policy.Revision != 2 || len(policy.Rules) != 0 {
		t.Fatalf("unexpected conversion: %+v", policy)
	}
	if err := service.TouchAssociation(context.Background(), Principal{WorkspaceID: 1, UserID: 7}, id, 2, "detach", false); err != nil {
		t.Fatal(err)
	}
	if store.policies[id].Mode != ModeRestricted {
		t.Fatal("detach changed independent policy")
	}
}
func TestRequirementSourcesAreDynamicViewerAndNeverPersisted(t *testing.T) {
	id := documentlibrary.ResourceID("doc")
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{id: {ResourceID: id, WorkspaceID: 1, Mode: ModeRestricted, Revision: 1}}}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true, {Kind: identity.SubjectProjectSet, ID: "19"}: true}}
	service, _ := NewPolicyService(&resolverStub{resource: documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 9, TypeKey: documentlibrary.TypeRichText}}, directory, &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationView: true, documentlibrary.OperationEdit: true}}, nil, store, fixedClock{})
	source := &associationSourceStub{sets: map[string][]uint{"doc": {19}}}
	service.ConfigureRequirementSource(source)
	evaluate := func(op documentlibrary.Operation) Evaluation {
		t.Helper()
		result, err := service.Evaluate(context.Background(), Principal{WorkspaceID: 1, UserID: 7}, EvaluationRequest{ResourceID: id, Operation: op})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if !evaluate(documentlibrary.OperationView).Allowed || evaluate(documentlibrary.OperationEdit).Allowed {
		t.Fatal("derived viewer scope incorrect")
	}
	if len(store.policies[id].Rules) != 0 {
		t.Fatal("derived rules persisted")
	}
	directory.matches[identity.SubjectRef{Kind: identity.SubjectProjectSet, ID: "19"}] = false
	if evaluate(documentlibrary.OperationView).Allowed {
		t.Fatal("removed member retained access")
	}
	directory.matches[identity.SubjectRef{Kind: identity.SubjectProjectSet, ID: "19"}] = true
	source.sets = map[string][]uint{}
	if evaluate(documentlibrary.OperationView).Allowed {
		t.Fatal("removed source retained access")
	}
	source.sets = map[string][]uint{"doc": {19}}
	policy := store.policies[id]
	policy.Mode = ModePrivate
	store.policies[id] = policy
	if evaluate(documentlibrary.OperationView).Allowed {
		t.Fatal("source bypassed private policy")
	}
}
