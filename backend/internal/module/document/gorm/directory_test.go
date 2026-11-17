package documentgorm

import (
	"context"
	"errors"
	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"testing"
)

type directoryTestAccess struct{ revoked bool }

func (*directoryTestAccess) EffectiveDocumentActionCodes(context.Context, uint, uint) ([]string, error) {
	return []string{string(document.ActionNodesView), string(document.ActionNodesCreate), string(document.ActionNodesMove), string(document.ActionNodesUpdate), string(document.ActionNodesArchive), string(document.ActionScopeAll)}, nil
}
func (*directoryTestAccess) ResolveRequirementParticipants(context.Context, uint, uint) (document.RequirementParticipants, error) {
	return document.RequirementParticipants{}, nil
}
func (a *directoryTestAccess) ResolveDocumentAccess(context.Context, document.Principal, uint) (document.ResourceAccess, error) {
	return document.ResourceAccess{ResourceID: "resource-test", CanView: !a.revoked}, nil
}
func (a *directoryTestAccess) ResolveDocumentsAccess(_ context.Context, _ document.Principal, ids []uint) (map[uint]document.ResourceAccess, error) {
	r := map[uint]document.ResourceAccess{}
	for _, id := range ids {
		r[id] = document.ResourceAccess{ResourceID: "resource-test", CanView: !a.revoked}
	}
	return r, nil
}

