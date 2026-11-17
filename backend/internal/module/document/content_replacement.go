package document

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrReplacementResultUnknown = errors.New("document replacement result unknown")
	ErrEditingInProgress        = errors.New("document editing in progress")
	ErrEditingStateUnavailable  = errors.New("document editing state unavailable")
	ErrReplaceInProgress        = errors.New("document replacement in progress")
)

type ContentReplacement struct {
	ExpectedRevision uint
	SchemaVersion    uint
	Content          json.RawMessage
	OperationID      string
	ConnectionID     string
}
type ContentReplacementResult struct {
	ID          uint   `json:"id"`
	Revision    uint64 `json:"revision"`
	ContentHash string `json:"content_hash"`
}

// The collaboration adapter owns exclusive room access, Yjs conversion and
// durable commit replay. Legacy writes keep the document transaction boundary.
type ContentReplacer interface {
	Replace(context.Context, Principal, Document, ContentReplacement) (ContentReplacementResult, error)
}

func (service *Service) ConfigureContentReplacer(replacer ContentReplacer) {
	service.contentReplacer = replacer
}

// ReplaceToolContent is intentionally exposed only by the Tool adapter. The
// browser's legacy PATCH retains its existing cutover fence.
func (service *Service) ReplaceToolContent(ctx context.Context, p Principal, id uint, input ContentReplacement) (ContentReplacementResult, error) {
	state, err := service.accessState(ctx, p)
	if err != nil {
		return ContentReplacementResult{}, err
	}
	if !state.has(string(ActionContentUpdate)) {
		return ContentReplacementResult{}, ErrForbidden
	}
	current, err := service.GetDocument(ctx, p, id)
	if err != nil {
		return ContentReplacementResult{}, err
	}
	role, err := service.documentRole(ctx, p, state, id)
	if err != nil {
		return ContentReplacementResult{}, err
	}
	if role < roleEditor {
		return ContentReplacementResult{}, ErrForbidden
	}
	if input.ExpectedRevision == 0 || input.OperationID == "" || input.SchemaVersion < current.SchemaVersion {
		return ContentReplacementResult{}, ErrInvalidInput
	}
	normalized, _, _, err := NormalizeContentForSchema(input.Content, input.SchemaVersion)
	if err != nil {
		return ContentReplacementResult{}, err
	}
	input.Content = normalized
	if current.CollaborationCutoverAt == nil {
		updated, err := service.UpdateDocument(ctx, p, id, input.ExpectedRevision, input.SchemaVersion, input.Content)
		return ContentReplacementResult{ID: updated.ID, Revision: uint64(updated.Revision), ContentHash: updated.ContentHash}, err
	}
	if service.contentReplacer == nil {
		return ContentReplacementResult{}, ErrEditingStateUnavailable
	}
	result, err := service.contentReplacer.Replace(ctx, p, current, input)
	if err != nil {
		return ContentReplacementResult{}, err
	}
	err = service.record(ctx, MutationEffect{WorkspaceID: p.WorkspaceID, PrincipalID: p.UserID, Resource: "document", ResourceID: id, Action: "content_updated", Payload: map[string]any{"revision": result.Revision, "source": "tool"}})
	return result, err
}
