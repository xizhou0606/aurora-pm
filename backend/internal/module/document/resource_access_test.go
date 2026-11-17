package document

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type stubResourceAccess struct {
	values map[uint]ResourceAccess
	err    error
}

type stubCollaborationProjection struct {
	value CollaborationProjection
	err   error
}

func (stub stubCollaborationProjection) LoadDocumentProjection(
	context.Context, Principal, string,
) (CollaborationProjection, error) {
	return stub.value, stub.err
}

func (stub stubResourceAccess) ResolveDocumentAccess(
	_ context.Context, _ Principal, documentID uint,
) (ResourceAccess, error) {
	if stub.err != nil {
		return ResourceAccess{}, stub.err
	}
	return stub.values[documentID], nil
}

func (stub stubResourceAccess) ResolveDocumentsAccess(
	_ context.Context, _ Principal, documentIDs []uint,
) (map[uint]ResourceAccess, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	result := make(map[uint]ResourceAccess, len(documentIDs))
	for _, documentID := range documentIDs {
		result[documentID] = stub.values[documentID]
	}
	return result, nil
}

func TestUnifiedResourcePolicyCannotBeBypassedByLegacyScopeAll(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionScopeAll), string(ActionNodesView), string(ActionContentView), string(ActionContentUpdate),
	})
	service.ConfigureResourceAccess(stubResourceAccess{values: map[uint]ResourceAccess{
		11: {ResourceID: "dr_visible", CanView: true},
		12: {ResourceID: "dr_private"},
	}})
	service.store.(*stubStore).documents[12] = Document{ID: 12, WorkspaceID: 7, NodeID: 3, Revision: 1}

	tree, err := service.Tree(context.Background(), Principal{UserID: 5, WorkspaceID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 2 || tree.Nodes[1].ID != 2 || tree.Nodes[1].ResourceID != "dr_visible" {
		t.Fatalf("filtered tree=%+v", tree.Nodes)
	}
	if _, err := service.GetDocument(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 12); !errors.Is(err, ErrForbidden) {
		t.Fatalf("scope.all exposed private resource: %v", err)
	}
}

func TestUnifiedResourceProjectionAndFailClosedDependency(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionContentView)})
	service.ConfigureResourceAccess(stubResourceAccess{values: map[uint]ResourceAccess{
		11: {ResourceID: "dr_11", CanView: true, CanEdit: true},
	}})
	body, err := service.GetDocument(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11)
	if err != nil || body.ResourceID != "dr_11" || !includesAction(body.AllowedActions, string(ActionContentView)) {
		t.Fatalf("body=%+v err=%v", body, err)
	}

	service.ConfigureResourceAccess(stubResourceAccess{err: errors.New("policy unavailable")})
	if _, err := service.GetDocument(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11); err == nil {
		t.Fatal("expected dependency failure to close access")
	}
}

func TestLegacyResourceAdapterDoesNotReenterUnifiedResolver(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionContentView)})
	service.ConfigureResourceAccess(stubResourceAccess{err: errors.New("must not be called")})
	access, err := service.EvaluateLegacyResourceAccess(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11,
	)
	if err != nil || !access.CanView || !access.CanEdit || !access.CanManage {
		t.Fatalf("legacy access=%+v err=%v", access, err)
	}
}

func TestCutoverDocumentReadsOnlyDurableCollaborationProjection(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionContentView), string(ActionContentUpdate)})
	cutoverAt := time.Now().UTC()
	store := service.store.(*stubStore)
	legacy := store.documents[11]
	legacy.CollaborationCutoverAt = &cutoverAt
	legacy.Content = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"stale"}]}]}`)
	legacy.Revision = 9
	store.documents[11] = legacy
	service.ConfigureResourceAccess(stubResourceAccess{values: map[uint]ResourceAccess{
		11: {ResourceID: "dr_11", CanView: true, CanEdit: true},
	}})
	service.ConfigureCollaborationProjection(stubCollaborationProjection{value: CollaborationProjection{
		Content:     json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"durable"}]}]}`),
		ContentHash: "projection-hash", PlainText: "durable", SchemaVersion: SchemaVersionV3, Revision: 17,
	}})

	body, err := service.GetDocument(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11)
	if err != nil {
		t.Fatal(err)
	}
	if string(body.Content) == string(legacy.Content) || body.PlainText != "durable" || body.ContentHash != "projection-hash" ||
		body.SchemaVersion != SchemaVersionV3 || body.Revision != 17 {
		t.Fatalf("document=%+v", body)
	}
}

func TestCutoverDocumentRejectsLegacyBodyWritesBeforeStoreMutation(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionContentUpdate)})
	cutoverAt := time.Now().UTC()
	store := service.store.(*stubStore)
	legacy := store.documents[11]
	legacy.CollaborationCutoverAt = &cutoverAt
	store.documents[11] = legacy

	_, err := service.UpdateDocument(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, legacy.Revision,
		SchemaVersionV3, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`),
	)
	if !errors.Is(err, ErrLegacyContentWriteDisabled) {
		t.Fatalf("error=%v", err)
	}
	if got := store.documents[11]; got.Revision != legacy.Revision || string(got.Content) != string(legacy.Content) {
		t.Fatalf("legacy row mutated=%+v", got)
	}
}

func TestUnifiedTreeKeepsEmptyFoldersWithoutExposingPrivateDocuments(t *testing.T) {
	for _, scopeAll := range []bool{false, true} {
		t.Run(map[bool]string{false: "creator", true: "all_scope"}[scopeAll], func(t *testing.T) {
			actions := []string{string(ActionNodesView)}
			if scopeAll {
				actions = append(actions, string(ActionScopeAll))
			}
			service, _ := newPolicyTestService(t, actions)
			store := service.store.(*stubStore)
			store.nodes = []Node{
				{ID: 1, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, CreatedBy: 9, Title: "parent"},
				{ID: 2, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, ParentID: uintTestPointer(1), CreatedBy: 5, Title: "new empty folder"},
				{ID: 3, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, CreatedBy: 9, Title: "other empty folder"},
				{ID: 4, WorkspaceID: 7, SpaceID: 1, Type: NodeDocument, ParentID: uintTestPointer(2), DocumentID: uintTestPointer(11)},
				{ID: 5, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, CreatedBy: 5, Title: "empty root"},
			}
			service.ConfigureResourceAccess(stubResourceAccess{values: map[uint]ResourceAccess{11: {CanView: false}}})
			tree, err := service.Tree(context.Background(), Principal{UserID: 5, ParticipantUserID: 50, WorkspaceID: 7})
			if err != nil {
				t.Fatal(err)
			}
			ids := map[uint]bool{}
			for _, node := range tree.Nodes {
				ids[node.ID] = true
			}
			if !ids[1] || !ids[2] || !ids[5] || ids[3] != scopeAll || ids[4] {
				t.Fatalf("unexpected folder visibility: %v", ids)
			}
		})
	}
}
