package document

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type stubWorkspaceUsers struct {
	workspaceID uint
	userID      uint
	err         error
}

func (stub *stubWorkspaceUsers) ValidateActiveWorkspaceUser(_ context.Context, workspaceID, userID uint) error {
	stub.workspaceID, stub.userID = workspaceID, userID
	return stub.err
}

func TestCopyNodeCreatesIndependentDocumentAndRequiresManagerCeiling(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionScopeAll), string(ActionNodesCopy), string(ActionNodesCreate), string(ActionContentView),
	})
	result, err := service.CopyNode(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 2, CopyNodeInput{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || result.Node.Type != NodeDocument || result.Node.ID == 2 || result.Document.ID == 11 ||
		result.Document.OwnerUserID != 5 || result.Document.Revision != 1 || string(result.Document.Content) != string(DefaultContent()) {
		t.Fatalf("copy=%+v", result)
	}
	result.Document.Content = []byte(`{"type":"doc","content":[{"type":"paragraph"}]}`)
	if string(service.store.(*stubStore).documents[11].Content) != string(DefaultContent()) {
		t.Fatal("copy mutation changed source content")
	}

	denied, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionNodesCopy), string(ActionContentView)})
	if _, err := denied.CopyNode(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 2, CopyNodeInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("copy without create action err=%v", err)
	}
}

func TestShortcutOnlyTargetsCanonicalDocumentNode(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionShortcutsCreate)})
	shortcut, err := service.CreateShortcut(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 2, CreateShortcutInput{})
	if err != nil || shortcut.Type != NodeShortcut || shortcut.ShortcutTargetNodeID == nil || *shortcut.ShortcutTargetNodeID != 2 || shortcut.DocumentID == nil || *shortcut.DocumentID != 11 {
		t.Fatalf("shortcut=%+v err=%v", shortcut, err)
	}
	if _, err := service.CreateShortcut(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, shortcut.ID, CreateShortcutInput{}); !errors.Is(err, ErrShortcutTargetInvalid) {
		t.Fatalf("shortcut chain err=%v", err)
	}
	if _, err := service.CreateShortcut(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 1, CreateShortcutInput{}); !errors.Is(err, ErrShortcutTargetInvalid) {
		t.Fatalf("folder shortcut err=%v", err)
	}
}

func TestBindingViewerCanUpdateOnlyOwnPreference(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionPreferencesUpdate)})
	pinned, favorited := true, true
	preference, err := service.UpdateNodePreference(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 2,
		UpdateNodePreferenceInput{Pinned: &pinned, Favorited: &favorited},
	)
	if err != nil || preference.PinnedAt == nil || !preference.Favorited {
		t.Fatalf("preference=%+v err=%v", preference, err)
	}
	if _, err := service.UpdateNodePreference(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 3,
		UpdateNodePreferenceInput{Pinned: &pinned},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("hidden target preference err=%v", err)
	}
}

func TestOwnerTransferDoesNotChangeAllowedActions(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionScopeAll), string(ActionOwnershipTransfer), string(ActionContentView), string(ActionContentUpdate),
	})
	users := &stubWorkspaceUsers{}
	service.ConfigureWorkspaceUsers(users)
	before, err := service.GetDocument(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11)
	if err != nil {
		t.Fatal(err)
	}
	after, err := service.TransferDocumentOwner(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, 9)
	if err != nil || after.OwnerUserID != 9 || users.workspaceID != 7 || users.userID != 9 {
		t.Fatalf("after=%+v users=%+v err=%v", after, users, err)
	}
	if !reflect.DeepEqual(before.AllowedActions, after.AllowedActions) {
		t.Fatalf("owner changed allowed actions: before=%v after=%v", before.AllowedActions, after.AllowedActions)
	}
	users.err = errors.New("inactive")
	if _, err := service.TransferDocumentOwner(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, 10); !errors.Is(err, ErrOwnerUserInvalid) {
		t.Fatalf("inactive owner err=%v", err)
	}
}

func TestPreorderPlacesFavoritesThenNewestPinnedThenStableTreeOrder(t *testing.T) {
	oldPinned := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	newPinned := oldPinned.Add(time.Hour)
	ownerID := uint(5)
	nodes := []Node{
		{ID: 1, Type: NodeDocument, SortOrder: 10},
		{ID: 2, Type: NodeDocument, SortOrder: 30, PinnedAt: &oldPinned},
		{ID: 3, Type: NodeDocument, SortOrder: 20, PinnedAt: &newPinned},
		{ID: 4, Type: NodeFolder, SortOrder: 100, SystemManaged: true, SystemKind: NodeSystemFavoritesFolder, SystemOwnerUserID: &ownerID},
	}
	ordered := preorderNodes(nodes)
	ids := []uint{ordered[0].ID, ordered[1].ID, ordered[2].ID, ordered[3].ID}
	if !reflect.DeepEqual(ids, []uint{4, 3, 2, 1}) {
		t.Fatalf("ordered ids=%v", ids)
	}
}

