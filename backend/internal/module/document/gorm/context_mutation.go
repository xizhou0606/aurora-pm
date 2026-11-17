package documentgorm

import (
	"context"
	"time"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) CopyDocumentNode(
	ctx context.Context,
	command document.CopyDocumentNodeCommand,
) (document.CreateNodeResult, error) {
	var result document.CreateNodeResult
	err := repository.withContextMutation(ctx, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		sourceNode, sourceBody, err := lockCopySource(tx, command)
		if err != nil {
			return err
		}
		if err := lockContextDestination(tx, command.WorkspaceID, sourceNode.SpaceID, command.ParentID); err != nil {
			return err
		}
		title, err := nextCopyTitle(tx, command.WorkspaceID, sourceNode.SpaceID, command.ParentID, sourceNode.Title)
		if err != nil {
			return err
		}
		sortOrder, err := nextSortOrderTx(tx, command.WorkspaceID, sourceNode.SpaceID, command.ParentID)
		if err != nil {
			return err
		}
		nodeRow := Node{
			WorkspaceID: command.WorkspaceID, SpaceID: sourceNode.SpaceID, ParentID: command.ParentID,
			Type: string(document.NodeDocument), Title: title, SortOrder: sortOrder,
			CreatedBy: command.ActorUserID, UpdatedBy: command.ActorUserID,
		}
		if err := tx.Create(&nodeRow).Error; err != nil {
			return err
		}
		bodyRow := Document{
			WorkspaceID: command.WorkspaceID, NodeID: nodeRow.ID,
			ContentJSON: sourceBody.ContentJSON, SchemaVersion: sourceBody.SchemaVersion,
			Revision: 1, ContentHash: sourceBody.ContentHash, PlainText: sourceBody.PlainText,
			OwnerUserID: command.ActorUserID, CreatedBy: command.ActorUserID, UpdatedBy: command.ActorUserID,
		}
		if err := tx.Create(&bodyRow).Error; err != nil {
			return err
		}
		if err := copyAssetReferences(tx, command.WorkspaceID, sourceBody.ID, bodyRow.ID, command.ActorUserID); err != nil {
			return err
		}
		createdNode := nodeFromRecord(nodeRow)
		createdNode.DocumentID = uintPointer(bodyRow.ID)
		createdBody := documentFromRecord(bodyRow)
		if repository.resourceLifecycleFactory != nil {
			lifecycle, err := repository.resourceLifecycleFactory(tx)
			if err != nil || lifecycle == nil {
				if err != nil {
					return err
				}
				return document.ErrContextActionsUnavailable
			}
			resourceID, err := lifecycle.CreatePrivateDocumentResource(
				ctx,
				document.Principal{WorkspaceID: command.WorkspaceID, UserID: command.ActorUserID},
				createdNode,
				createdBody,
			)
			if err != nil {
				return err
			}
			createdNode.ResourceID, createdBody.ResourceID = resourceID, resourceID
		}
		result = document.CreateNodeResult{Node: createdNode, Document: &createdBody}
		command.Effect.ResourceID = nodeRow.ID
		command.Effect.Payload["node_id"] = nodeRow.ID
		command.Effect.Payload["document_id"] = bodyRow.ID
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return result, err
}

func (repository *Repository) CreateShortcut(
	ctx context.Context,
	command document.CreateShortcutCommand,
) (document.Node, error) {
	var created document.Node
	err := repository.withContextMutation(ctx, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		target, body, err := lockShortcutTarget(tx, command.WorkspaceID, command.TargetNode.ID)
		if err != nil {
			return err
		}
		if err := lockContextDestination(tx, command.WorkspaceID, target.SpaceID, command.ParentID); err != nil {
			return err
		}
		sortOrder, err := nextSortOrderTx(tx, command.WorkspaceID, target.SpaceID, command.ParentID)
		if err != nil {
			return err
		}
		row := Node{
			DirectoryOwnerUserID: command.DirectoryOwnerUserID,
			WorkspaceID:          command.WorkspaceID, SpaceID: target.SpaceID, ParentID: command.ParentID,
			Type: string(document.NodeShortcut), Title: "", SortOrder: sortOrder,
			ShortcutTargetNodeID: &target.ID, CreatedBy: command.ActorUserID, UpdatedBy: command.ActorUserID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		created = nodeFromRecord(row)
		created.Title = target.Title
		created.DocumentID = uintPointer(body.ID)
		command.Effect.ResourceID = row.ID
		command.Effect.Payload["shortcut_node_id"] = row.ID
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return created, err
}

func (repository *Repository) UpdateNodePreference(
	ctx context.Context,
	command document.UpdateNodePreferenceCommand,
) (document.NodePreference, error) {
	var result document.NodePreference
	err := repository.withContextMutation(ctx, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		target, err := lockActiveNode(tx, command.WorkspaceID, command.TargetNode.ID)
		if err != nil {
			return err
		}
		if target.SystemManaged {
			return document.ErrSystemManagedNode
		}
		now := time.Now().UTC()
		if command.Pinned != nil {
			if *command.Pinned {
				row := UserPreference{
					WorkspaceID: command.WorkspaceID, UserID: command.ActorUserID,
					NodeID: target.ID, PinnedAt: &now, CreatedAt: now, UpdatedAt: now,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "user_id"}, {Name: "node_id"}},
					DoUpdates: clause.Assignments(map[string]any{"pinned_at": now, "updated_at": now}),
				}).Create(&row).Error; err != nil {
					return err
				}
			} else if err := tx.Where(
				"workspace_id = ? AND user_id = ? AND node_id = ?",
				command.WorkspaceID, command.ActorUserID, target.ID,
			).Delete(&UserPreference{}).Error; err != nil {
				return err
			}
		}
		if command.Favorited != nil {
			if target.Type != string(document.NodeDocument) {
				return document.ErrShortcutTargetInvalid
			}
			if err := setFavorite(tx, command, target, *command.Favorited, now); err != nil {
				return err
			}
		}
		result, err = readNodePreference(tx, command.WorkspaceID, command.ActorUserID, target.ID, now)
		if err != nil {
			return err
		}
		command.Effect.ResourceID = target.ID
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return result, err
}

func (repository *Repository) TransferDocumentOwner(
	ctx context.Context,
	command document.TransferDocumentOwnerCommand,
) (document.Document, error) {
	var updated document.Document
	err := repository.withContextMutation(ctx, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		var row Document
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND id = ? AND archived_at IS NULL", command.WorkspaceID, command.DocumentID,
		).First(&row).Error; err != nil {
			return normalizeDocumentContextError(err)
		}
		if err := tx.Model(&row).Updates(map[string]any{
			"owner_user_id": command.OwnerUserID, "updated_by": command.ActorUserID, "updated_at": time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&row, row.ID).Error; err != nil {
			return err
		}
		updated = documentFromRecord(row)
		if repository.resourceLifecycleFactory != nil {
			var nodeRow Node
			if err := tx.Where("workspace_id = ? AND id = ? AND archived_at IS NULL", command.WorkspaceID, row.NodeID).
				First(&nodeRow).Error; err != nil {
				return normalizeDocumentContextError(err)
			}
			lifecycle, err := repository.resourceLifecycleFactory(tx)
			if err != nil || lifecycle == nil {
				if err != nil {
					return err
				}
				return document.ErrContextActionsUnavailable
			}
			resourceID, err := lifecycle.SynchronizeDocumentResource(
				ctx,
				document.Principal{WorkspaceID: command.WorkspaceID, UserID: command.ActorUserID},
				nodeFromRecord(nodeRow),
				updated,
			)
			if err != nil {
				return err
			}
			updated.ResourceID = resourceID
		}
		command.Effect.ResourceID = row.ID
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return updated, err
}

func (repository *Repository) withContextMutation(
	ctx context.Context,
	apply func(*gorm.DB, document.MutationRecorder) error,
) error {
	if repository == nil || repository.db == nil || repository.mutationRecorderFactory == nil || apply == nil {
		return document.ErrContextActionsUnavailable
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		recorder, err := repository.mutationRecorderFactory(tx)
		if err != nil || recorder == nil {
			if err != nil {
				return err
			}
			return document.ErrContextActionsUnavailable
		}
		return apply(tx, recorder)
	})
}
