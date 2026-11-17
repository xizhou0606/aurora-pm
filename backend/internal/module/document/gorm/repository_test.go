package documentgorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func openDocumentDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(MigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRepositoryDefaultSpaceNodeAndRevisionCAS(t *testing.T) {
	db := openDocumentDB(t)
	repository := NewRepository(db)
	ctx := context.Background()
	space, err := repository.EnsureDefaultSpace(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	again, err := repository.EnsureDefaultSpace(ctx, 7)
	if err != nil || again.ID != space.ID {
		t.Fatalf("default space=%+v err=%v", again, err)
	}
	content, hash, plain, err := document.NormalizeContent(document.DefaultContent())
	if err != nil {
		t.Fatal(err)
	}
	node, body, err := repository.CreateNode(ctx, document.Node{
		WorkspaceID: 7, SpaceID: space.ID, Type: document.NodeDocument, Title: "ADR", SortOrder: 1024,
		CreatedBy: 1, UpdatedBy: 1,
	}, &document.Document{
		WorkspaceID: 7, Content: content, SchemaVersion: 1, Revision: 1,
		ContentHash: hash, PlainText: plain, CreatedBy: 1, UpdatedBy: 1,
	})
	if err != nil || node.DocumentID == nil || body == nil || body.NodeID != node.ID {
		t.Fatalf("node=%+v body=%+v err=%v", node, body, err)
	}
	node.Title = "Renamed ADR"
	renamed, err := repository.UpdateNode(ctx, node)
	if err != nil || renamed.DocumentID == nil || *renamed.DocumentID != body.ID {
		t.Fatalf("renamed=%+v err=%v", renamed, err)
	}
	body.PlainText = "changed"
	updated, err := repository.UpdateDocument(ctx, *body, 1, 2)
	if err != nil || updated.Revision != 2 || updated.UpdatedBy != 2 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	_, err = repository.UpdateDocument(ctx, *body, 1, 2)
	var conflict document.RevisionConflictError
	if !errors.As(err, &conflict) || conflict.LatestRevision != 2 {
		t.Fatalf("conflict=%v latest=%d", err, conflict.LatestRevision)
	}
	if _, err := repository.GetDocument(ctx, 8, body.ID); !errors.Is(err, document.ErrDocumentNotFound) {
		t.Fatalf("cross-workspace get=%v", err)
	}
	if err := repository.ArchiveNodes(ctx, 7, []uint{node.ID}, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetNode(ctx, 7, node.ID); !errors.Is(err, document.ErrNodeNotFound) {
		t.Fatalf("archived node get=%v", err)
	}
	if _, err := repository.GetDocument(ctx, 7, body.ID); !errors.Is(err, document.ErrDocumentNotFound) {
		t.Fatalf("archived document must not be readable or exportable: %v", err)
	}
}

func TestRepositoryPrimaryRequirementBindingIsIdempotent(t *testing.T) {
	db := openDocumentDB(t)
	repository := NewRepository(db)
	ctx := context.Background()
	first, err := repository.CreateBinding(ctx, document.Binding{
		WorkspaceID: 7, DocumentID: 11, RequirementID: 21,
		Kind: document.BindingPrimaryProductDoc, CreatedBy: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.CreateBinding(ctx, document.Binding{
		WorkspaceID: 7, DocumentID: 11, RequirementID: 21,
		Kind: document.BindingPrimaryProductDoc, CreatedBy: 1,
	})
	if err != nil || replayed.ID != first.ID {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	_, err = repository.CreateBinding(ctx, document.Binding{
		WorkspaceID: 7, DocumentID: 12, RequirementID: 21,
		Kind: document.BindingPrimaryProductDoc, CreatedBy: 1,
	})
	if !errors.Is(err, document.ErrBindingConflict) {
		t.Fatalf("different primary document err=%v", err)
	}
}

func TestRepositoryArchivedDocumentReleasesActivePrimaryBinding(t *testing.T) {
	db := openDocumentDB(t)
	repository := NewRepository(db)
	ctx := context.Background()
	space, err := repository.EnsureDefaultSpace(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	content, hash, plain, err := document.NormalizeContent(document.DefaultContent())
	if err != nil {
		t.Fatal(err)
	}
	createDocument := func(title string) (document.Node, document.Document) {
		node, body, createErr := repository.CreateNode(ctx, document.Node{
			WorkspaceID: 7, SpaceID: space.ID, Type: document.NodeDocument, Title: title,
			SortOrder: 1024, CreatedBy: 1, UpdatedBy: 1,
		}, &document.Document{
			WorkspaceID: 7, Content: content, SchemaVersion: 1, Revision: 1,
			ContentHash: hash, PlainText: plain, CreatedBy: 1, UpdatedBy: 1,
		})
		if createErr != nil || body == nil {
			t.Fatalf("create %q body=%+v err=%v", title, body, createErr)
		}
		return node, *body
	}
	firstNode, firstDocument := createDocument("first")
	_, secondDocument := createDocument("second")
	firstBinding, err := repository.CreateBinding(ctx, document.Binding{
		WorkspaceID: 7, DocumentID: firstDocument.ID, RequirementID: 21,
		Kind: document.BindingPrimaryProductDoc, CreatedBy: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.ArchiveNodes(ctx, 7, []uint{firstNode.ID}, 2); err != nil {
		t.Fatal(err)
	}
	active, err := repository.ListBindings(ctx, 7, firstDocument.ID)
	if err != nil || len(active) != 0 {
		t.Fatalf("active bindings=%+v err=%v", active, err)
	}
	var archived Binding
	if err := db.First(&archived, firstBinding.ID).Error; err != nil || archived.ArchivedAt == nil {
		t.Fatalf("archived binding=%+v err=%v", archived, err)
	}
	secondBinding, err := repository.CreateBinding(ctx, document.Binding{
		WorkspaceID: 7, DocumentID: secondDocument.ID, RequirementID: 21,
		Kind: document.BindingPrimaryProductDoc, CreatedBy: 1,
	})
	if err != nil || secondBinding.DocumentID != secondDocument.ID {
		t.Fatalf("replacement binding=%+v err=%v", secondBinding, err)
	}
}