func TestSystemManagedFavoritesNodesRejectOrdinaryMutationsAndStayUserPrivate(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionScopeAll), string(ActionNodesView), string(ActionNodesCreate),
		string(ActionNodesUpdate), string(ActionNodesMove), string(ActionNodesArchive),
	})
	owner5, owner6 := uint(5), uint(6)
	targetID := uint(2)
	service.store.(*stubStore).nodes = append(service.store.(*stubStore).nodes,
		Node{ID: 7, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, Title: FavoritesFolderTitle, SystemManaged: true, SystemKind: NodeSystemFavoritesFolder, SystemOwnerUserID: &owner5},
		Node{ID: 8, WorkspaceID: 7, SpaceID: 1, ParentID: uintTestPointer(7), Type: NodeShortcut, ShortcutTargetNodeID: &targetID, DocumentID: uintTestPointer(11), SystemManaged: true, SystemKind: NodeSystemFavoriteReference, SystemOwnerUserID: &owner5},
		Node{ID: 9, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, Title: FavoritesFolderTitle, SystemManaged: true, SystemKind: NodeSystemFavoritesFolder, SystemOwnerUserID: &owner6},
	)
	principal := Principal{UserID: 5, WorkspaceID: 7}
	if _, err := service.RenameNode(context.Background(), principal, 7, "changed"); !errors.Is(err, ErrSystemManagedNode) {
		t.Fatalf("rename system folder err=%v", err)
	}
	if _, err := service.MoveNode(context.Background(), principal, 8, MoveNodeInput{}); !errors.Is(err, ErrSystemManagedNode) {
		t.Fatalf("move system reference err=%v", err)
	}
	if _, err := service.ArchiveImpact(context.Background(), principal, 7); !errors.Is(err, ErrSystemManagedNode) {
		t.Fatalf("archive system folder err=%v", err)
	}
	if _, err := service.CreateNode(context.Background(), principal, CreateNodeInput{ParentID: uintTestPointer(7), Type: NodeDocument, Title: "forbidden"}); !errors.Is(err, ErrSystemManagedNode) {
		t.Fatalf("create inside favorites err=%v", err)
	}
	tree, err := service.Tree(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range tree.Nodes {
		if node.ID == 9 {
			t.Fatalf("another user's favorites leaked: %+v", tree.Nodes)
		}
	}
}

func TestShortcutAndFavoriteReferenceNeverGrantTargetVisibility(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionNodesView), string(ActionPreferencesUpdate)})
	ownerID, targetID := uint(5), uint(3)
	service.store.(*stubStore).nodes = append(service.store.(*stubStore).nodes,
		Node{ID: 7, WorkspaceID: 7, SpaceID: 1, Type: NodeShortcut, Title: "Secret shortcut", ShortcutTargetNodeID: &targetID, DocumentID: uintTestPointer(12)},
		Node{ID: 8, WorkspaceID: 7, SpaceID: 1, Type: NodeFolder, Title: FavoritesFolderTitle, SystemManaged: true, SystemKind: NodeSystemFavoritesFolder, SystemOwnerUserID: &ownerID},
		Node{ID: 9, WorkspaceID: 7, SpaceID: 1, ParentID: uintTestPointer(8), Type: NodeShortcut, Title: "Secret favorite", ShortcutTargetNodeID: &targetID, DocumentID: uintTestPointer(12), SystemManaged: true, SystemKind: NodeSystemFavoriteReference, SystemOwnerUserID: &ownerID},
	)
	tree, err := service.Tree(context.Background(), Principal{UserID: 5, WorkspaceID: 7})
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range tree.Nodes {
		if node.ID == 3 || node.ID == 7 || node.ID == 9 {
			t.Fatalf("shortcut leaked inaccessible target: %+v", tree.Nodes)
		}
	}
	if _, err := service.UpdateNodePreference(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 3,
		UpdateNodePreferenceInput{Pinned: boolTestPointer(true)},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("shortcut-derived preference access err=%v", err)
	}
}

func boolTestPointer(value bool) *bool { return &value }
