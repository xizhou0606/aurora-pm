package api

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"gorm.io/gorm"
)

// This adapter is only installed into CreateNode's insertion transaction.
// It never calls the maintenance cutover executor or scans existing documents.
type newDocumentCollaboration struct{ tx *gorm.DB }

func newDocumentCollaborationFactory(tx *gorm.DB) (document.NewDocumentCollaborationInitializer, error) {
	if tx == nil {
		return nil, document.ErrInvalidInput
	}
	return newDocumentCollaboration{tx: tx}, nil
}

func (initializer newDocumentCollaboration) InitializeNewDocument(ctx context.Context, source document.Document) (*time.Time, error) {
	_, blankHash, _, err := document.NormalizeContent(document.DefaultContent())
	if err != nil {
		return nil, err
	}
	// Imported/nonempty documents retain their original mode. Existing documents
	// cannot reach this insertion-only seam, even if their body happens to be blank.
	if source.Revision != 1 || source.ContentHash != blankHash {
		return nil, nil
	}
	if source.ResourceID == "" || source.CollaborationCutoverAt != nil {
		return nil, document.ErrInvalidInput
	}
	policy, err := resourceaccessgorm.NewRepository(initializer.tx).LockCurrentPolicy(ctx, source.WorkspaceID, documentlibrary.ResourceID(source.ResourceID))
	if err != nil {
		return nil, err
	}
	seed, err := collaboration.NewEmptyDocumentSeed(collaboration.Room{WorkspaceID: source.WorkspaceID, ResourceID: source.ResourceID}, source.CreatedBy, policy.Revision, source.ContentHash)
	if err != nil {
		return nil, err
	}
	store, err := collaborationgorm.NewDocumentTransaction(initializer.tx)
	if err != nil {
		return nil, err
	}
	state, err := store.SeedDocument(ctx, seed)
	if err != nil {
		return nil, err
	}
	at := state.SeededAt
	if err = documentgorm.MarkDocumentCollaborationCutover(ctx, initializer.tx, source.WorkspaceID, source.ID, source.Revision, source.ContentHash, at); err != nil {
		return nil, err
	}
	return &at, nil
}
