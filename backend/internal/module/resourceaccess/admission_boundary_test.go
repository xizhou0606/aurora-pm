package resourceaccess

import (
	"context"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func TestWorkspaceAdmissionDoesNotBypassPrivateResourceOwnership(t *testing.T) {
	for _, kind := range []documentlibrary.TypeKey{documentlibrary.TypeRichText, documentlibrary.TypeBitable} {
		t.Run(string(kind), func(t *testing.T) {
			id := documentlibrary.ResourceID("private-resource")
			resolver := &resolverStub{resource: documentlibrary.Resource{ID: id, WorkspaceID: 2, OwnerUserID: 110, TypeKey: kind}}
			directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "110"}: true}}
			store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{id: {WorkspaceID: 2, ResourceID: id, Mode: ModePrivate, Revision: 1}}}
			authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationView: true}}
			service, err := NewPolicyService(resolver, directory, authorizer, nil, store, fixedClock{})
			if err != nil {
				t.Fatal(err)
			}
			principal := Principal{WorkspaceID: 2, UserID: 110}
			request := EvaluationRequest{ResourceID: id, Operation: documentlibrary.OperationView}
			for _, owner := range []uint{110, 111} {
				resolver.resource.OwnerUserID = owner
				want := owner == principal.UserID
				point, err := service.Evaluate(context.Background(), principal, request)
				if err != nil || point.Allowed != want {
					t.Fatalf("point=%+v err=%v owner=%d", point, err, owner)
				}
				list, err := service.BatchEvaluate(context.Background(), principal, []EvaluationRequest{request})
				if err != nil || len(list) != 1 || list[0].Allowed != want {
					t.Fatalf("list=%+v err=%v owner=%d", list, err, owner)
				}
			}
		})
	}
}
