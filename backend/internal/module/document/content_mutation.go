package document

import (
	"context"
	"encoding/json"
)

func (service *Service) UpdateDocument(
	ctx context.Context,
	principal Principal,
	documentID uint,
	expectedRevision uint,
	schemaVersion uint,
	content json.RawMessage,
) (Document, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Document{}, err
	}
	if !state.has(string(ActionContentUpdate)) {
		return Document{}, ErrForbidden
	}
	if documentID == 0 || expectedRevision == 0 {
		return Document{}, ErrInvalidInput
	}
	if schemaVersion != SchemaVersionV1 && schemaVersion != SchemaVersionV2 && schemaVersion != SchemaVersionV3 {
		return Document{}, ErrInvalidInput
	}
	current, err := service.store.GetDocument(ctx, principal.WorkspaceID, documentID)
	if err != nil {
		return Document{}, err
	}
	if current.CollaborationCutoverAt != nil {
		return Document{}, ErrLegacyContentWriteDisabled
	}
	role, err := service.documentRole(ctx, principal, state, documentID)
	if err != nil {
		return Document{}, err
	}
	if role < roleEditor {
		return Document{}, ErrForbidden
	}
	if schemaVersion < current.SchemaVersion {
		return Document{}, ErrInvalidInput
	}
	normalized, hash, plainText, err := NormalizeContentForSchema(content, schemaVersion)
	if err != nil {
		return Document{}, err
	}
	current.Content = normalized
	current.SchemaVersion = schemaVersion
	current.ContentHash = hash
	current.PlainText = plainText
	updated, err := service.store.UpdateDocument(ctx, current, expectedRevision, principal.UserID)
	if err != nil {
		return Document{}, err
	}
	updated.AllowedActions = allowedActions(state, role)
	if err := service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID, Resource: "document",
		ResourceID: updated.ID, Action: "content_updated", Payload: map[string]any{
			"revision": updated.Revision, "schema_version": updated.SchemaVersion,
		},
	}); err != nil {
		return Document{}, err
	}
	return updated, nil
}
