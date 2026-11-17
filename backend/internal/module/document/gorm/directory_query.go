package documentgorm

import (
	"context"
	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func lockPersonalSpace(tx *gorm.DB, p document.Principal) (PersonalSpace, error) {
	if p.WorkspaceID == 0 || p.UserID == 0 {
		return PersonalSpace{}, document.ErrForbidden
	}
	space := PersonalSpace{WorkspaceID: p.WorkspaceID, OwnerUserID: p.UserID, Revision: 1}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&space).Error; err != nil {
		return space, err
	}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND owner_user_id = ?", p.WorkspaceID, p.UserID).First(&space).Error
	return space, err
}

func (repository *Repository) ReadDirectory(ctx context.Context, p document.Principal, seeds []document.Node) (document.DirectorySnapshot, error) {
	var result document.DirectorySnapshot
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		space, err := lockPersonalSpace(tx, p)
		if err != nil {
			return err
		}
		var rows []DirectoryEntry
		if err := tx.Where("space_id = ?", space.ID).Find(&rows).Error; err != nil {
			return err
		}
		existing := map[uint]DirectoryEntry{}
		for _, row := range rows {
			existing[row.NodeID] = row
		}
		// Only access-filtered seeds arrive here. Removed entries remain tombstones.
		added := false
		for _, node := range seeds {
			if _, ok := existing[node.ID]; ok {
				continue
			}
			parent := node.ParentID
			if parent != nil {
				if old, ok := existing[*parent]; !ok || old.Removed {
					parent = nil
				}
			}
			row := DirectoryEntry{SpaceID: space.ID, NodeID: node.ID, ResourceID: node.ResourceID, ParentID: parent, SortOrder: node.SortOrder}
			if node.Type == document.NodeFolder {
				row.Title = node.Title
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			rows = append(rows, row)
			existing[node.ID] = row
			added = true
		}
		if added {
			space.Revision++
			if err := tx.Model(&space).Updates(map[string]any{"revision": space.Revision, "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		result.SpaceID, result.Revision = space.ID, space.Revision
		for _, row := range rows {
			result.Entries = append(result.Entries, document.DirectoryEntry{NodeID: row.NodeID, ParentID: row.ParentID, SortOrder: row.SortOrder, Title: row.Title, Removed: row.Removed})
		}
		return nil
	})
	return result, err
}

func (repository *Repository) InDirectoryTransaction(ctx context.Context, apply func(document.Store, document.MutationRecorder) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		copy := *repository
		copy.db = tx
		var recorder document.MutationRecorder
		if repository.mutationRecorderFactory != nil {
			var err error
			recorder, err = repository.mutationRecorderFactory(tx)
			if err != nil {
				return err
			}
		}
		return apply(&copy, recorder)
	})
}

func (repository *Repository) DirectoryPrincipals(ctx context.Context) ([]document.Principal, error) {
	var spaces []PersonalSpace
	if err := repository.db.WithContext(ctx).Order("id").Find(&spaces).Error; err != nil {
		return nil, err
	}
	result := make([]document.Principal, 0, len(spaces))
	for _, space := range spaces {
		result = append(result, document.Principal{WorkspaceID: space.WorkspaceID, UserID: space.OwnerUserID})
	}
	return result, nil
}
