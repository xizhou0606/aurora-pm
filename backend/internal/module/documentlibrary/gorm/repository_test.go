package documentlibrarygorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"gorm.io/gorm"
)

func testRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(MigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	return NewRepository(db)
}

func TestRepositoryScopesBindingsByWorkspaceAndUsesCAS(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	resource, binding, err := repository.Create(ctx, documentlibrary.Principal{UserID: 7, WorkspaceID: 11}, documentlibrary.TypeRichText, "42", "方案", nil)
	if err != nil {
		t.Fatal(err)
	}
	if binding.ResourceID != resource.ID || resource.OwnerUserID != 7 || resource.Revision != 1 {
		t.Fatalf("unexpected create result: %#v %#v", resource, binding)
	}
	if _, _, err := repository.Get(ctx, 12, resource.ID); !errors.Is(err, documentlibrary.ErrNotFound) {
		t.Fatalf("cross workspace get error = %v", err)
	}
	if _, err := repository.UpdateMetadata(ctx, documentlibrary.Principal{UserID: 7, WorkspaceID: 11}, resource.ID, 2, "新名称", nil, nil); !errors.Is(err, documentlibrary.ErrConflict) {
		t.Fatalf("stale revision error = %v", err)
	}
	updated, err := repository.UpdateMetadata(ctx, documentlibrary.Principal{UserID: 7, WorkspaceID: 11}, resource.ID, 1, "新名称", nil, nil)
	if err != nil || updated.Revision != 2 || updated.Name != "新名称" {
		t.Fatalf("update = %#v, %v", updated, err)
	}
}

func TestRepositoryResolvesContentBindingsInOneWorkspaceScopedBatch(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	first, _, err := repository.Create(ctx, documentlibrary.Principal{UserID: 7, WorkspaceID: 11}, documentlibrary.TypeRichText, "41", "一", nil)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := repository.Create(ctx, documentlibrary.Principal{UserID: 7, WorkspaceID: 11}, documentlibrary.TypeRichText, "42", "二", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = repository.Create(ctx, documentlibrary.Principal{UserID: 8, WorkspaceID: 12}, documentlibrary.TypeRichText, "43", "三", nil)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := repository.ResolveContents(ctx, 11, documentlibrary.TypeRichText, []string{"42", "41", "43"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 2 || resolved["41"].ID != first.ID || resolved["42"].ID != second.ID {
		t.Fatalf("resolved=%+v", resolved)
	}
}
