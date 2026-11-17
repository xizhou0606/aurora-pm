package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type stubActions struct {
	codes       []string
	workspaceID uint
}

func (stub *stubActions) EffectiveDocumentActionCodes(_ context.Context, _ uint, workspaceID uint) ([]string, error) {
	stub.workspaceID = workspaceID
	return append([]string(nil), stub.codes...), nil
}

type stubRequirements struct {
	values map[uint]RequirementParticipants
}

func (stub stubRequirements) ResolveRequirementParticipants(
	_ context.Context,
	workspaceID uint,
	requirementID uint,
) (RequirementParticipants, error) {
	value, exists := stub.values[requirementID]
	if !exists || value.WorkspaceID != workspaceID {
		return RequirementParticipants{}, ErrRequirementUnavailable
	}
	return value, nil
}

type stubStore struct {
	space       Space
	nodes       []Node
	documents   map[uint]Document
	bindings    map[uint][]Binding
	assets      map[uint]Asset
	nextAsset   uint
	preferences map[uint]NodePreference
}

func (stub *stubStore) EnsureDefaultSpace(context.Context, uint) (Space, error) {
	return stub.space, nil
}
func (stub *stubStore) ListNodes(context.Context, uint, uint) ([]Node, error) {
	return append([]Node(nil), stub.nodes...), nil
}
func (stub *stubStore) GetNode(_ context.Context, workspaceID, id uint) (Node, error) {
	for _, node := range stub.nodes {
		if node.WorkspaceID == workspaceID && node.ID == id {
			return node, nil
		}
	}
	return Node{}, ErrNodeNotFound
}
func (*stubStore) NextSortOrder(context.Context, uint, uint, *uint) (int64, error) { return 1024, nil }
func (*stubStore) CreateNode(context.Context, Node, *Document) (Node, *Document, error) {
	return Node{}, nil, errors.New("not implemented")
}
func (stub *stubStore) UpdateNode(_ context.Context, value Node) (Node, error) {
	for index, node := range stub.nodes {
		if node.WorkspaceID == value.WorkspaceID && node.ID == value.ID && node.ArchivedAt == nil {
			stub.nodes[index] = value
			return value, nil
		}
	}
	return Node{}, ErrNodeNotFound
}
func (stub *stubStore) ArchiveNodes(_ context.Context, workspaceID uint, ids []uint, actorID uint) error {
	archivedAt := time.Now().UTC()
	for index, node := range stub.nodes {
		if node.WorkspaceID != workspaceID || !includesUser(ids, node.ID) {
			continue
		}
		stub.nodes[index].ArchivedAt = &archivedAt
		stub.nodes[index].UpdatedBy = actorID
	}
	return nil
}
func (stub *stubStore) GetDocument(_ context.Context, workspaceID, id uint) (Document, error) {
	value, exists := stub.documents[id]
	if !exists || value.WorkspaceID != workspaceID {
		return Document{}, ErrDocumentNotFound
	}
	return value, nil
}
func (stub *stubStore) UpdateDocument(_ context.Context, body Document, expectedRevision, actorID uint) (Document, error) {
	current, exists := stub.documents[body.ID]
	if !exists || current.WorkspaceID != body.WorkspaceID {
		return Document{}, ErrDocumentNotFound
	}
	if current.Revision != expectedRevision {
		return Document{}, RevisionConflictError{LatestRevision: current.Revision}
	}
	body.Revision = current.Revision + 1
	body.UpdatedBy = actorID
	stub.documents[body.ID] = body
	return body, nil
}
func (stub *stubStore) ListBindings(_ context.Context, workspaceID, documentID uint) ([]Binding, error) {
	result := make([]Binding, 0)
	for _, binding := range stub.bindings[documentID] {
		if binding.WorkspaceID == workspaceID {
			result = append(result, binding)
		}
	}
	return result, nil
}
func (stub *stubStore) ListBindingsByDocumentIDs(context.Context, uint, []uint) ([]Binding, error) {
	return nil, errors.New("not implemented")
}
func (*stubStore) CreateBinding(context.Context, Binding) (Binding, error) {
	return Binding{}, errors.New("not implemented")
}
func (*stubStore) DeleteBinding(context.Context, uint, uint, uint) error {
	return errors.New("not implemented")
}
func (stub *stubStore) ListAssets(_ context.Context, workspaceID, documentID uint) ([]Asset, error) {
	result := make([]Asset, 0)
	for _, asset := range stub.assets {
		if asset.WorkspaceID == workspaceID && asset.DocumentID == documentID && asset.ArchivedAt == nil {
			result = append(result, asset)
		}
	}
	return result, nil
}
func (stub *stubStore) GetAsset(_ context.Context, workspaceID, documentID, assetID uint) (Asset, error) {
	asset, exists := stub.assets[assetID]
	if !exists || asset.WorkspaceID != workspaceID || asset.DocumentID != documentID || asset.ArchivedAt != nil {
		return Asset{}, ErrAssetNotFound
	}
	return asset, nil
}
func (stub *stubStore) CreateAsset(_ context.Context, asset Asset) (Asset, error) {
	if stub.assets == nil {
		stub.assets = map[uint]Asset{}
	}
	stub.nextAsset++
	asset.ID = stub.nextAsset
	stub.assets[asset.ID] = asset
	return asset, nil
}
func (stub *stubStore) ArchiveAsset(_ context.Context, workspaceID, documentID, assetID, _ uint) error {
	asset, err := stub.GetAsset(context.Background(), workspaceID, documentID, assetID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	asset.ArchivedAt = &now
	stub.assets[assetID] = asset
	return nil
}
func (stub *stubStore) CopyDocumentNode(_ context.Context, command CopyDocumentNodeCommand) (CreateNodeResult, error) {
	nodeID := uint(len(stub.nodes) + 10)
	documentID := uint(len(stub.documents) + 20)
	node := Node{
		ID: nodeID, WorkspaceID: command.WorkspaceID, SpaceID: command.SourceNode.SpaceID,
		ParentID: command.ParentID, Type: NodeDocument, Title: command.SourceNode.Title + "（副本）",
		DocumentID: uintTestPointer(documentID), CreatedBy: command.ActorUserID, UpdatedBy: command.ActorUserID,
	}
	body := command.Source
	body.ID, body.NodeID, body.Revision = documentID, nodeID, 1
	body.OwnerUserID, body.CreatedBy, body.UpdatedBy = command.ActorUserID, command.ActorUserID, command.ActorUserID
	stub.nodes = append(stub.nodes, node)
	stub.documents[documentID] = body
	return CreateNodeResult{Node: node, Document: &body}, nil
}
func (stub *stubStore) CreateShortcut(_ context.Context, command CreateShortcutCommand) (Node, error) {
	node := Node{
		ID: uint(len(stub.nodes) + 10), WorkspaceID: command.WorkspaceID, SpaceID: command.TargetNode.SpaceID,
		ParentID: command.ParentID, Type: NodeShortcut, Title: command.TargetNode.Title,
		ShortcutTargetNodeID: uintTestPointer(command.TargetNode.ID), DocumentID: command.TargetNode.DocumentID,
		CreatedBy: command.ActorUserID, UpdatedBy: command.ActorUserID,
	}
	stub.nodes = append(stub.nodes, node)
	return node, nil
}
func (stub *stubStore) ListNodePreferences(_ context.Context, _, _ uint, nodeIDs []uint) ([]NodePreference, error) {
	result := make([]NodePreference, 0)
	for _, nodeID := range nodeIDs {
		if preference, exists := stub.preferences[nodeID]; exists {
			result = append(result, preference)
		}
	}
	return result, nil
}
func (stub *stubStore) UpdateNodePreference(_ context.Context, command UpdateNodePreferenceCommand) (NodePreference, error) {
	if stub.preferences == nil {
		stub.preferences = map[uint]NodePreference{}
	}
	value := stub.preferences[command.TargetNode.ID]
	value.NodeID, value.UpdatedAt = command.TargetNode.ID, time.Now().UTC()
	if command.Pinned != nil {
		if *command.Pinned {
			value.PinnedAt = &value.UpdatedAt
		} else {
			value.PinnedAt = nil
		}
	}
	if command.Favorited != nil {
		value.Favorited = *command.Favorited
	}
	stub.preferences[value.NodeID] = value
	return value, nil
}
func (stub *stubStore) TransferDocumentOwner(_ context.Context, command TransferDocumentOwnerCommand) (Document, error) {
	body, exists := stub.documents[command.DocumentID]
	if !exists || body.WorkspaceID != command.WorkspaceID {
		return Document{}, ErrDocumentNotFound
	}
	body.OwnerUserID, body.UpdatedBy = command.OwnerUserID, command.ActorUserID
	stub.documents[body.ID] = body
	return body, nil
}

type stubAssetStorage struct{ objects map[string][]byte }

func (stub *stubAssetStorage) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	value, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if stub.objects == nil {
		stub.objects = map[string][]byte{}
	}
	stub.objects[key] = value
	return nil
}
func (stub *stubAssetStorage) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	value, exists := stub.objects[key]
	if !exists {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(value)), nil
}
func (stub *stubAssetStorage) RemoveObject(_ context.Context, key string) error {
	delete(stub.objects, key)
	return nil
}

