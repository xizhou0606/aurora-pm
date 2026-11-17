package task

import (
	"context"
	"errors"
)

var ErrCreatorPolicyInvalid = errors.New("task creator policy is invalid")

type CreatorPolicyStore interface {
	CreatorIDs(context.Context, uint, []uint) (map[uint]uint, error)
}

type CreatorPolicyService struct{ store CreatorPolicyStore }

func NewCreatorPolicyService(store CreatorPolicyStore) (*CreatorPolicyService, error) {
	if store == nil {
		return nil, ErrCreatorPolicyInvalid
	}
	return &CreatorPolicyService{store: store}, nil
}

// AllCreatedBy requires every selected task to have an immutable created or
// cloned activity whose first actor is the current PM user. Missing creator
// evidence fails closed.
func (service *CreatorPolicyService) AllCreatedBy(
	ctx context.Context,
	workspaceID, userID uint,
	taskIDs []uint,
) (bool, error) {
	if service == nil || service.store == nil || workspaceID == 0 || userID == 0 || len(taskIDs) == 0 || len(taskIDs) > 500 {
		return false, ErrCreatorPolicyInvalid
	}
	seen := make(map[uint]struct{}, len(taskIDs))
	canonical := make([]uint, 0, len(taskIDs))
	for _, id := range taskIDs {
		if id == 0 {
			return false, ErrCreatorPolicyInvalid
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		canonical = append(canonical, id)
	}
	allowed, err := service.AllowedCreatedBy(ctx, workspaceID, userID, canonical)
	if err != nil {
		return false, err
	}
	for _, id := range canonical {
		if !allowed[id] {
			return false, nil
		}
	}
	return true, nil
}

// AllowedCreatedBy returns one fail-closed decision per requested task. The
// creator is derived only from the earliest immutable created/cloned activity.
func (service *CreatorPolicyService) AllowedCreatedBy(
	ctx context.Context,
	workspaceID, userID uint,
	taskIDs []uint,
) (map[uint]bool, error) {
	if service == nil || service.store == nil || workspaceID == 0 || userID == 0 || len(taskIDs) == 0 || len(taskIDs) > 500 {
		return nil, ErrCreatorPolicyInvalid
	}
	seen := make(map[uint]struct{}, len(taskIDs))
	canonical := make([]uint, 0, len(taskIDs))
	for _, id := range taskIDs {
		if id == 0 {
			return nil, ErrCreatorPolicyInvalid
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		canonical = append(canonical, id)
	}
	creators, err := service.store.CreatorIDs(ctx, workspaceID, canonical)
	if err != nil {
		return nil, err
	}
	allowed := make(map[uint]bool, len(canonical))
	for _, id := range canonical {
		allowed[id] = creators[id] != 0 && creators[id] == userID
	}
	return allowed, nil
}
