package document

import (
	"context"
	"errors"
	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

func (service *Service) PersonalTree(ctx context.Context, p Principal) (Tree, error) {
	tree, err := service.Tree(ctx, p)
	if err != nil {
		return Tree{}, err
	}
	store, ok := service.store.(DirectoryStore)
	if !ok {
		return Tree{}, ErrContextActionsUnavailable
	}
	snapshot, err := store.ReadDirectory(ctx, p, tree.Nodes)
	if err != nil {
		return Tree{}, err
	}
	byID := make(map[uint]Node, len(tree.Nodes))
	for _, node := range tree.Nodes {
		byID[node.ID] = node
	}
	result := make([]Node, 0, len(snapshot.Entries))
	state, err := service.accessState(ctx, p)
	if err != nil {
		return Tree{}, err
	}
	for _, entry := range snapshot.Entries {
		node, visible := byID[entry.NodeID]
		if !visible || entry.Removed {
			continue
		}
		node.SpaceID, node.ParentID, node.SortOrder = snapshot.SpaceID, entry.ParentID, entry.SortOrder
		if node.Type == NodeFolder && entry.Title != "" {
			node.Title = entry.Title
		}
		node.DirectoryRevision = snapshot.Revision
		node.CanMove = !node.SystemManaged && state.has(string(ActionNodesMove))
		result = append(result, node)
	}
	// Revocation/removal must never strand otherwise visible resources behind an absent parent.
	present := make(map[uint]bool, len(result))
	for _, node := range result {
		present[node.ID] = true
	}
	for i := range result {
		if result[i].ParentID != nil && !present[*result[i].ParentID] {
			result[i].ParentID = nil
		}
	}
	tree.Space.ID, tree.Nodes = snapshot.SpaceID, preorderNodes(result)
	return tree, nil
}

func (service *Service) PersonalDefaultSpace(ctx context.Context, p Principal) (Space, error) {
	tree, err := service.PersonalTree(ctx, p)
	return tree.Space, err
}

func (service *Service) directoryNode(ctx context.Context, p Principal, id uint, action accesscatalog.ActionCode) (Node, Tree, error) {
	state, err := service.accessState(ctx, p)
	if err != nil {
		return Node{}, Tree{}, err
	}
	if !state.has(string(action)) {
		return Node{}, Tree{}, ErrForbidden
	}
	tree, err := service.PersonalTree(ctx, p)
	if err != nil {
		return Node{}, Tree{}, err
	}
	for _, node := range tree.Nodes {
		if node.ID == id {
			return node, tree, nil
		}
	}
	return Node{}, Tree{}, ErrNodeNotFound
}

func directoryParent(tree Tree, parent *uint, moving uint) error {
	if parent == nil {
		return nil
	}
	for _, node := range tree.Nodes {
		if node.ID != *parent {
			continue
		}
		if node.Type != NodeFolder {
			return ErrParentNotFolder
		}
		if node.SystemManaged {
			return ErrSystemManagedNode
		}
		if _, cyclic := subtreeIDs(tree.Nodes, moving)[node.ID]; moving != 0 && cyclic {
			return ErrHierarchyCycle
		}
		return nil
	}
	return ErrNodeNotFound
}

func (service *Service) PersonalMoveNode(ctx context.Context, p Principal, id uint, input MoveNodeInput) (Node, error) {
	node, tree, err := service.directoryNode(ctx, p, id, ActionNodesMove)
	if err != nil {
		return Node{}, err
	}
	if node.SystemManaged {
		return Node{}, ErrSystemManagedNode
	}
	if err := directoryParent(tree, input.ParentID, id); err != nil {
		return Node{}, err
	}
	for _, sibling := range []*uint{input.BeforeNodeID, input.AfterNodeID} {
		if sibling == nil {
			continue
		}
		found := false
		for _, candidate := range tree.Nodes {
			if candidate.ID == *sibling && !candidate.SystemManaged {
				found = true
				break
			}
		}
		if !found {
			return Node{}, ErrNodeNotFound
		}
	}
	err = service.store.(DirectoryStore).ChangeDirectory(ctx, p, DirectoryCommand{
		NodeID: id, ParentID: input.ParentID, BeforeNodeID: input.BeforeNodeID,
		AfterNodeID: input.AfterNodeID, ExpectedRevision: input.ExpectedDirectoryRevision,
	})
	if err != nil {
		return Node{}, err
	}
	node, _, err = service.directoryNode(ctx, p, id, ActionNodesView)
	return node, err
}

func (service *Service) InitializePersonalDirectories(ctx context.Context) error {
	store, ok := service.store.(DirectoryStore)
	if !ok {
		return ErrContextActionsUnavailable
	}
	principals, err := store.DirectoryPrincipals(ctx)
	if err != nil {
		return err
	}
	for _, p := range principals {
		if _, err := service.PersonalTree(ctx, p); err != nil && !errors.Is(err, ErrForbidden) {
			return err
		}
	}
	return nil
}
