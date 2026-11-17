package document

import "context"

func (service *Service) PreviewBinding(
	ctx context.Context,
	principal Principal,
	documentID uint,
	requirementID uint,
) (BindingPreview, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return BindingPreview{}, err
	}
	if documentID == 0 || requirementID == 0 {
		return BindingPreview{}, ErrInvalidInput
	}
	if !state.has(string(ActionBindingsUpdate)) || !state.has(string(ActionRequirementView)) {
		return BindingPreview{}, ErrForbidden
	}
	if _, err := service.store.GetDocument(ctx, principal.WorkspaceID, documentID); err != nil {
		return BindingPreview{}, err
	}
	role, roleErr := service.documentRole(ctx, principal, state, documentID)
	if roleErr != nil {
		return BindingPreview{}, roleErr
	}
	if role < roleManager || (service.resourceAccess == nil && !state.scopeAll) {
		return BindingPreview{}, ErrForbidden
	}
	participants, err := service.requirements.ResolveRequirementParticipants(
		ctx,
		principal.WorkspaceID,
		requirementID,
	)
	if err != nil || !participants.Active || participants.WorkspaceID != principal.WorkspaceID {
		return BindingPreview{}, ErrRequirementUnavailable
	}
	editors := uniqueIDs(editorParticipantIDs(participants))
	commenters := uniqueIDs(participants.TesterUserIDs)
	if len(editors) > 0 && len(commenters) > 0 {
		filtered := commenters[:0]
		for _, userID := range commenters {
			if !includesUser(editors, userID) {
				filtered = append(filtered, userID)
			}
		}
		commenters = filtered
	}
	return BindingPreview{
		RequirementID: requirementID, WorkspaceID: participants.WorkspaceID, Version: participants.Version,
		EditorUserIDs: editors, CommenterUserIDs: commenters,
	}, nil
}

func (service *Service) BindRequirement(
	ctx context.Context,
	principal Principal,
	documentID uint,
	requirementID uint,
	confirmed bool,
) (Binding, error) {
	if !confirmed {
		return Binding{}, ErrBindingConfirmationRequired
	}
	preview, err := service.PreviewBinding(ctx, principal, documentID, requirementID)
	if err != nil {
		return Binding{}, err
	}
	binding, err := service.store.CreateBinding(ctx, Binding{
		WorkspaceID: principal.WorkspaceID, DocumentID: documentID, RequirementID: preview.RequirementID,
		Kind: BindingPrimaryProductDoc, CreatedBy: principal.UserID,
	})
	if err != nil {
		return Binding{}, err
	}
	if err := service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID, Resource: "document_binding",
		ResourceID: binding.ID, Action: "created", Payload: map[string]any{
			"document_id": documentID, "requirement_id": requirementID, "kind": binding.Kind,
		},
	}); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func (service *Service) UnbindRequirement(
	ctx context.Context,
	principal Principal,
	documentID uint,
	bindingID uint,
) error {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return err
	}
	if documentID == 0 || bindingID == 0 {
		return ErrInvalidInput
	}
	if !state.has(string(ActionBindingsUpdate)) {
		return ErrForbidden
	}
	role, roleErr := service.documentRole(ctx, principal, state, documentID)
	if roleErr != nil {
		return roleErr
	}
	if role < roleManager || (service.resourceAccess == nil && !state.scopeAll) {
		return ErrForbidden
	}
	if err := service.store.DeleteBinding(ctx, principal.WorkspaceID, documentID, bindingID); err != nil {
		return err
	}
	return service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID, Resource: "document_binding",
		ResourceID: bindingID, Action: "deleted", Payload: map[string]any{"document_id": documentID},
	})
}