func newPolicyTestService(t *testing.T, actions []string) (*Service, *stubActions) {
	t.Helper()
	resolver := &stubActions{codes: actions}
	store := &stubStore{
		space: Space{ID: 1, WorkspaceID: 7, IsDefault: true},
		documents: map[uint]Document{11: {
			ID: 11, WorkspaceID: 7, NodeID: 2, Content: DefaultContent(), SchemaVersion: 1, Revision: 1,
		}},
		bindings: map[uint][]Binding{11: {{ID: 1, WorkspaceID: 7, DocumentID: 11, RequirementID: 21, Kind: BindingPrimaryProductDoc}}},
		nodes: []Node{
			{ID: 1, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, Title: "Visible parent"},
			{ID: 2, WorkspaceID: 7, SpaceID: 1, ParentID: uintTestPointer(1), Type: NodeDocument, Title: "Bound", DocumentID: uintTestPointer(11)},
			{ID: 3, WorkspaceID: 7, SpaceID: 1, Type: NodeDocument, Title: "Secret", DocumentID: uintTestPointer(12)},
		},
	}
	service, err := NewService(store, resolver, stubRequirements{values: map[uint]RequirementParticipants{
		21: {RequirementID: 21, WorkspaceID: 7, Version: 3, Active: true, DeveloperUserIDs: []uint{5}, TesterUserIDs: []uint{6}},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return service, resolver
}

func TestBindingParticipantReadsWithoutRequirementViewAction(t *testing.T) {
	service, actions := newPolicyTestService(t, []string{string(ActionContentView), string(ActionNodesView)})
	service.requirements = stubRequirements{values: map[uint]RequirementParticipants{
		21: {RequirementID: 21, WorkspaceID: 7, Version: 3, Active: true, DeveloperUserIDs: []uint{50}},
	}}
	body, err := service.GetDocument(
		context.Background(),
		Principal{UserID: 5, ParticipantUserID: 50, WorkspaceID: 7},
		11,
	)
	if err != nil {
		t.Fatal(err)
	}
	if actions.workspaceID != 7 {
		t.Fatalf("action resolver workspace=%d", actions.workspaceID)
	}
	if !includesAction(body.AllowedActions, string(ActionContentView)) {
		t.Fatalf("allowed_actions=%v", body.AllowedActions)
	}
}

func TestTesterIsReadOnlyInV1(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionContentView), string(ActionContentUpdate)})
	_, err := service.UpdateDocument(
		context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11, 1, SchemaVersionV1,
		json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`),
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestDocumentContentCanUpgradeToSchemaV2ButCannotDowngrade(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionContentUpdate)})
	content := json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":4},"content":[{"type":"text","text":"Imported"}]},{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph"}]}]}]}]}`)
	updated, err := service.UpdateDocument(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, 1, SchemaVersionV2, content,
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SchemaVersion != SchemaVersionV2 || updated.Revision != 2 {
		t.Fatalf("updated=%+v", updated)
	}
	_, err = service.UpdateDocument(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, 2, SchemaVersionV1, DefaultContent(),
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("downgrade=%v", err)
	}
}

func TestDocumentAssetUploadDownloadAndDelete(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionScopeAll), string(ActionAssetsUpload), string(ActionAssetsDownload), string(ActionAssetsDelete),
	})
	storage := &stubAssetStorage{}
	service.ConfigureAssetStorage(storage, "pm/native")
	payload := []byte("png-bytes")
	asset, err := service.UploadAsset(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, AssetUpload{
		FileName: "diagram.png", ContentType: "image/png", SizeBytes: int64(len(payload)), Body: bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
	if asset.ID == 0 || !strings.HasPrefix(asset.ObjectKey, "pm/native/workspace/7/documents/11/assets/") {
		t.Fatalf("asset=%+v", asset)
	}
	opened, body, err := service.OpenAsset(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	readBack, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil || opened.ID != asset.ID || !bytes.Equal(readBack, payload) {
		t.Fatalf("opened=%+v body=%q err=%v", opened, readBack, err)
	}
	if err := service.DeleteAsset(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, asset.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.OpenAsset(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, asset.ID); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("open deleted asset=%v", err)
	}
}

func TestDocumentAssetAcceptsProtectedNonImageAttachments(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionScopeAll), string(ActionAssetsUpload), string(ActionAssetsDownload),
	})
	storage := &stubAssetStorage{}
	service.ConfigureAssetStorage(storage, "pm/native")
	payload := []byte("ADR attachment")
	asset, err := service.UploadAsset(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, AssetUpload{
		FileName: "decision.txt", ContentType: "text/plain; charset=utf-8", SizeBytes: int64(len(payload)), Body: bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
	if asset.ContentType != "text/plain; charset=utf-8" || !strings.HasSuffix(asset.ObjectKey, ".txt") {
		t.Fatalf("asset=%+v", asset)
	}
}

func TestBindingMutationStillRequiresRequirementViewAction(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionBindingsUpdate)})
	_, err := service.PreviewBinding(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, 21)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestFilteredTreeOnlyReturnsAccessibleDocumentAndAncestors(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionNodesView)})
	tree, err := service.Tree(context.Background(), Principal{UserID: 5, WorkspaceID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 2 || tree.Nodes[0].Title != "Visible parent" || tree.Nodes[1].Title != "Bound" {
		t.Fatalf("filtered nodes=%+v", tree.Nodes)
	}
}

func TestTreeReturnsStableFlatPreorder(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionNodesView)})
	service.store.(*stubStore).nodes = []Node{
		{ID: 3, WorkspaceID: 7, SpaceID: 1, Type: NodeDocument, Title: "second root", SortOrder: 20, DocumentID: uintTestPointer(12)},
		{ID: 2, WorkspaceID: 7, SpaceID: 1, ParentID: uintTestPointer(1), Type: NodeDocument, Title: "child", SortOrder: 10, DocumentID: uintTestPointer(11)},
		{ID: 1, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, Title: "first root", SortOrder: 10},
	}
	tree, err := service.Tree(context.Background(), Principal{UserID: 5, WorkspaceID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 3 || tree.Nodes[0].ID != 1 || tree.Nodes[1].ID != 2 || tree.Nodes[2].ID != 3 {
		t.Fatalf("preorder nodes=%+v", tree.Nodes)
	}
}

func TestMoveNodeSupportsExplicitReorder(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionNodesMove)})
	sortOrder := int64(7)
	moved, err := service.MoveNode(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 3,
		MoveNodeInput{ParentID: uintTestPointer(1), SortOrder: &sortOrder},
	)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ParentID == nil || *moved.ParentID != 1 || moved.SortOrder != sortOrder {
		t.Fatalf("moved=%+v", moved)
	}
}

