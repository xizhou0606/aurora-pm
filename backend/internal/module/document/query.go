package document

import (
	"context"
	"slices"
	"sort"
)

func (service *Service) RequireAllScope(ctx context.Context, principal Principal) error {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return err
	}
	if !state.scopeAll {
		return ErrForbidden
	}
	return nil
}

func (service *Service) DefaultSpace(ctx context.Context, principal Principal) (Space, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Space{}, err
	}
	if !state.has(string(ActionNodesView)) {
		return Space{}, ErrForbidden
	}
	space, err := service.store.EnsureDefaultSpace(ctx, principal.WorkspaceID)
	space.CollaborationEnabled = service.canCreateCollaboratively(state)
	return space, err
}

func (service *Service) Tree(ctx context.Context, principal Principal) (Tree, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Tree{}, err
	}
	if !state.has(string(ActionNodesView)) {
		return Tree{}, ErrForbidden
	}
	space, err := service.store.EnsureDefaultSpace(ctx, principal.WorkspaceID)
	if err != nil {
		return Tree{}, err
	}
	nodes, err := service.store.ListNodes(ctx, principal.WorkspaceID, space.ID)
	if err != nil {
		return Tree{}, err
	}
	nodes = visibleSystemNodes(nodes, principal.UserID)
	nodeIDs := make([]uint, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.ID)
	}
	preferences, err := service.store.ListNodePreferences(ctx, principal.WorkspaceID, principal.UserID, nodeIDs)
	if err != nil {
		return Tree{}, err
	}
	nodes = applyNodePreferences(nodes, preferences)
	nodes = preorderNodes(nodes)
	for i := range nodes {
		nodes[i].CanMove = !nodes[i].SystemManaged && state.scopeAll && state.has(string(ActionNodesMove))
	}
	if service.resourceAccess == nil && state.scopeAll {
		return Tree{Space: space, Nodes: nodes}, nil
	}
	documentIDs := make([]uint, 0, len(nodes))
	for _, node := range nodes {
		if node.DocumentID != nil {
			documentIDs = append(documentIDs, *node.DocumentID)
		}
	}
	resourceAccess := map[uint]ResourceAccess{}
	if service.resourceAccess != nil && len(documentIDs) > 0 {
		resourceAccess, err = service.resourceAccess.ResolveDocumentsAccess(ctx, principal, uniqueIDs(documentIDs))
		if err != nil {
			return Tree{}, err
		}
	}
	byID := make(map[uint]Node, len(nodes))
	visible := make(map[uint]struct{})
	for _, node := range nodes {
		byID[node.ID] = node
		if node.SystemKind == NodeSystemFavoritesFolder && node.SystemOwnerUserID != nil && *node.SystemOwnerUserID == principal.UserID {
			visible[node.ID] = struct{}{}
		}
	}
	for _, node := range nodes {
		if node.Type == NodeFolder {
			// Folders have no document resource policy. Keep directory access
			// for all-scope readers and the creator, even before any document
			// is added. Descendant documents still require their own policy.
			if node.SystemManaged || (!state.scopeAll && node.CreatedBy != principal.UserID) {
				continue
			}
		} else if (node.Type != NodeDocument && node.Type != NodeShortcut) || node.DocumentID == nil {
			continue
		} else if service.resourceAccess != nil {
			access := resourceAccess[*node.DocumentID]
			if !access.CanView {
				continue
			}
			node.ResourceID = access.ResourceID
			if node.Type == NodeDocument {
				node.CanMove = !node.SystemManaged && access.CanManage && state.has(string(ActionNodesMove))
			}
			byID[node.ID] = node
		} else {
			role, err := service.documentRole(ctx, principal, state, *node.DocumentID)
			if err != nil {
				return Tree{}, err
			}
			node.CanMove = !node.SystemManaged && role >= roleManager && state.has(string(ActionNodesMove))
			byID[node.ID] = node
			if role < roleViewer {
				continue
			}
		}
		visible[node.ID] = struct{}{}
		for parentID := node.ParentID; parentID != nil; {
			parent, exists := byID[*parentID]
			if !exists {
				break
			}
			visible[parent.ID] = struct{}{}
			parentID = parent.ParentID
		}
	}
	filtered := make([]Node, 0, len(visible))
	for _, node := range nodes {
		if _, ok := visible[node.ID]; ok {
			filtered = append(filtered, byID[node.ID])
		}
	}
	return Tree{Space: space, Nodes: filtered}, nil
}

func visibleSystemNodes(nodes []Node, userID uint) []Node {
	result := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		if node.DirectoryOwnerUserID != nil && *node.DirectoryOwnerUserID != userID {
			continue
		}
		if node.SystemManaged && (node.SystemOwnerUserID == nil || *node.SystemOwnerUserID != userID) {
			continue
		}
		result = append(result, node)
	}
	return result
}

