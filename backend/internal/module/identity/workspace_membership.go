package identity

import (
	"context"
	"errors"
)

var ErrWorkspaceMembershipInactive = errors.New("workspace membership is inactive")

type WorkspaceMembershipStore interface {
	HasActiveWorkspaceMembership(context.Context, uint, uint) (bool, error)
}

type WorkspaceMembershipService struct {
	store WorkspaceMembershipStore
}

func NewWorkspaceMembershipService(store WorkspaceMembershipStore) (*WorkspaceMembershipService, error) {
	if store == nil {
		return nil, ErrWorkspaceMembershipInactive
	}
	return &WorkspaceMembershipService{store: store}, nil
}

// VerifyWorkspaceMembership is deliberately shaped like the Tool Runtime
// membership port without importing that module. Workspace participation is a
// visibility boundary shared by HTTP and Tool Runtime callers: ordinary users
// need an enabled workspace_members row, while the canonical system access
// administrator may operate across enabled workspaces.
func (service *WorkspaceMembershipService) VerifyWorkspaceMembership(
	ctx context.Context,
	principalID uint,
	workspaceID uint,
) error {
	if service == nil || service.store == nil || principalID == 0 || workspaceID == 0 {
		return ErrWorkspaceMembershipInactive
	}
	active, err := service.store.HasActiveWorkspaceMembership(ctx, principalID, workspaceID)
	if err != nil {
		return err
	}
	if !active {
		return ErrWorkspaceMembershipInactive
	}
	return nil
}
