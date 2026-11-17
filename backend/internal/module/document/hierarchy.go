package document

import "context"

func (service *Service) MoveNode(
	ctx context.Context,
	principal Principal,
	nodeID uint,
	input MoveNodeInput,
) (Node, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Node{}, err
	}
	if nodeID == 0 || !state.has(string(ActionNodesMove)) {
		return Node{}, ErrForbidden
	}
	node, err := service.store.GetNode(ctx, principal.WorkspaceID, nodeID)
	if err != nil {
		return Node{}, err
	}
	if node.DirectoryOwnerUserID != nil && *node.DirectoryOwnerUserID != principal.UserID {
		return Node{}, ErrForbidden
	}
	if node.ArchivedAt != nil {
		return Node{}, ErrNodeNotFound
	}
	if node.SystemManaged {
		return Node{}, ErrSystemManagedNode
	}
	if node.Type == NodeDocument {
		if node.DocumentID == nil {
			return Node{}, ErrNodeNotFound
		}
		role, roleErr := service.documentRole(ctx, principal, state, *node.DocumentID)
		if roleErr != nil {
			return Node{}, roleErr
		}
		if role < roleManager {
			return Node{}, ErrForbidden
		}
	} else if !state.scopeAll {
		return Node{}, ErrForbidden
	}
	if err := service.validateParent(ctx, principal.WorkspaceID, node.SpaceID, input.ParentID, node.ID); err != nil {
		return Node{}, err
	}
	if input.SortOrder == nil {
		value, err := service.store.NextSortOrder(ctx, principal.WorkspaceID, node.SpaceID, input.ParentID)
		if err != nil {
			return Node{}, err
		}
		input.SortOrder = &value
	}
	if *input.SortOrder < 0 {
		return Node{}, ErrInvalidInput
	}
	node.ParentID = input.ParentID
	node.SortOrder = *input.SortOrder
	node.UpdatedBy = principal.UserID
	node, err = service.store.UpdateNode(ctx, node)
	if err != nil {
		return Node{}, err
	}
	return node, service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID, Resource: "document_node",
		ResourceID: node.ID, Action: "moved", Payload: map[string]any{"parent_id": node.ParentID, "sort_order": node.SortOrder},
	})
}

func (service *Service) ArchiveImpact(
	ctx context.Context,
	principal Principal,
	nodeID uint,
) (ArchiveImpact, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return ArchiveImpact{}, err
	}
	if nodeID == 0 || !state.has(string(ActionNodesArchive)) {
		return ArchiveImpact{}, ErrForbidden
	}
	node, err := service.store.GetNode(ctx, principal.WorkspaceID, nodeID)
	if err != nil {
		return ArchiveImpact{}, err
	}
	if node.DirectoryOwnerUserID != nil && *node.DirectoryOwnerUserID != principal.UserID {
		return ArchiveImpact{}, ErrForbidden
	}
	if node.SystemManaged {
		return ArchiveImpact{}, ErrSystemManagedNode
	}
	nodes, err := service.store.ListNodes(ctx, principal.WorkspaceID, node.SpaceID)
	if err != nil {
		return ArchiveImpact{}, err
	}
	subtree := subtreeIDs(nodes, node.ID)
	documents := int64(0)
	for _, candidate := range nodes {
		if _, ok := subtree[candidate.ID]; ok && candidate.Type == NodeDocument {
			if candidate.DocumentID == nil {
				return ArchiveImpact{}, ErrNodeNotFound
			}
			role, roleErr := service.documentRole(ctx, principal, state, *candidate.DocumentID)
			if roleErr != nil {
				return ArchiveImpact{}, roleErr
			}
			if role < roleManager {
				return ArchiveImpact{}, ErrForbidden
			}
			documents++
		}
	}
	if documents == 0 && !state.scopeAll {
		return ArchiveImpact{}, ErrForbidden
	}
	descendants := int64(len(subtree) - 1)
	return ArchiveImpact{
		NodeID: node.ID, DescendantCount: descendants, DocumentCount: documents,
		RequiresConfirmation: node.Type == NodeFolder && descendants > 0,
	}, nil
}

func (service *Service) ArchiveNode(
	ctx context.Context,
	principal Principal,
	nodeID uint,
	confirmation ArchiveConfirmation,
) error {
	impact, err := service.ArchiveImpact(ctx, principal, nodeID)
	if err != nil {
		return err
	}
	if impact.RequiresConfirmation && !confirmation.ConfirmNonEmpty {
		return ErrArchiveConfirmationRequired
	}
	if impact.RequiresConfirmation && (confirmation.ExpectedDescendantCount == nil ||
		confirmation.ExpectedDocumentCount == nil ||
		*confirmation.ExpectedDescendantCount != impact.DescendantCount ||
		*confirmation.ExpectedDocumentCount != impact.DocumentCount) {
		return ArchiveImpactChangedError{Latest: impact}
	}
	node, err := service.store.GetNode(ctx, principal.WorkspaceID, nodeID)
	if err != nil {
		return err
	}
	nodes, err := service.store.ListNodes(ctx, principal.WorkspaceID, node.SpaceID)
	if err != nil {
		return err
	}
	idsByValue := subtreeIDs(nodes, node.ID)
	ids := make([]uint, 0, len(idsByValue))
	for id := range idsByValue {
		ids = append(ids, id)
	}
	if err := service.store.ArchiveNodes(ctx, principal.WorkspaceID, ids, principal.UserID); err != nil {
		return err
	}
	return service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID, Resource: "document_node",
		ResourceID: nodeID, Action: "archived", Payload: map[string]any{
			"descendant_count": impact.DescendantCount, "document_count": impact.DocumentCount,
		},
	})
}

func (service *Service) validateParent(
	ctx context.Context,
	workspaceID uint,
	spaceID uint,
	parentID *uint,
	movingNodeID uint,
) error {
	if parentID == nil {
		return nil
	}
	if *parentID == 0 || *parentID == movingNodeID {
		return ErrHierarchyCycle
	}
	parent, err := service.store.GetNode(ctx, workspaceID, *parentID)
	if err != nil {
		return err
	}
	if parent.SpaceID != spaceID || parent.ArchivedAt != nil {
		return ErrNodeNotFound
	}
	if parent.DirectoryOwnerUserID != nil {
		return ErrForbidden
	}
	if parent.Type != NodeFolder {
		return ErrParentNotFolder
	}
	if parent.SystemManaged {
		return ErrSystemManagedNode
	}
	if movingNodeID == 0 {
		return nil
	}
	nodes, err := service.store.ListNodes(ctx, workspaceID, spaceID)
	if err != nil {
		return err
	}
	if _, descendant := subtreeIDs(nodes, movingNodeID)[*parentID]; descendant {
		return ErrHierarchyCycle
	}
	return nil
}

func subtreeIDs(nodes []Node, rootID uint) map[uint]struct{} {
	result := map[uint]struct{}{rootID: {}}
	for changed := true; changed; {
		changed = false
		for _, node := range nodes {
			if node.ParentID == nil {
				continue
			}
			if _, parentIncluded := result[*node.ParentID]; !parentIncluded {
				continue
			}
			if _, alreadyIncluded := result[node.ID]; alreadyIncluded {
				continue
			}
			result[node.ID] = struct{}{}
			changed = true
		}
	}
	return result
}
