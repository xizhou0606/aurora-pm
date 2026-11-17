package documentgorm

import (
	"context"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func (repository *Repository) hydrateNodeRecords(
	ctx context.Context,
	workspaceID uint,
	rows []Node,
) ([]document.Node, error) {
	if len(rows) == 0 {
		return []document.Node{}, nil
	}
	nodeIDs := make([]uint, 0, len(rows)*2)
	targetIDs := make([]uint, 0)
	for _, row := range rows {
		nodeIDs = append(nodeIDs, row.ID)
		if row.ShortcutTargetNodeID != nil {
			nodeIDs = append(nodeIDs, *row.ShortcutTargetNodeID)
			targetIDs = append(targetIDs, *row.ShortcutTargetNodeID)
		}
	}
	targets := make(map[uint]Node, len(targetIDs))
	if len(targetIDs) > 0 {
		var targetRows []Node
		if err := repository.db.WithContext(ctx).Where(
			"workspace_id = ? AND id IN ? AND type = ? AND archived_at IS NULL",
			workspaceID, targetIDs, string(document.NodeDocument),
		).Find(&targetRows).Error; err != nil {
			return nil, err
		}
		for _, target := range targetRows {
			targets[target.ID] = target
		}
	}
	documents := make(map[uint]uint, len(nodeIDs))
	var bodies []Document
	if err := repository.db.WithContext(ctx).Select("id", "node_id").Where(
		"workspace_id = ? AND node_id IN ? AND archived_at IS NULL", workspaceID, nodeIDs,
	).Find(&bodies).Error; err != nil {
		return nil, err
	}
	for _, body := range bodies {
		documents[body.NodeID] = body.ID
	}
	result := make([]document.Node, 0, len(rows))
	for _, row := range rows {
		item := nodeFromRecord(row)
		switch item.Type {
		case document.NodeFolder:
			result = append(result, item)
		case document.NodeDocument:
			documentID, exists := documents[row.ID]
			if !exists {
				continue
			}
			item.DocumentID = uintPointer(documentID)
			result = append(result, item)
		case document.NodeShortcut:
			if row.ShortcutTargetNodeID == nil {
				continue
			}
			target, exists := targets[*row.ShortcutTargetNodeID]
			if !exists {
				continue
			}
			documentID, exists := documents[target.ID]
			if !exists {
				continue
			}
			item.Title = target.Title
			item.DocumentID = uintPointer(documentID)
			result = append(result, item)
		}
	}
	return result, nil
}

func (repository *Repository) ListNodePreferences(
	ctx context.Context,
	workspaceID, userID uint,
	nodeIDs []uint,
) ([]document.NodePreference, error) {
	if len(nodeIDs) == 0 {
		return []document.NodePreference{}, nil
	}
	var rows []UserPreference
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND user_id = ? AND node_id IN ? AND pinned_at IS NOT NULL",
		workspaceID, userID, nodeIDs,
	).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]document.NodePreference, 0, len(rows))
	for _, row := range rows {
		result = append(result, document.NodePreference{
			NodeID: row.NodeID, PinnedAt: row.PinnedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	var favorites []Node
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND system_owner_user_id = ? AND system_kind = ? AND shortcut_target_node_id IN ? AND archived_at IS NULL",
		workspaceID, userID, string(document.NodeSystemFavoriteReference), nodeIDs,
	).Find(&favorites).Error; err != nil {
		return nil, err
	}
	byNode := make(map[uint]int, len(result))
	for index := range result {
		byNode[result[index].NodeID] = index
	}
	for _, row := range favorites {
		if row.ShortcutTargetNodeID == nil {
			continue
		}
		nodeID := *row.ShortcutTargetNodeID
		if index, exists := byNode[nodeID]; exists {
			result[index].Favorited = true
			continue
		}
		result = append(result, document.NodePreference{NodeID: nodeID, Favorited: true, UpdatedAt: row.UpdatedAt})
	}
	return result, nil
}
