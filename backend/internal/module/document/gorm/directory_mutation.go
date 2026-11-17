package documentgorm

import (
	"context"
	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
	"sort"
	"time"
)

func sameDirectoryParent(a, b *uint) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (repository *Repository) ChangeDirectory(ctx context.Context, p document.Principal, command document.DirectoryCommand) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		space, err := lockPersonalSpace(tx, p)
		if err != nil {
			return err
		}
		if command.ExpectedRevision != nil && *command.ExpectedRevision != space.Revision {
			return document.ErrDirectoryConflict
		}
		var rows []DirectoryEntry
		if err := tx.Where("space_id = ? AND removed = ?", space.ID, false).Order("sort_order, node_id").Find(&rows).Error; err != nil {
			return err
		}
		byID := map[uint]DirectoryEntry{}
		for _, row := range rows {
			byID[row.NodeID] = row
		}
		node, ok := byID[command.NodeID]
		if !ok {
			return document.ErrNodeNotFound
		}
		var source Node
		if err := tx.Where("workspace_id = ? AND id = ? AND archived_at IS NULL", p.WorkspaceID, node.NodeID).First(&source).Error; err != nil {
			return document.ErrNodeNotFound
		}
		if source.SystemManaged {
			return document.ErrSystemManagedNode
		}
		now := time.Now().UTC()
		if command.Title != nil {
			if source.Type != string(document.NodeFolder) {
				return document.ErrInvalidInput
			}
			if err := tx.Model(&node).Updates(map[string]any{"title": *command.Title, "updated_at": now}).Error; err != nil {
				return err
			}
		} else if command.Remove {
			ids := map[uint]bool{node.NodeID: true}
			for changed := true; changed; {
				changed = false
				for _, row := range rows {
					if row.ParentID != nil && ids[*row.ParentID] && !ids[row.NodeID] {
						ids[row.NodeID] = true
						changed = true
					}
				}
			}
			values := make([]uint, 0, len(ids))
			for id := range ids {
				values = append(values, id)
			}
			if err := tx.Model(&DirectoryEntry{}).Where("space_id = ? AND node_id IN ?", space.ID, values).Updates(map[string]any{"removed": true, "updated_at": now}).Error; err != nil {
				return err
			}
		} else {
			if command.BeforeNodeID != nil && command.AfterNodeID != nil {
				return document.ErrInvalidInput
			}
			seen := map[uint]bool{node.NodeID: true}
			for parent := command.ParentID; parent != nil; {
				if seen[*parent] {
					return document.ErrHierarchyCycle
				}
				seen[*parent] = true
				ancestor, exists := byID[*parent]
				if !exists {
					return document.ErrNodeNotFound
				}
				var folder Node
				if err := tx.Where("workspace_id = ? AND id = ? AND type = ? AND archived_at IS NULL", p.WorkspaceID, *parent, string(document.NodeFolder)).First(&folder).Error; err != nil {
					return document.ErrParentNotFolder
				}
				if folder.SystemManaged {
					return document.ErrSystemManagedNode
				}
				parent = ancestor.ParentID
			}
			siblings := []DirectoryEntry{}
			for _, row := range rows {
				if row.NodeID != node.NodeID && sameDirectoryParent(row.ParentID, command.ParentID) {
					siblings = append(siblings, row)
				}
			}
			sort.SliceStable(siblings, func(i, j int) bool {
				if siblings[i].SortOrder == siblings[j].SortOrder {
					return siblings[i].NodeID < siblings[j].NodeID
				}
				return siblings[i].SortOrder < siblings[j].SortOrder
			})
			index := len(siblings)
			target := command.BeforeNodeID
			if target == nil {
				target = command.AfterNodeID
			}
			if target != nil {
				index = -1
				for i, row := range siblings {
					if row.NodeID == *target {
						index = i
						if command.AfterNodeID != nil {
							index++
						}
						break
					}
				}
				if index < 0 {
					return document.ErrInvalidInput
				}
			}
			node.ParentID = command.ParentID
			siblings = append(siblings, DirectoryEntry{})
			copy(siblings[index+1:], siblings[index:])
			siblings[index] = node
			// Rebalance within this user's space in the same locked transaction.
			for i, row := range siblings {
				if err := tx.Model(&DirectoryEntry{}).Where("space_id = ? AND id = ?", space.ID, row.ID).Updates(map[string]any{"parent_id": row.ParentID, "sort_order": int64(i+1) * 1024, "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&space).Updates(map[string]any{"revision": space.Revision + 1, "updated_at": now}).Error; err != nil {
			return err
		}
		if repository.mutationRecorderFactory != nil {
			recorder, err := repository.mutationRecorderFactory(tx)
			if err != nil {
				return err
			}
			return recorder.RecordDocumentMutation(ctx, document.MutationEffect{WorkspaceID: p.WorkspaceID, PrincipalID: p.UserID, Resource: "personal_document_directory", ResourceID: space.ID, Action: "updated", Payload: map[string]any{"node_id": node.NodeID, "revision": space.Revision + 1}})
		}
		return nil
	})
}
