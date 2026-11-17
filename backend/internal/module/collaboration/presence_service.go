package collaboration

import (
	"context"
	"time"
)

type PresenceService struct {
	store  PresenceStore
	access AccessEvaluator
}

func NewPresenceService(store PresenceStore, access AccessEvaluator) (*PresenceService, error) {
	if store == nil || access == nil {
		return nil, ErrInvalidInput
	}
	return &PresenceService{store: store, access: access}, nil
}

func (service *PresenceService) Heartbeat(
	ctx context.Context,
	principal Principal,
	room Room,
	state []byte,
) (Presence, error) {
	if service == nil || principal.WorkspaceID == 0 || principal.WorkspaceID != room.WorkspaceID {
		return Presence{}, ErrInvalidInput
	}
	decision, err := service.access.EvaluateCollaborationAccess(ctx, principal, room)
	if err != nil {
		return Presence{}, err
	}
	// Viewers receive the server-owned presence projection from the event
	// stream, but cannot publish mutable presence/selection messages.
	if !decision.CanEdit || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(time.Now()) {
		return Presence{}, ErrForbidden
	}
	return service.store.Heartbeat(ctx, principal, room, state)
}

func (service *PresenceService) Leave(
	ctx context.Context,
	principal Principal,
	room Room,
) error {
	if service == nil || principal.WorkspaceID == 0 || principal.WorkspaceID != room.WorkspaceID ||
		principal.ConnectionID == "" {
		return ErrInvalidInput
	}
	// Cleanup remains available after an access revocation. The store keys the
	// deletion by the authenticated connection and canonical room.
	return service.store.Leave(ctx, principal, room)
}