func applyNodePreferences(nodes []Node, preferences []NodePreference) []Node {
	byNode := make(map[uint]NodePreference, len(preferences))
	for _, preference := range preferences {
		byNode[preference.NodeID] = preference
	}
	for index := range nodes {
		preference := byNode[nodes[index].ID]
		nodes[index].PinnedAt = preference.PinnedAt
		nodes[index].Favorited = preference.Favorited
		if nodes[index].SystemKind == NodeSystemFavoriteReference {
			nodes[index].Favorited = true
		}
	}
	return nodes
}

func (service *Service) GetDocument(ctx context.Context, principal Principal, id uint) (Document, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Document{}, err
	}
	if id == 0 || !state.has(string(ActionContentView)) {
		return Document{}, ErrForbidden
	}
	row, err := service.store.GetDocument(ctx, principal.WorkspaceID, id)
	if err != nil {
		return Document{}, err
	}
	role, err := service.documentRole(ctx, principal, state, row.ID)
	if err != nil {
		return Document{}, err
	}
	if role < roleViewer {
		return Document{}, ErrForbidden
	}
	row.AllowedActions = allowedActions(state, role)
	if service.resourceAccess != nil {
		access, accessErr := service.resourceAccess.ResolveDocumentAccess(ctx, principal, row.ID)
		if accessErr != nil {
			return Document{}, accessErr
		}
		row.ResourceID = access.ResourceID
	}
	row, err = service.applyCollaborationProjection(ctx, principal, row)
	if err != nil {
		return Document{}, err
	}
	return row, nil
}

func (service *Service) applyCollaborationProjection(
	ctx context.Context,
	principal Principal,
	row Document,
) (Document, error) {
	if row.CollaborationCutoverAt == nil {
		return row, nil
	}
	if service.collaboration == nil || row.ResourceID == "" {
		return Document{}, ErrContextActionsUnavailable
	}
	projection, projectionErr := service.collaboration.LoadDocumentProjection(ctx, principal, row.ResourceID)
	if projectionErr != nil || projection.Revision == 0 || projection.SchemaVersion == 0 ||
		uint64(uint(projection.Revision)) != projection.Revision {
		return Document{}, ErrContextActionsUnavailable
	}
	row.Content = append(row.Content[:0], projection.Content...)
	row.ContentHash = projection.ContentHash
	row.PlainText = projection.PlainText
	row.SchemaVersion = projection.SchemaVersion
	row.Revision = uint(projection.Revision)
	return row, nil
}

// ExportDocument returns the persisted snapshot only. Export is a separate action
// from reading: object scope (including requirement participation) never grants it.
func (service *Service) ExportDocument(ctx context.Context, principal Principal, id uint) (Document, error) {
	row, err := service.GetDocument(ctx, principal, id)
	if err != nil {
		return Document{}, err
	}
	if !slices.Contains(row.AllowedActions, string(ActionContentExport)) {
		return Document{}, ErrForbidden
	}
	return row, nil
}

func (service *Service) ListBindings(
	ctx context.Context,
	principal Principal,
	documentID uint,
) ([]Binding, error) {
	if _, err := service.GetDocument(ctx, principal, documentID); err != nil {
		return nil, err
	}
	bindings, err := service.store.ListBindings(ctx, principal.WorkspaceID, documentID)
	if bindings == nil && err == nil {
		bindings = []Binding{}
	}
	return bindings, err
}

func preorderNodes(nodes []Node) []Node {
	children := make(map[uint][]Node)
	for _, node := range nodes {
		parentID := uint(0)
		if node.ParentID != nil {
			parentID = *node.ParentID
		}
		children[parentID] = append(children[parentID], node)
	}
	for parentID := range children {
		sort.SliceStable(children[parentID], func(i, j int) bool {
			leftFavoriteFolder := children[parentID][i].SystemKind == NodeSystemFavoritesFolder
			rightFavoriteFolder := children[parentID][j].SystemKind == NodeSystemFavoritesFolder
			if leftFavoriteFolder != rightFavoriteFolder {
				return leftFavoriteFolder
			}
			leftPinned := children[parentID][i].PinnedAt != nil
			rightPinned := children[parentID][j].PinnedAt != nil
			if leftPinned != rightPinned {
				return leftPinned
			}
			if leftPinned && !children[parentID][i].PinnedAt.Equal(*children[parentID][j].PinnedAt) {
				return children[parentID][i].PinnedAt.After(*children[parentID][j].PinnedAt)
			}
			if children[parentID][i].SortOrder != children[parentID][j].SortOrder {
				return children[parentID][i].SortOrder < children[parentID][j].SortOrder
			}
			return children[parentID][i].ID < children[parentID][j].ID
		})
	}
	result := make([]Node, 0, len(nodes))
	visited := make(map[uint]struct{}, len(nodes))
	var visit func(uint)
	visit = func(parentID uint) {
		for _, node := range children[parentID] {
			if _, exists := visited[node.ID]; exists {
				continue
			}
			visited[node.ID] = struct{}{}
			result = append(result, node)
			visit(node.ID)
		}
	}
	visit(0)
	return result
}
