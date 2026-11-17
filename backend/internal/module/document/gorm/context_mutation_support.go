package documentgorm

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func lockCopySource(tx *gorm.DB, command document.CopyDocumentNodeCommand) (Node, Document, error) {
	node, err := lockActiveNode(tx, command.WorkspaceID, command.SourceNode.ID)
	if err != nil {
		return Node{}, Document{}, err
	}
	if node.Type != string(document.NodeDocument) || node.SystemManaged {
		return Node{}, Document{}, document.ErrShortcutTargetInvalid
	}
	var body Document
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND id = ? AND node_id = ? AND archived_at IS NULL",
		command.WorkspaceID, command.Source.ID, node.ID,
	).First(&body).Error; err != nil {
		return Node{}, Document{}, normalizeDocumentContextError(err)
	}
	return node, body, nil
}

func lockShortcutTarget(tx *gorm.DB, workspaceID, nodeID uint) (Node, Document, error) {
	node, err := lockActiveNode(tx, workspaceID, nodeID)
	if err != nil {
		return Node{}, Document{}, err
	}
	if node.Type != string(document.NodeDocument) || node.SystemManaged {
		return Node{}, Document{}, document.ErrShortcutTargetInvalid
	}
	var body Document
	if err := tx.Where("workspace_id = ? AND node_id = ? AND archived_at IS NULL", workspaceID, node.ID).First(&body).Error; err != nil {
		return Node{}, Document{}, normalizeDocumentContextError(err)
	}
	return node, body, nil
}

func lockActiveNode(tx *gorm.DB, workspaceID, nodeID uint) (Node, error) {
	var row Node
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND id = ? AND archived_at IS NULL", workspaceID, nodeID,
	).First(&row).Error; err != nil {
		return Node{}, normalizeNodeContextError(err)
	}
	return row, nil
}

func lockContextDestination(tx *gorm.DB, workspaceID, spaceID uint, parentID *uint) error {
	if parentID == nil {
		var space Space
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND id = ?", workspaceID, spaceID,
		).First(&space).Error; err != nil {
			return normalizeNodeContextError(err)
		}
		return nil
	}
	parent, err := lockActiveNode(tx, workspaceID, *parentID)
	if err != nil {
		return err
	}
	if parent.SpaceID != spaceID {
		return document.ErrNodeNotFound
	}
	if parent.Type != string(document.NodeFolder) {
		return document.ErrParentNotFolder
	}
	if parent.SystemManaged {
		return document.ErrSystemManagedNode
	}
	return nil
}

func nextSortOrderTx(tx *gorm.DB, workspaceID, spaceID uint, parentID *uint) (int64, error) {
	query := tx.Model(&Node{}).Where("workspace_id = ? AND space_id = ? AND archived_at IS NULL", workspaceID, spaceID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	var maximum *int64
	if err := query.Select("MAX(sort_order)").Scan(&maximum).Error; err != nil {
		return 0, err
	}
	if maximum == nil {
		return 1024, nil
	}
	return *maximum + 1024, nil
}

func nextCopyTitle(tx *gorm.DB, workspaceID, spaceID uint, parentID *uint, source string) (string, error) {
	query := tx.Model(&Node{}).Where("workspace_id = ? AND space_id = ? AND archived_at IS NULL", workspaceID, spaceID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	var titles []string
	if err := query.Pluck("title", &titles).Error; err != nil {
		return "", err
	}
	existing := make(map[string]struct{}, len(titles))
	for _, title := range titles {
		existing[title] = struct{}{}
	}
	for ordinal := 1; ordinal < 100000; ordinal++ {
		candidate := copyTitle(source, ordinal)
		if _, exists := existing[candidate]; !exists {
			return candidate, nil
		}
	}
	return "", document.ErrInvalidInput
}

func copyTitle(source string, ordinal int) string {
	suffix := "（副本）"
	if ordinal > 1 {
		suffix = fmt.Sprintf("（副本 %d）", ordinal)
	}
	base := []rune(strings.TrimSpace(source))
	limit := 220 - utf8.RuneCountInString(suffix)
	if len(base) > limit {
		base = base[:limit]
	}
	return strings.TrimSpace(string(base)) + suffix
}

func normalizeNodeContextError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return document.ErrNodeNotFound
	}
	return err
}

func normalizeDocumentContextError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return document.ErrDocumentNotFound
	}
	return err
}