func TestArchiveRequiresConfirmationForNonEmptyFolder(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionNodesArchive)})
	impact, err := service.ArchiveImpact(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if impact.DescendantCount != 1 || impact.DocumentCount != 1 || !impact.RequiresConfirmation {
		t.Fatalf("impact=%+v", impact)
	}
	if err := service.ArchiveNode(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 1, ArchiveConfirmation{}); !errors.Is(err, ErrArchiveConfirmationRequired) {
		t.Fatalf("unconfirmed archive=%v", err)
	}
	staleDescendants := impact.DescendantCount + 1
	if err := service.ArchiveNode(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 1, ArchiveConfirmation{
		ConfirmNonEmpty: true, ExpectedDescendantCount: &staleDescendants,
		ExpectedDocumentCount: &impact.DocumentCount,
	}); !errors.Is(err, ErrArchiveImpactChanged) {
		t.Fatalf("stale archive=%v", err)
	}
	if err := service.ArchiveNode(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 1, ArchiveConfirmation{
		ConfirmNonEmpty: true, ExpectedDescendantCount: &impact.DescendantCount,
		ExpectedDocumentCount: &impact.DocumentCount,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaV1RejectsUnknownNode(t *testing.T) {
	_, _, _, err := NormalizeContent(json.RawMessage(`{"type":"doc","content":[{"type":"table"}]}`))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestSchemaV2AcceptsStandardMarkdownNodesAndExtendedHeadings(t *testing.T) {
	content := json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":4},"content":[{"type":"text","text":"Heading"}]},{"type":"codeBlock","attrs":{"language":"text"},"content":[{"type":"text","text":"code"}]},{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"Cell"}]}]}]}]},{"type":"taskList","content":[{"type":"taskItem","attrs":{"checked":false},"content":[{"type":"paragraph","content":[{"type":"text","text":"Task"}]}]}]}]}`)
	normalized, _, plainText, err := NormalizeContentForSchema(content, SchemaVersionV2)
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized) == 0 || !strings.Contains(plainText, "Heading") || !strings.Contains(plainText, "Task") {
		t.Fatalf("normalized=%s plainText=%q", normalized, plainText)
	}
}

func TestSchemaV1RejectsV2HeadingLevel(t *testing.T) {
	_, _, _, err := NormalizeContentForSchema(
		json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":4}}]}`),
		SchemaVersionV1,
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestSchemaV1RejectsTrailingJSON(t *testing.T) {
	_, _, _, err := NormalizeContent(json.RawMessage(`{"type":"doc"}{"type":"doc"}`))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func uintTestPointer(value uint) *uint { return &value }

func includesAction(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
