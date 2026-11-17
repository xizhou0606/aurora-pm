package documentgorm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func (repository *Repository) GetDocument(ctx context.Context, workspaceID, id uint) (document.Document, error) {
	var row Document
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ? AND archived_at IS NULL", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return document.Document{}, document.ErrDocumentNotFound
	}
	return documentFromRecord(row), err
}

func (repository *Repository) UpdateDocument(
	ctx context.Context,
	body document.Document,
	expectedRevision uint,
	actorID uint,
) (document.Document, error) {
	now := time.Now().UTC()
	result := repository.db.WithContext(ctx).Model(&Document{}).
		Where(
			"workspace_id = ? AND id = ? AND revision = ? AND archived_at IS NULL AND collaboration_cutover_at IS NULL",
			body.WorkspaceID, body.ID, expectedRevision,
		).
		Updates(map[string]any{
			"content_json": string(body.Content), "schema_version": body.SchemaVersion,
			"content_hash": body.ContentHash, "plain_text": body.PlainText,
			"revision": gorm.Expr("revision + 1"), "updated_by": actorID, "updated_at": now,
		})
	if result.Error != nil {
		return document.Document{}, result.Error
	}
	if result.RowsAffected != 1 {
		latest, err := repository.GetDocument(ctx, body.WorkspaceID, body.ID)
		if err != nil {
			return document.Document{}, err
		}
		if latest.CollaborationCutoverAt != nil {
			return document.Document{}, document.ErrLegacyContentWriteDisabled
		}
		return document.Document{}, document.RevisionConflictError{LatestRevision: latest.Revision}
	}
	return repository.GetDocument(ctx, body.WorkspaceID, body.ID)
}

func (repository *Repository) ListBindings(
	ctx context.Context,
	workspaceID uint,
	documentID uint,
) ([]document.Binding, error) {
	return repository.listBindings(
		repository.db.WithContext(ctx).Where(
			"workspace_id = ? AND document_id = ? AND archived_at IS NULL",
			workspaceID,
			documentID,
		),
	)
}

func (repository *Repository) ListBindingsByDocumentIDs(
	ctx context.Context,
	workspaceID uint,
	documentIDs []uint,
) ([]document.Binding, error) {
	if len(documentIDs) == 0 {
		return []document.Binding{}, nil
	}
	return repository.listBindings(
		repository.db.WithContext(ctx).Where(
			"workspace_id = ? AND document_id IN ? AND archived_at IS NULL",
			workspaceID,
			documentIDs,
		),
	)
}

func (repository *Repository) listBindings(query *gorm.DB) ([]document.Binding, error) {
	var rows []Binding
	if err := query.Order("document_id ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]document.Binding, 0, len(rows))
	for _, row := range rows {
		result = append(result, bindingFromRecord(row))
	}
	return result, nil
}

func (repository *Repository) CreateBinding(ctx context.Context, value document.Binding) (document.Binding, error) {
	row := bindingRecord(value)
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		if !errors.Is(duplicateBindingError(err), document.ErrBindingConflict) {
			return document.Binding{}, err
		}
		var existing Binding
		lookupErr := repository.db.WithContext(ctx).Where(
			"workspace_id = ? AND requirement_id = ? AND kind = ? AND archived_at IS NULL",
			value.WorkspaceID,
			value.RequirementID,
			string(value.Kind),
		).First(&existing).Error
		if lookupErr != nil {
			return document.Binding{}, document.ErrBindingConflict
		}
		if existing.DocumentID != value.DocumentID {
			return document.Binding{}, document.ErrBindingConflict
		}
		return bindingFromRecord(existing), nil
	}
	return bindingFromRecord(row), nil
}

func (repository *Repository) DeleteBinding(
	ctx context.Context,
	workspaceID uint,
	documentID uint,
	bindingID uint,
) error {
	result := repository.db.WithContext(ctx).
		Where(
			"workspace_id = ? AND document_id = ? AND id = ? AND archived_at IS NULL",
			workspaceID,
			documentID,
			bindingID,
		).
		Delete(&Binding{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return document.ErrBindingNotFound
	}
	return nil
}
