package documentgorm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

// LockDocumentForCollaboration acquires the legacy source row in a
// caller-owned transaction. The app coordinator keeps this lock until the
// collaboration seed and cutover marker have committed together.
func LockDocumentForCollaboration(
	ctx context.Context,
	tx *gorm.DB,
	workspaceID, documentID uint,
) (document.Document, error) {
	if tx == nil || workspaceID == 0 || documentID == 0 {
		return document.Document{}, document.ErrInvalidInput
	}
	var row Document
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND id = ? AND archived_at IS NULL", workspaceID, documentID,
	).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return document.Document{}, document.ErrDocumentNotFound
	}
	if err != nil {
		return document.Document{}, err
	}
	return documentFromRecord(row), nil
}

// MarkDocumentCollaborationCutover changes only the authoritative mode flag.
// The immutable source revision/hash remains the seed provenance.
func MarkDocumentCollaborationCutover(
	ctx context.Context,
	tx *gorm.DB,
	workspaceID, documentID, sourceRevision uint,
	sourceHash string,
	cutoverAt time.Time,
) error {
	if tx == nil || workspaceID == 0 || documentID == 0 || sourceRevision == 0 || sourceHash == "" || cutoverAt.IsZero() {
		return document.ErrInvalidInput
	}
	result := tx.WithContext(ctx).Model(&Document{}).Where(
		"workspace_id = ? AND id = ? AND archived_at IS NULL AND collaboration_cutover_at IS NULL AND revision = ? AND content_hash = ?",
		workspaceID, documentID, sourceRevision, sourceHash,
	).Updates(map[string]any{"collaboration_cutover_at": cutoverAt, "updated_at": cutoverAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return document.ErrRevisionConflict
	}
	return nil
}
