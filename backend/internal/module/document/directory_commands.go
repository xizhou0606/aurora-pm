package document

import (
	"context"
	"strings"
)

func (service *Service) directoryTransaction(ctx context.Context, apply func(*Service) error) error {
	store, ok := service.store.(DirectoryStore)
	if !ok {
		return ErrContextActionsUnavailable
	}
	return store.InDirectoryTransaction(ctx, func(txStore Store, recorder MutationRecorder) error {
		scoped := *service
		scoped.store = txStore
		scoped.effects = recorder
		return apply(&scoped)
	})
}

func (service *Service) PersonalCreateNode(ctx context.Context, p Principal, input CreateNodeInput) (CreateNodeResult, error) {
	var result CreateNodeResult
	err := service.directoryTransaction(ctx, func(scoped *Service) error {
		tree, err := scoped.PersonalTree(ctx, p)
		if err != nil {
			return err
		}
		if err := directoryParent(tree, input.ParentID, 0); err != nil {
			return err
		}
		parent := input.ParentID
		input.ParentID = nil
		if input.Type == NodeFolder {
			input.directoryOwner = &p.UserID
		}
		result, err = scoped.CreateNode(ctx, p, input)
		if err != nil {
			return err
		}
		// Membership and creation commit together; no file is left behind after a failed placement.
		result.Node, err = scoped.personalPlaceCreated(ctx, p, result.Node, parent)
		return err
	})
	return result, err
}

// The caller has authorized creation and validated the destination. Seed the
// returned resource directly: access readers outside this transaction cannot see
// its uncommitted resource record yet.
func (service *Service) personalPlaceCreated(ctx context.Context, p Principal, node Node, parent *uint) (Node, error) {
	store := service.store.(DirectoryStore)
	if _, err := store.ReadDirectory(ctx, p, []Node{node}); err != nil {
		return Node{}, err
	}
	if err := store.ChangeDirectory(ctx, p, DirectoryCommand{NodeID: node.ID, ParentID: parent}); err != nil {
		return Node{}, err
	}
	snapshot, err := store.ReadDirectory(ctx, p, nil)
	if err != nil {
		return Node{}, err
	}
	for _, entry := range snapshot.Entries {
		if entry.NodeID == node.ID {
			node.SpaceID, node.ParentID, node.SortOrder = snapshot.SpaceID, entry.ParentID, entry.SortOrder
			node.DirectoryRevision = snapshot.Revision
			return node, nil
		}
	}
	return Node{}, ErrNodeNotFound
}
func (service *Service) PersonalRenameNode(ctx context.Context, p Principal, id uint, title string) (Node, error) {
	node, _, err := service.directoryNode(ctx, p, id, ActionNodesUpdate)
	if err != nil {
		return Node{}, err
	}
	if node.SystemManaged {
		return Node{}, ErrSystemManagedNode
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 220 {
		return Node{}, ErrInvalidInput
	}
	if node.Type != NodeFolder {
		return service.RenameNode(ctx, p, id, title)
	}
	if err := service.store.(DirectoryStore).ChangeDirectory(ctx, p, DirectoryCommand{NodeID: id, Title: &title}); err != nil {
		return Node{}, err
	}
	node, _, err = service.directoryNode(ctx, p, id, ActionNodesView)
	return node, err
}
func (service *Service) PersonalArchiveImpact(ctx context.Context, p Principal, id uint) (ArchiveImpact, error) {
	node, tree, err := service.directoryNode(ctx, p, id, ActionNodesArchive)
	if err != nil {
		return ArchiveImpact{}, err
	}
	if node.SystemManaged {
		return ArchiveImpact{}, ErrSystemManagedNode
	}
	ids := subtreeIDs(tree.Nodes, id)
	count := int64(0)
	for _, item := range tree.Nodes {
		if _, ok := ids[item.ID]; ok && item.Type != NodeFolder {
			count++
		}
	}
	return ArchiveImpact{NodeID: id, DescendantCount: int64(len(ids) - 1), DocumentCount: count, RequiresConfirmation: len(ids) > 1}, nil
}
func (service *Service) PersonalArchiveNode(ctx context.Context, p Principal, id uint, confirmation ArchiveConfirmation) error {
	return service.directoryTransaction(ctx, func(scoped *Service) error {
		impact, err := scoped.PersonalArchiveImpact(ctx, p, id)
		if err != nil {
			return err
		}
		if impact.RequiresConfirmation && !confirmation.ConfirmNonEmpty {
			return ErrArchiveConfirmationRequired
		}
		if impact.RequiresConfirmation && (confirmation.ExpectedDescendantCount == nil || confirmation.ExpectedDocumentCount == nil || *confirmation.ExpectedDescendantCount != impact.DescendantCount || *confirmation.ExpectedDocumentCount != impact.DocumentCount) {
			return ArchiveImpactChangedError{Latest: impact}
		}
		return scoped.store.(DirectoryStore).ChangeDirectory(ctx, p, DirectoryCommand{NodeID: id, Remove: true})
	})
}
func (service *Service) PersonalCopyNode(ctx context.Context, p Principal, id uint, input CopyNodeInput) (CreateNodeResult, error) {
	var result CreateNodeResult
	err := service.directoryTransaction(ctx, func(scoped *Service) error {
		_, tree, err := scoped.directoryNode(ctx, p, id, ActionNodesCopy)
		if err != nil {
			return err
		}
		if err := directoryParent(tree, input.ParentID, 0); err != nil {
			return err
		}
		result, err = scoped.CopyNode(ctx, p, id, CopyNodeInput{})
		if err != nil {
			return err
		}
		result.Node, err = scoped.personalPlaceCreated(ctx, p, result.Node, input.ParentID)
		return err
	})
	return result, err
}
func (service *Service) PersonalCreateShortcut(ctx context.Context, p Principal, id uint, input CreateShortcutInput) (Node, error) {
	var result Node
	err := service.directoryTransaction(ctx, func(scoped *Service) error {
		_, tree, err := scoped.directoryNode(ctx, p, id, ActionShortcutsCreate)
		if err != nil {
			return err
		}
		if err := directoryParent(tree, input.ParentID, 0); err != nil {
			return err
		}
		result, err = scoped.CreateShortcut(ctx, p, id, CreateShortcutInput{directoryOwner: &p.UserID})
		if err != nil {
			return err
		}
		result, err = scoped.personalPlaceCreated(ctx, p, result, input.ParentID)
		return err
	})
	return result, err
}