func TestPersonalDirectoryIsolationAndResourceVisibility(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(openDocumentDB(t))
	access := &directoryTestAccess{}
	service, err := document.NewService(repo, access, access, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureResourceAccess(access)
	a := document.Principal{WorkspaceID: 7, UserID: 5}
	b := document.Principal{WorkspaceID: 7, UserID: 6}
	folder, err := service.PersonalCreateNode(ctx, a, document.CreateNodeInput{Type: document.NodeFolder, Title: "A folder"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateNode(ctx, a, document.CreateNodeInput{Type: document.NodeDocument, Title: "shared readable"})
	if err != nil {
		t.Fatal(err)
	}
	ta, err := service.PersonalTree(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	tb, err := service.PersonalTree(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if ta.Space.ID == tb.Space.ID {
		t.Fatal("spaces shared")
	}
	for _, n := range tb.Nodes {
		if n.ID == folder.Node.ID {
			t.Fatal("private folder leaked")
		}
	}
	// Read access is sufficient for arranging one's directory, while shared file move is denied.
	if _, err := service.MoveNode(ctx, a, created.Node.ID, document.MoveNodeInput{}); !errors.Is(err, document.ErrForbidden) {
		t.Fatalf("shared move=%v", err)
	}
	moved, err := service.PersonalMoveNode(ctx, a, created.Node.ID, document.MoveNodeInput{ParentID: &folder.Node.ID})
	if err != nil || moved.ParentID == nil {
		t.Fatalf("move=%+v err=%v", moved, err)
	}
	original, err := repo.GetNode(ctx, 7, created.Node.ID)
	if err != nil || original.ParentID != nil || original.SortOrder != created.Node.SortOrder {
		t.Fatalf("shared node mutated: %+v %v", original, err)
	}
	tb, err = service.PersonalTree(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tb.Nodes {
		if n.ID == created.Node.ID && n.ParentID != nil {
			t.Fatal("B changed")
		}
	}
	if _, err := service.PersonalMoveNode(ctx, b, created.Node.ID, document.MoveNodeInput{ParentID: &folder.Node.ID}); !errors.Is(err, document.ErrNodeNotFound) {
		t.Fatalf("foreign parent=%v", err)
	}
	if _, err := service.PersonalMoveNode(ctx, document.Principal{WorkspaceID: 8, UserID: 5}, created.Node.ID, document.MoveNodeInput{}); !errors.Is(err, document.ErrNodeNotFound) {
		t.Fatalf("cross tenant=%v", err)
	}
	root, err := service.PersonalMoveNode(ctx, a, created.Node.ID, document.MoveNodeInput{})
	if err != nil || root.ParentID != nil {
		t.Fatalf("root=%+v %v", root, err)
	}
	if _, err := service.PersonalRenameNode(ctx, a, folder.Node.ID, "private title"); err != nil {
		t.Fatal(err)
	}
	if err := service.PersonalArchiveNode(ctx, a, created.Node.ID, document.ArchiveConfirmation{}); err != nil {
		t.Fatal(err)
	}
	ta, _ = service.PersonalTree(ctx, a)
	for _, n := range ta.Nodes {
		if n.ID == created.Node.ID {
			t.Fatal("removed entry returned")
		}
	}
	if _, err := repo.GetDocument(ctx, 7, created.Document.ID); err != nil {
		t.Fatalf("file was deleted: %v", err)
	}
	access.revoked = true
	tb, err = service.PersonalTree(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tb.Nodes {
		if n.ID == created.Node.ID {
			t.Fatal("revoked file leaked")
		}
	}
}

func TestPersonalDirectoryOrderingConflictAndCycles(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(openDocumentDB(t))
	access := &directoryTestAccess{}
	service, _ := document.NewService(repo, access, access, nil)
	p := document.Principal{WorkspaceID: 7, UserID: 5}
	first, err := service.PersonalCreateNode(ctx, p, document.CreateNodeInput{Type: document.NodeFolder, Title: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.PersonalCreateNode(ctx, p, document.CreateNodeInput{Type: document.NodeFolder, Title: "second"})
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := service.PersonalTree(ctx, p)
	old := tree.Nodes[0].DirectoryRevision
	moved, err := service.PersonalMoveNode(ctx, p, second.Node.ID, document.MoveNodeInput{BeforeNodeID: &first.Node.ID, ExpectedDirectoryRevision: &old})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PersonalMoveNode(ctx, p, first.Node.ID, document.MoveNodeInput{ExpectedDirectoryRevision: &old}); !errors.Is(err, document.ErrDirectoryConflict) {
		t.Fatalf("stale update=%v", err)
	}
	for i := 0; i < 30; i++ {
		if _, err := service.PersonalMoveNode(ctx, p, first.Node.ID, document.MoveNodeInput{BeforeNodeID: &moved.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.PersonalMoveNode(ctx, p, second.Node.ID, document.MoveNodeInput{ParentID: &first.Node.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PersonalMoveNode(ctx, p, first.Node.ID, document.MoveNodeInput{ParentID: &second.Node.ID}); !errors.Is(err, document.ErrHierarchyCycle) {
		t.Fatalf("cycle=%v", err)
	}
}

func TestPersonalResourceDirectoryIsolationAndReplay(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(openDocumentDB(t))
	access := &directoryTestAccess{}
	service, _ := document.NewService(repo, access, access, nil)
	a := document.Principal{WorkspaceID: 7, UserID: 5}
	b := document.Principal{WorkspaceID: 7, UserID: 6}
	folder, err := service.PersonalCreateNode(ctx, a, document.CreateNodeInput{Type: document.NodeFolder, Title: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := repo.ResourceDirectoryLocation(ctx, a, "bitable:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := repo.MoveResourceDirectory(ctx, a, "bitable:1", &folder.Node.ID, initial.Revision, "move-1")
	if err != nil || moved.ParentID == nil {
		t.Fatalf("move=%+v %v", moved, err)
	}
	replay, err := repo.MoveResourceDirectory(ctx, a, "bitable:1", &folder.Node.ID, initial.Revision, "move-1")
	if err != nil || replay.Revision != moved.Revision {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	if _, err := repo.MoveResourceDirectory(ctx, a, "bitable:1", nil, initial.Revision, "stale"); !errors.Is(err, document.ErrDirectoryConflict) {
		t.Fatalf("conflict=%v", err)
	}
	other, err := repo.ResourceDirectoryLocation(ctx, b, "bitable:1", nil)
	if err != nil || other.ParentID != nil || other.Revision != 1 {
		t.Fatalf("other=%+v %v", other, err)
	}
	if _, err := repo.MoveResourceDirectory(ctx, b, "bitable:1", &folder.Node.ID, 1, "foreign"); !errors.Is(err, document.ErrNodeNotFound) {
		t.Fatalf("foreign=%v", err)
	}
	root, err := repo.MoveResourceDirectory(ctx, a, "bitable:1", nil, moved.Revision, "root")
	if err != nil || root.ParentID != nil {
		t.Fatalf("root=%+v %v", root, err)
	}
}
