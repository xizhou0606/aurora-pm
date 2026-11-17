package resourceaccess

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"testing"
	"time"
)

type typedRequestGrants map[documentlibrary.ResourceID][]RequestGrant

func (g typedRequestGrants) ApprovedRequestResources(context.Context, Principal, []documentlibrary.ResourceID) (map[documentlibrary.ResourceID][]RequestGrant, error) {
	return g, nil
}

func TestTimedEditorUnionWithPermanentViewerAndReapplication(t *testing.T) {
	for _, kind := range []documentlibrary.TypeKey{documentlibrary.TypeRichText, documentlibrary.TypeBitable} {
		t.Run(string(kind), func(t *testing.T) {
			now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
			expiry := now.Add(24 * time.Hour)
			id := documentlibrary.ResourceID("resource")
			source := typedRequestGrants{id: {{Level: LevelViewer}, {Level: LevelEditor, ExpiresAt: &expiry}}}
			resolver := &resolverStub{resource: documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 9, TypeKey: kind}}
			policies, err := NewPolicyService(resolver, &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true}}, &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationView: true, documentlibrary.OperationEdit: true, documentlibrary.OperationViewAccess: true}}, nil, &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{id: {ResourceID: id, Mode: ModePrivate, Revision: 1}}}, fixedClock{now: now})
			if err != nil {
				t.Fatal(err)
			}
			policies.ConfigureApprovedRequests(source)
			p := Principal{WorkspaceID: 1, UserID: 7}
			view, err := policies.Evaluate(context.Background(), p, EvaluationRequest{ResourceID: id, Operation: documentlibrary.OperationView})
			if err != nil || !view.Allowed || view.ExpiresAt != nil {
				t.Fatalf("view=%+v err=%v", view, err)
			}
			edit, err := policies.Evaluate(context.Background(), p, EvaluationRequest{ResourceID: id, Operation: documentlibrary.OperationEdit})
			if err != nil || !edit.Allowed || edit.ExpiresAt == nil || !edit.ExpiresAt.Equal(expiry) {
				t.Fatalf("edit=%+v err=%v", edit, err)
			}
			policies.clock = fixedClock{now: expiry}
			edit, err = policies.Evaluate(context.Background(), p, EvaluationRequest{ResourceID: id, Operation: documentlibrary.OperationEdit})
			if err != nil || edit.Allowed {
				t.Fatalf("expired editor=%+v err=%v", edit, err)
			}
			requests := NewRequestService(policies, emptyRequestStore{})
			requests.ConfigureParticipantNames(func(context.Context, []uint) (map[uint]string, error) { return map[uint]string{9: "Owner"}, nil })
			contextView, err := requests.Context(context.Background(), p, id)
			if err != nil || !contextView.HasAccess || !contextView.CanRequest || contextView.ResourceType != kind {
				t.Fatalf("reapplication=%+v err=%v", contextView, err)
			}
		})
	}
}

func TestGrantLifetimeDoesNotShortenExistingPermanentEditor(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(time.Hour)
	editor := LevelEditor
	result := Evaluation{Operation: documentlibrary.OperationEdit, Allowed: true, EffectiveLevel: &editor, Sources: []string{"workspace"}}
	applyRequestGrants(&result, []RequestGrant{{Level: LevelEditor, ExpiresAt: &expiry}}, now)
	if result.ExpiresAt != nil || !result.Allowed {
		t.Fatalf("existing grant shortened: %+v", result)
	}
}
