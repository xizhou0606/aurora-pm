package documentgorm

import (
	"context"
	"errors"
	"testing"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
)

func prepareContextRepository(t *testing.T, recorder *commentRecorderStub) (*Repository, document.Node, document.Document) {
	t.Helper()
	db := openDocumentDB(t)
	repository := NewRepository(db)
	repository.ConfigureMutationRecorderFactory(func(*gorm.DB) (document.MutationRecorder, error) {
		return recorder, nil
	})
	space, err := repository.EnsureDefaultSpace(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	content, hash, plain, err := document.NormalizeContent(document.DefaultContent())
	if err != nil {
		t.Fatal(err)
	}
	node, body, err := repository.CreateNode(context.Background(), document.Node{
		WorkspaceID: 7, SpaceID: space.ID, Type: document.NodeDocument,
		Title: "ADR", SortOrder: 1024, CreatedBy: 5, UpdatedBy: 5,
	}, &document.Document{
		WorkspaceID: 7, Content: content, SchemaVersion: 1, Revision: 3,
		ContentHash: hash, PlainText: plain, OwnerUserID: 5, CreatedBy: 5, UpdatedBy: 5,
	})
	if err != nil || body == nil {
		t.Fatalf("node=%+v body=%+v err=%v", node, body, err)
	}
	return repository, node, *body
}

func TestContextRepositoryCopyUsesDeterministicNameAndSafeAssetReferences(t *testing.T) {
	recorder := &commentRecorderStub{}
	repository, sourceNode, sourceBody := prepareContextRepository(t, recorder)
	asset, err := repository.CreateAsset(context.Background(), document.Asset{
		WorkspaceID: 7, DocumentID: sourceBody.ID, ObjectKey: "documents/source/image.png",
		FileName: "image.png", ContentType: "image/png", SizeBytes: 4,
		ContentHash: "hash", CreatedBy: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	copyNode := func() document.CreateNodeResult {
		result, copyErr := repository.CopyDocumentNode(context.Background(), document.CopyDocumentNodeCommand{
			WorkspaceID: 7, ActorUserID: 6, SourceNode: sourceNode, Source: sourceBody,
			Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 6, Payload: map[string]any{}},
		})
		if copyErr != nil {
			t.Fatal(copyErr)
		}
		return result
	}
	first, second := copyNode(), copyNode()
	if first.Node.Title != "ADR（副本）" || second.Node.Title != "ADR（副本 2）" {
		t.Fatalf("copy titles=%q, %q", first.Node.Title, second.Node.Title)
	}
	if first.Document == nil || first.Document.Revision != 1 || first.Document.ID == sourceBody.ID ||
		first.Document.OwnerUserID != 6 || string(first.Document.Content) != string(sourceBody.Content) {
		t.Fatalf("copy body=%+v", first.Document)
	}
	referenced, err := repository.GetAsset(context.Background(), 7, first.Document.ID, asset.ID)
	if err != nil || !referenced.IsReference || referenced.ObjectKey != asset.ObjectKey {
		t.Fatalf("referenced asset=%+v err=%v", referenced, err)
	}
	if err := repository.ArchiveAsset(context.Background(), 7, sourceBody.ID, asset.ID, 5); err != nil {
		t.Fatal(err)
	}
	referenced, err = repository.GetAsset(context.Background(), 7, first.Document.ID, asset.ID)
	if err != nil || !referenced.IsReference {
		t.Fatalf("copy lost archived source asset reference=%+v err=%v", referenced, err)
	}
}

func TestContextRepositoryShortcutTitleSyncAndTargetArchiveInvalidation(t *testing.T) {
	recorder := &commentRecorderStub{}
	repository, sourceNode, sourceBody := prepareContextRepository(t, recorder)
	shortcut, err := repository.CreateShortcut(context.Background(), document.CreateShortcutCommand{
		WorkspaceID: 7, ActorUserID: 5, TargetNode: sourceNode,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 5, Payload: map[string]any{}},
	})
	if err != nil || shortcut.Title != "ADR" || shortcut.DocumentID == nil || *shortcut.DocumentID != sourceBody.ID {
		t.Fatalf("shortcut=%+v err=%v", shortcut, err)
	}
	sourceNode.Title = "Renamed ADR"
	if _, err := repository.UpdateNode(context.Background(), sourceNode); err != nil {
		t.Fatal(err)
	}
	resolved, err := repository.GetNode(context.Background(), 7, shortcut.ID)
	if err != nil || resolved.Title != "Renamed ADR" {
		t.Fatalf("resolved shortcut=%+v err=%v", resolved, err)
	}
	if err := repository.ArchiveNodes(context.Background(), 7, []uint{sourceNode.ID}, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetNode(context.Background(), 7, shortcut.ID); !errors.Is(err, document.ErrNodeNotFound) {
		t.Fatalf("shortcut after target archive err=%v", err)
	}
}

func TestContextRepositoryPreferenceFavoriteFolderAndOwnerAreIdempotentAndScoped(t *testing.T) {
	recorder := &commentRecorderStub{}
	repository, sourceNode, sourceBody := prepareContextRepository(t, recorder)
	pinned, favorited := true, true
	command := document.UpdateNodePreferenceCommand{
		WorkspaceID: 7, ActorUserID: 5, TargetNode: sourceNode,
		Pinned: &pinned, Favorited: &favorited,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 5, Payload: map[string]any{}},
	}
	for range 2 {
		preference, err := repository.UpdateNodePreference(context.Background(), command)
		if err != nil || preference.PinnedAt == nil || !preference.Favorited {
			t.Fatalf("preference=%+v err=%v", preference, err)
		}
	}
	var folderCount, referenceCount int64
	if err := repository.db.Model(&Node{}).Where(
		"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND archived_at IS NULL",
		7, 5, string(document.NodeSystemFavoritesFolder),
	).Count(&folderCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.db.Model(&Node{}).Where(
		"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND archived_at IS NULL",
		7, 5, string(document.NodeSystemFavoriteReference),
	).Count(&referenceCount).Error; err != nil {
		t.Fatal(err)
	}
	if folderCount != 1 || referenceCount != 1 {
		t.Fatalf("favorites folder=%d references=%d", folderCount, referenceCount)
	}
	treeRows, err := repository.ListNodes(context.Background(), 7, sourceNode.SpaceID)
	if err != nil {
		t.Fatal(err)
	}
	foundReference := false
	for _, node := range treeRows {
		if node.SystemKind == document.NodeSystemFavoriteReference {
			foundReference = node.Title == sourceNode.Title && node.DocumentID != nil && *node.DocumentID == sourceBody.ID
		}
	}
	if !foundReference {
		t.Fatalf("favorite reference not resolved: %+v", treeRows)
	}
	sourceNode.Title = "Renamed favorite"
	if _, err := repository.UpdateNode(context.Background(), sourceNode); err != nil {
		t.Fatal(err)
	}
	treeRows, err = repository.ListNodes(context.Background(), 7, sourceNode.SpaceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range treeRows {
		if node.SystemKind == document.NodeSystemFavoriteReference && node.Title != "Renamed favorite" {
			t.Fatalf("favorite title was not synchronized: %+v", node)
		}
	}

	pinned, favorited = false, false
	command.Pinned, command.Favorited = &pinned, &favorited
	for range 2 {
		preference, err := repository.UpdateNodePreference(context.Background(), command)
		if err != nil || preference.PinnedAt != nil || preference.Favorited {
			t.Fatalf("cleared preference=%+v err=%v", preference, err)
		}
	}
	if err := repository.db.Model(&Node{}).Where(
		"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND archived_at IS NULL",
		7, 5, string(document.NodeSystemFavoriteReference),
	).Count(&referenceCount).Error; err != nil || referenceCount != 0 {
		t.Fatalf("active favorite references=%d err=%v", referenceCount, err)
	}

	updated, err := repository.TransferDocumentOwner(context.Background(), document.TransferDocumentOwnerCommand{
		WorkspaceID: 7, ActorUserID: 5, DocumentID: sourceBody.ID, OwnerUserID: 9,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 5, Payload: map[string]any{}},
	})
	if err != nil || updated.OwnerUserID != 9 {
		t.Fatalf("owner=%+v err=%v", updated, err)
	}
	if _, err := repository.TransferDocumentOwner(context.Background(), document.TransferDocumentOwnerCommand{
		WorkspaceID: 8, ActorUserID: 5, DocumentID: sourceBody.ID, OwnerUserID: 9,
		Effect: document.MutationEffect{WorkspaceID: 8, PrincipalID: 5, Payload: map[string]any{}},
	}); !errors.Is(err, document.ErrDocumentNotFound) {
		t.Fatalf("cross-workspace owner err=%v", err)
	}
}

func TestContextRepositoryRollsBackWhenAuditOutboxFails(t *testing.T) {
	recorder := &commentRecorderStub{err: errors.New("outbox unavailable")}
	repository, sourceNode, sourceBody := prepareContextRepository(t, recorder)
	_, err := repository.CopyDocumentNode(context.Background(), document.CopyDocumentNodeCommand{
		WorkspaceID: 7, ActorUserID: 6, SourceNode: sourceNode, Source: sourceBody,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 6, Payload: map[string]any{}},
	})
	if err == nil {
		t.Fatal("expected effect failure")
	}
	var count int64
	if err := repository.db.Model(&Document{}).Where("workspace_id = ?", 7).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("document count=%d err=%v", count, err)
	}
}
