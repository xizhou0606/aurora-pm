package documentread

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

var projectionResourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

type CollaborationProjectionReader struct {
	States collaboration.DocumentStateReader
}

func (reader CollaborationProjectionReader) LoadDocumentProjection(
	ctx context.Context,
	principal document.Principal,
	resourceID string,
) (document.CollaborationProjection, error) {
	if reader.States == nil || principal.WorkspaceID == 0 || !projectionResourceIDPattern.MatchString(resourceID) {
		return document.CollaborationProjection{}, document.ErrContextActionsUnavailable
	}
	state, err := reader.States.LoadDocumentState(ctx, collaboration.Room{
		WorkspaceID: principal.WorkspaceID,
		ResourceID:  resourceID,
	})
	if err != nil || state.DurableRevision == 0 || state.ProjectionHash == "" {
		return document.CollaborationProjection{}, document.ErrContextActionsUnavailable
	}
	normalized, hash, plainText, err := document.NormalizeContentForSchema(
		json.RawMessage(state.Projection), document.SchemaVersionV3,
	)
	if err != nil || hash != state.ProjectionHash {
		return document.CollaborationProjection{}, document.ErrContextActionsUnavailable
	}
	return document.CollaborationProjection{
		Content:       normalized,
		ContentHash:   hash,
		PlainText:     plainText,
		SchemaVersion: document.SchemaVersionV3,
		Revision:      state.DurableRevision,
	}, nil
}
