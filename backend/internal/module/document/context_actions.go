package document

import "context"

func (service *Service) CopyNode(
	ctx context.Context,
	principal Principal,
	nodeID uint,
	input CopyNodeInput,
) (CreateNodeResult, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return CreateNodeResult{}, err
	}
	if nodeID == 0 || !state.has(string(ActionNodesCopy)) ||
		!state.has(string(ActionNodesCreate)) || !state.has(string(ActionContentView)) {
		return CreateNodeResult{}, ErrForbidden
	}
	node, err := service.store.GetNode(ctx, principal.WorkspaceID, nodeID)
	if err != nil {
		return CreateNodeResult{}, err
	}
	if node.SystemManaged || node.Type != NodeDocument || node.DocumentID == nil {
		return CreateNodeResult{}, ErrShortcutTargetInvalid
	}
	role, roleErr := service.documentRole(ctx, principal, state, *node.DocumentID)
	if roleErr != nil {
		return CreateNodeResult{}, roleErr
	}
	if role < roleViewer {
		return CreateNodeResult{}, ErrForbidden
	}
	if service.resourceAccess == nil && !state.scopeAll {
		return CreateNodeResult{}, ErrForbidden
	}
	if err := service.validateParent(ctx, principal.WorkspaceID, node.SpaceID, input.ParentID, 0); err != nil {
		return CreateNodeResult{}, err
	}
	body, err := service.store.GetDocument(ctx, principal.WorkspaceID, *node.DocumentID)
	if err != nil {
		return CreateNodeResult{}, err
	}
	result, err := service.store.CopyDocumentNode(ctx, CopyDocumentNodeCommand{
		WorkspaceID: principal.WorkspaceID, ActorUserID: principal.UserID,
		SourceNode: node, Source: body, ParentID: input.ParentID,
		Effect: contextMutationEffect(principal, "document_node", "copied", map[string]any{
			"source_node_id": node.ID, "source_document_id": body.ID,
		}),
	})
	if err != nil {
		return CreateNodeResult{}, err
	}
	if result.Document != nil {
		result.Document.AllowedActions = allowedActions(state, roleManager)
	}
	return result, nil
}

func (service *Service) CreateShortcut(
	ctx context.Context,
	principal Principal,
	nodeID uint,
	input CreateShortcutInput,
) (Node, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Node{}, err
	}
	if nodeID == 0 || !state.has(string(ActionShortcutsCreate)) {
		return Node{}, ErrForbidden
	}
	target, err := service.store.GetNode(ctx, principal.WorkspaceID, nodeID)
	if err != nil {
		return Node{}, err
	}
	if target.SystemManaged || target.Type != NodeDocument || target.DocumentID == nil {
		return Node{}, ErrShortcutTargetInvalid
	}
	role, roleErr := service.documentRole(ctx, principal, state, *target.DocumentID)
	if roleErr != nil {
		return Node{}, roleErr
	}
	if role < roleViewer {
		return Node{}, ErrForbidden
	}
	if service.resourceAccess == nil && !state.scopeAll {
		return Node{}, ErrForbidden
	}
	if err := service.validateParent(ctx, principal.WorkspaceID, target.SpaceID, input.ParentID, 0); err != nil {
		return Node{}, err
	}
	return service.store.CreateShortcut(ctx, CreateShortcutCommand{
		DirectoryOwnerUserID: input.directoryOwner,
		WorkspaceID:          principal.WorkspaceID, ActorUserID: principal.UserID,
		TargetNode: target, ParentID: input.ParentID,
		Effect: contextMutationEffect(principal, "document_node", "shortcut.created", map[string]any{
			"target_node_id": target.ID, "target_document_id": *target.DocumentID,
		}),
	})
}

func (service *Service) UpdateNodePreference(
	ctx context.Context,
	principal Principal,
	nodeID uint,
	input UpdateNodePreferenceInput,
) (NodePreference, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return NodePreference{}, err
	}
	if nodeID == 0 || (input.Pinned == nil && input.Favorited == nil) {
		return NodePreference{}, ErrInvalidInput
	}
	if !state.has(string(ActionPreferencesUpdate)) {
		return NodePreference{}, ErrForbidden
	}
	node, err := service.store.GetNode(ctx, principal.WorkspaceID, nodeID)
	if err != nil {
		return NodePreference{}, err
	}
	if node.SystemManaged {
		return NodePreference{}, ErrSystemManagedNode
	}
	switch node.Type {
	case NodeFolder:
		if !state.scopeAll || input.Favorited != nil {
			return NodePreference{}, ErrForbidden
		}
	case NodeDocument, NodeShortcut:
		if node.DocumentID == nil {
			return NodePreference{}, ErrNodeNotFound
		}
		role, roleErr := service.documentRole(ctx, principal, state, *node.DocumentID)
		if roleErr != nil {
			return NodePreference{}, roleErr
		}
		if role < roleViewer {
			return NodePreference{}, ErrForbidden
		}
		if input.Favorited != nil && node.Type != NodeDocument {
			return NodePreference{}, ErrShortcutTargetInvalid
		}
	default:
		return NodePreference{}, ErrNodeNotFound
	}
	return service.store.UpdateNodePreference(ctx, UpdateNodePreferenceCommand{
		WorkspaceID: principal.WorkspaceID, ActorUserID: principal.UserID, TargetNode: node,
		Pinned: input.Pinned, Favorited: input.Favorited,
		Effect: contextMutationEffect(principal, "document_node_preference", "preference.updated", map[string]any{
			"node_id": node.ID, "pinned": input.Pinned, "favorited": input.Favorited,
		}),
	})
}

func (service *Service) TransferDocumentOwner(
	ctx context.Context,
	principal Principal,
	documentID, ownerUserID uint,
) (Document, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Document{}, err
	}
	if documentID == 0 || ownerUserID == 0 {
		return Document{}, ErrInvalidInput
	}
	if !state.has(string(ActionOwnershipTransfer)) {
		return Document{}, ErrForbidden
	}
	if service.workspaceUsers == nil {
		return Document{}, ErrContextActionsUnavailable
	}
	if _, err := service.store.GetDocument(ctx, principal.WorkspaceID, documentID); err != nil {
		return Document{}, err
	}
	role, roleErr := service.documentRole(ctx, principal, state, documentID)
	if roleErr != nil {
		return Document{}, roleErr
	}
	if role < roleManager {
		return Document{}, ErrForbidden
	}
	if service.resourceAccess == nil && !state.scopeAll {
		return Document{}, ErrForbidden
	}
	if err := service.workspaceUsers.ValidateActiveWorkspaceUser(ctx, principal.WorkspaceID, ownerUserID); err != nil {
		return Document{}, ErrOwnerUserInvalid
	}
	body, err := service.store.TransferDocumentOwner(ctx, TransferDocumentOwnerCommand{
		WorkspaceID: principal.WorkspaceID, ActorUserID: principal.UserID,
		DocumentID: documentID, OwnerUserID: ownerUserID,
		Effect: contextMutationEffect(principal, "document", "owner.transferred", map[string]any{
			"document_id": documentID, "owner_user_id": ownerUserID,
		}),
	})
	if err != nil {
		return Document{}, err
	}
	body.AllowedActions = allowedActions(state, roleManager)
	return body, nil
}

func contextMutationEffect(principal Principal, resource, action string, payload map[string]any) MutationEffect {
	return MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID,
		Resource: resource, Action: action, Payload: payload,
	}
}
