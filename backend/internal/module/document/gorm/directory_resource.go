package documentgorm

import (
	"context"
	"errors"
	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
)

func resourcePlacement(tx *gorm.DB, space PersonalSpace, key string, seed *uint) (DirectoryResourceEntry, error) {
	var row DirectoryResourceEntry
	if key == "" {
		return row, document.ErrInvalidInput
	}
	err := tx.Where("space_id = ? AND resource_id = ?", space.ID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if seed != nil {
			var count int64
			if err := tx.Model(&DirectoryEntry{}).Where("space_id = ? AND node_id = ? AND removed = ?", space.ID, *seed, false).Count(&count).Error; err != nil {
				return row, err
			}
			if count == 0 {
				seed = nil
			}
		}
		row = DirectoryResourceEntry{SpaceID: space.ID, ResourceID: key, ParentID: seed, Revision: 1}
		err = tx.Create(&row).Error
	}
	return row, err
}
func (repository *Repository) ResourceDirectoryLocation(ctx context.Context, p document.Principal, key string, seed *uint) (document.ResourceDirectoryLocation, error) {
	var result document.ResourceDirectoryLocation
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		space, err := lockPersonalSpace(tx, p)
		if err != nil {
			return err
		}
		row, err := resourcePlacement(tx, space, key, seed)
		if err != nil {
			return err
		}
		parent := row.ParentID
		if parent != nil {
			var count int64
			if err := tx.Model(&DirectoryEntry{}).Where("space_id = ? AND node_id = ? AND removed = ?", space.ID, *parent, false).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				parent = nil
			}
		}
		result = document.ResourceDirectoryLocation{ParentID: parent, Revision: row.Revision}
		return nil
	})
	return result, err
}
func (repository *Repository) MoveResourceDirectory(ctx context.Context, p document.Principal, key string, parent *uint, expected uint64, operation string) (document.ResourceDirectoryLocation, error) {
	var result document.ResourceDirectoryLocation
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		space, err := lockPersonalSpace(tx, p)
		if err != nil {
			return err
		}
		row, err := resourcePlacement(tx, space, key, nil)
		if err != nil {
			return err
		}
		if operation == "" {
			return document.ErrInvalidInput
		}
		if row.LastOperationID == operation {
			if !sameDirectoryParent(row.ParentID, parent) {
				return document.ErrDirectoryConflict
			}
			result = document.ResourceDirectoryLocation{ParentID: row.ParentID, Revision: row.Revision}
			return nil
		}
		if row.Revision != expected {
			return document.ErrDirectoryConflict
		}
		if parent != nil {
			var count int64
			if err := tx.Model(&DirectoryEntry{}).Where("space_id = ? AND node_id = ? AND removed = ?", space.ID, *parent, false).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return document.ErrNodeNotFound
			}
		}
		row.ParentID = parent
		row.Revision++
		row.LastOperationID = operation
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		result = document.ResourceDirectoryLocation{ParentID: row.ParentID, Revision: row.Revision}
		if repository.mutationRecorderFactory != nil {
			recorder, err := repository.mutationRecorderFactory(tx)
			if err != nil {
				return err
			}
			return recorder.RecordDocumentMutation(ctx, document.MutationEffect{WorkspaceID: p.WorkspaceID, PrincipalID: p.UserID, Resource: "personal_document_directory", ResourceID: space.ID, Action: "updated", Payload: map[string]any{"resource_id": key, "revision": row.Revision, "operation_id": operation}})
		}
		return nil
	})
	return result, err
}
