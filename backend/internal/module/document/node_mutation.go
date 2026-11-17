package document

import (
	"context"
	"strings"
)

func (service *Service) CreateNode(
	ctx context.Context,
	principal Principal,
	input CreateNodeInput,
) (CreateNodeResult, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return CreateNodeResult{}, err
	}
	title := strings.TrimSpace(input.Title)
	if !state.has(string(ActionNodesCreate)) || (!state.scopeAll && service.resourceAccess == nil) {
		return CreateNodeResult{}, ErrForbidden
	}
	if title == "" || len(title) > 220 || (input.Type != NodeFolder && input.Type != NodeDocument) {
		return CreateNodeResult{}, ErrInvalidInput
	}
	if input.Collaborative && input.Type != NodeDocument {
		return CreateNodeResult{}, ErrInvalidInput
	}
	if input.Collaborative && !service.canCreateCollaboratively(state) {
		return CreateNodeResult{}, ErrForbidden
	}
	space, err := service.store.EnsureDefaultSpace(ctx, principal.WorkspaceID)
	if err != nil {
		return CreateNodeResult{}, err
	}
	if err := service.validateParent(ctx, principal.WorkspaceID, space.ID, input.ParentID, 0); err != nil {
		return CreateNodeResult{}, err
	}
	sortOrder, err := service.store.NextSortOrder(ctx, principal.WorkspaceID, space.ID, input.ParentID)
	if err != nil {
		return CreateNodeResult{}, err
	}
	node := Node{
		DirectoryOwnerUserID: input.directoryOwner,
		WorkspaceID:          principal.WorkspaceID, SpaceID: space.ID, ParentID: input.ParentID,
		Type: input.Type, Title: title, SortOrder: sortOrder, CreatedBy: principal.UserID, UpdatedBy: principal.UserID,
	}
	var body *Document
	if input.Type == NodeDocument {
		content, hash, plainText, normalizeErr := NormalizeContent(DefaultContent())
		if normalizeErr != nil {
			return CreateNodeResult{}, normalizeErr
		}
		body = &Document{
			WorkspaceID: principal.WorkspaceID, Content: content, SchemaVersion: SchemaVersionV1,
			Revision: 1, ContentHash: hash, PlainText: plainText, OwnerUserID: principal.UserID,
			CreatedBy: principal.UserID, UpdatedBy: principal.UserID,
		}
	}
	create := service.store.CreateNode
	if input.Collaborative {
		creator, ok := service.store.(CollaborativeNodeCreator)
		if !ok {
			return CreateNodeResult{}, ErrContextActionsUnavailable
		}
		create = creator.CreateCollaborativeNode
	}
	createdNode, createdDocument, err := create(ctx, node, body)
	if err != nil {
		return CreateNodeResult{}, err
	}
	if createdDocument != nil {
		createdDocument.AllowedActions = allowedActions(state, roleManager)
	}
	if err := service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID, Resource: "document_node",
		ResourceID: createdNode.ID, Action: "created", Payload: map[string]any{"node_type": createdNode.Type},
	}); err != nil {
		return CreateNodeResult{}, err
	}
	return CreateNodeResult{Node: createdNode, Document: createdDocument}, nil
}

func (service *Service) RenameNode(
	ctx context.Context,
	principal Principal,
	nodeID uint,
	title string,
) (Node, error) {
	state, err := service.accessState(ctx, principal)
	title = strings.TrimSpace(title)
	if err != nil {
		return Node{}, err
	}
	if !state.has(string(ActionNodesUpdate)) {
		return Node{}, ErrForbidden
	}
	if nodeID == 0 || title == "" || len(title) > 220 {
		return Node{}, ErrInvalidInput
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
	if node.Type == NodeShortcut {
		return Node{}, ErrShortcutTargetInvalid
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
	node.Title = title
	node.UpdatedBy = principal.UserID
	node, err = service.store.UpdateNode(ctx, node)
	if err != nil {
		return Node{}, err
	}
	return node, service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID, Resource: "document_node",
		ResourceID: node.ID, Action: "renamed", Payload: map[string]any{"title": title},
	})
}
