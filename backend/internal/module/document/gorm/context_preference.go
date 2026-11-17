package documentgorm

import (
	"errors"
	"time"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setFavorite(
	tx *gorm.DB,
	command document.UpdateNodePreferenceCommand,
	target Node,
	favorited bool,
	now time.Time,
) error {
	if !favorited {
		return tx.Model(&Node{}).Where(
			"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND shortcut_target_node_id = ? AND archived_at IS NULL",
			command.WorkspaceID, command.ActorUserID, string(document.NodeSystemFavoriteReference), target.ID,
		).Updates(map[string]any{
			"archived_at": now, "archived_by": command.ActorUserID,
			"updated_at": now, "updated_by": command.ActorUserID,
		}).Error
	}
	folder, err := ensureFavoritesFolder(tx, command, target.SpaceID, now)
	if err != nil {
		return err
	}
	var existing Node
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND shortcut_target_node_id = ? AND archived_at IS NULL",
		command.WorkspaceID, command.ActorUserID, string(document.NodeSystemFavoriteReference), target.ID,
	).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	sortOrder, err := nextSortOrderTx(tx, command.WorkspaceID, target.SpaceID, &folder.ID)
	if err != nil {
		return err
	}
	created := Node{
		WorkspaceID: command.WorkspaceID, SpaceID: target.SpaceID, ParentID: &folder.ID,
		Type: string(document.NodeShortcut), Title: "", SortOrder: sortOrder,
		ShortcutTargetNodeID: &target.ID, SystemManaged: true,
		SystemKind: string(document.NodeSystemFavoriteReference), SystemOwnerUserID: &command.ActorUserID,
		CreatedBy: command.ActorUserID, UpdatedBy: command.ActorUserID, CreatedAt: now, UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created).Error
}

func ensureFavoritesFolder(
	tx *gorm.DB,
	command document.UpdateNodePreferenceCommand,
	spaceID uint,
	now time.Time,
) (Node, error) {
	var folder Node
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND archived_at IS NULL",
		command.WorkspaceID, command.ActorUserID, string(document.NodeSystemFavoritesFolder),
	).First(&folder).Error
	if err == nil {
		return folder, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Node{}, err
	}
	if err := lockContextDestination(tx, command.WorkspaceID, spaceID, nil); err != nil {
		return Node{}, err
	}
	folder = Node{
		WorkspaceID: command.WorkspaceID, SpaceID: spaceID, Type: string(document.NodeFolder),
		Title: document.FavoritesFolderTitle, SortOrder: 0, SystemManaged: true,
		SystemKind: string(document.NodeSystemFavoritesFolder), SystemOwnerUserID: &command.ActorUserID,
		CreatedBy: command.ActorUserID, UpdatedBy: command.ActorUserID, CreatedAt: now, UpdatedAt: now,
	}
	create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&folder)
	if create.Error != nil {
		return Node{}, create.Error
	}
	if create.RowsAffected == 0 {
		if err := tx.Where(
			"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND archived_at IS NULL",
			command.WorkspaceID, command.ActorUserID, string(document.NodeSystemFavoritesFolder),
		).First(&folder).Error; err != nil {
			return Node{}, err
		}
	}
	return folder, nil
}

func readNodePreference(
	tx *gorm.DB,
	workspaceID, userID, nodeID uint,
	now time.Time,
) (document.NodePreference, error) {
	result := document.NodePreference{NodeID: nodeID, UpdatedAt: now}
	var preference UserPreference
	err := tx.Where(
		"workspace_id = ? AND user_id = ? AND node_id = ? AND pinned_at IS NOT NULL",
		workspaceID, userID, nodeID,
	).First(&preference).Error
	if err == nil {
		result.PinnedAt = preference.PinnedAt
		result.UpdatedAt = preference.UpdatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return document.NodePreference{}, err
	}
	var count int64
	if err := tx.Model(&Node{}).Where(
		"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND shortcut_target_node_id = ? AND archived_at IS NULL",
		workspaceID, userID, string(document.NodeSystemFavoriteReference), nodeID,
	).Count(&count).Error; err != nil {
		return document.NodePreference{}, err
	}
	result.Favorited = count > 0
	return result, nil
}
