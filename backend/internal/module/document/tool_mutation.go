package document

import (
	"context"
	"encoding/json"
)

// ToolMutationStore commits the operation receipt and domain changes together.
// The callback receives transaction-scoped domain stores, never database handles.
type ToolMutationStore interface {
	ApplyToolMutation(context.Context, Principal, string, string, func(Store, MutationRecorder) (json.RawMessage, error)) (json.RawMessage, error)
}

func (service *Service) ApplyToolMutation(ctx context.Context, p Principal, key, digest string, apply func(*Service) (json.RawMessage, error)) (json.RawMessage, error) {
	if _, err := service.accessState(ctx, p); err != nil {
		return nil, err
	}
	if key == "" || len(key) > 128 || len(digest) != 64 || apply == nil {
		return nil, ErrInvalidInput
	}
	store, ok := service.store.(ToolMutationStore)
	if !ok {
		return nil, ErrContextActionsUnavailable
	}
	return store.ApplyToolMutation(ctx, p, key, digest, func(txStore Store, recorder MutationRecorder) (json.RawMessage, error) {
		scoped := *service
		scoped.store = txStore
		scoped.effects = recorder
		if comments, ok := txStore.(CommentStore); ok {
			scoped.comments = comments
		}
		return apply(&scoped)
	})
}
