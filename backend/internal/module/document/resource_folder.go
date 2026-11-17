package document

import "context"

// ValidateResourceFolder exposes directory facts to other modules without
// exporting document storage or allowing them to fabricate document nodes.
func (service *Service) ValidateResourceFolder(ctx context.Context, p Principal, id uint) error {
	_, tree, e := service.directoryNode(ctx, p, id, ActionNodesView)
	if e != nil {
		return e
	}
	return directoryParent(tree, &id, 0)
}

func (service *Service) PersonalResourceLocation(ctx context.Context, p Principal, key string, seed *uint) (ResourceDirectoryLocation, error) {
	store, ok := service.store.(ResourceDirectoryStore)
	if !ok {
		return ResourceDirectoryLocation{}, ErrContextActionsUnavailable
	}
	return store.ResourceDirectoryLocation(ctx, p, key, seed)
}
func (service *Service) MovePersonalResource(ctx context.Context, p Principal, key string, parent *uint, revision uint64, operation string) (ResourceDirectoryLocation, error) {
	if parent != nil {
		if err := service.ValidateResourceFolder(ctx, p, *parent); err != nil {
			return ResourceDirectoryLocation{}, err
		}
	}
	store, ok := service.store.(ResourceDirectoryStore)
	if !ok {
		return ResourceDirectoryLocation{}, ErrContextActionsUnavailable
	}
	return store.MoveResourceDirectory(ctx, p, key, parent, revision, operation)
}
