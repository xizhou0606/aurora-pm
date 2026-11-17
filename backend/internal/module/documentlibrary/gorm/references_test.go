package documentlibrarygorm

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"gorm.io/gorm"
	"testing"
)

type referenceTestChecker struct {
	referenced bool
	err        error
}

func (s *referenceTestChecker) HasResourceReferences(context.Context, uint, string) (bool, error) {
	return s.referenced, s.err
}
func TestArchiveChecksReferencesWhileResourceLocked(t *testing.T) {
	repo := testRepository(t)
	ctx := context.Background()
	p := documentlibrary.Principal{WorkspaceID: 7, UserID: 3}
	resource, _, err := repo.Create(ctx, p, documentlibrary.TypeRichText, "9", "document", nil)
	if err != nil {
		t.Fatal(err)
	}
	checker := &referenceTestChecker{referenced: true}
	repo.ConfigureReferenceChecker(func(*gorm.DB) documentlibrary.ReferenceChecker { return checker })
	if err = repo.Archive(ctx, p, resource.ID, 1); !errors.Is(err, documentlibrary.ErrConflict) {
		t.Fatalf("referenced archive: %v", err)
	}
	current, _, err := repo.Get(ctx, 7, resource.ID)
	if err != nil || current.ArchivedAt != nil || current.Revision != 1 {
		t.Fatalf("archive escaped guard: %+v %v", current, err)
	}
	checker.referenced = false
	if err = repo.Archive(ctx, p, resource.ID, 1); err != nil {
		t.Fatal(err)
	}
}
