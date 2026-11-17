package documentlibrary

import (
	"context"
	"strings"
)

type Service struct {
	store    Store
	registry *Registry
}

// ListResources is a metadata source; consumers MUST apply resource access
// before returning any titles or counts to a client.
func (service *Service) ListResources(ctx context.Context, workspaceID uint) ([]Resource, error) {
	if workspaceID == 0 {
		return nil, ErrInvalidInput
	}
	return service.store.List(ctx, workspaceID)
}

func (service *Service) CreateResource(
	ctx context.Context,
	principal Principal,
	typeKey TypeKey,
	payloadRef, name string,
	parentRef *string,
) (Resource, ContentBinding, error) {
	if principal.UserID == 0 || principal.WorkspaceID == 0 || strings.TrimSpace(payloadRef) == "" || strings.TrimSpace(name) == "" {
		return Resource{}, ContentBinding{}, ErrInvalidInput
	}
	if _, err := service.registry.Resolve(typeKey); err != nil {
		return Resource{}, ContentBinding{}, err
	}
	return service.store.Create(ctx, principal, typeKey, payloadRef, name, parentRef)
}

// ReplaceResourceMetadata writes the complete public metadata projection. A
// nil parent means the resource is at the library root; it is not interpreted
// as "leave unchanged".
func (service *Service) ReplaceResourceMetadata(
	ctx context.Context,
	principal Principal,
	resourceID ResourceID,
	expectedRevision uint64,
	name string,
	parentRef *string,
	ownerUserID uint,
) (Resource, error) {
	if principal.UserID == 0 || principal.WorkspaceID == 0 || strings.TrimSpace(string(resourceID)) == "" ||
		expectedRevision == 0 || strings.TrimSpace(name) == "" || ownerUserID == 0 {
		return Resource{}, ErrInvalidInput
	}
	return service.store.ReplaceMetadata(
		ctx, principal, resourceID, expectedRevision, strings.TrimSpace(name), parentRef, ownerUserID,
	)
}

func (service *Service) ArchiveResource(
	ctx context.Context,
	principal Principal,
	resourceID ResourceID,
	expectedRevision uint64,
) error {
	if principal.UserID == 0 || principal.WorkspaceID == 0 || strings.TrimSpace(string(resourceID)) == "" || expectedRevision == 0 {
		return ErrInvalidInput
	}
	return service.store.Archive(ctx, principal, resourceID, expectedRevision)
}

func NewService(store Store, registry *Registry) (*Service, error) {
	if store == nil || registry == nil {
		return nil, ErrInvalidInput
	}
	return &Service{store: store, registry: registry}, nil
}

func (service *Service) ResolveResource(
	ctx context.Context,
	workspaceID uint,
	resourceID ResourceID,
) (Resource, ContentBinding, error) {
	if workspaceID == 0 || strings.TrimSpace(string(resourceID)) == "" {
		return Resource{}, ContentBinding{}, ErrInvalidInput
	}
	resource, binding, err := service.store.Get(ctx, workspaceID, resourceID)
	if err != nil {
		return Resource{}, ContentBinding{}, err
	}
	if err := service.registry.Require(resource.TypeKey, OperationView); err != nil {
		return Resource{}, ContentBinding{}, err
	}
	if binding.ResourceID != resource.ID || binding.WorkspaceID != resource.WorkspaceID || binding.TypeKey != resource.TypeKey {
		return Resource{}, ContentBinding{}, ErrConflict
	}
	return resource, binding, nil
}

func (service *Service) ResolveContentResource(
	ctx context.Context,
	workspaceID uint,
	typeKey TypeKey,
	payloadRef string,
) (Resource, ContentBinding, error) {
	if workspaceID == 0 || strings.TrimSpace(payloadRef) == "" {
		return Resource{}, ContentBinding{}, ErrInvalidInput
	}
	if _, err := service.registry.Resolve(typeKey); err != nil {
		return Resource{}, ContentBinding{}, err
	}
	return service.store.ResolveContent(ctx, workspaceID, typeKey, payloadRef)
}

func (service *Service) ResolveContentResources(
	ctx context.Context,
	workspaceID uint,
	typeKey TypeKey,
	payloadRefs []string,
) (map[string]Resource, error) {
	if workspaceID == 0 || len(payloadRefs) == 0 {
		return nil, ErrInvalidInput
	}
	if _, err := service.registry.Resolve(typeKey); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(payloadRefs))
	for _, payloadRef := range payloadRefs {
		if strings.TrimSpace(payloadRef) == "" {
			return nil, ErrInvalidInput
		}
		seen[payloadRef] = struct{}{}
	}
	resources, err := service.store.ResolveContents(ctx, workspaceID, typeKey, payloadRefs)
	if err != nil {
		return nil, err
	}
	if len(resources) != len(seen) {
		return nil, ErrNotFound
	}
	return resources, nil
}

// RequireTypeOperation validates lifecycle and the registered type capability.
// It deliberately does not authorize a user; callers must additionally pass
// the resource through the resource-access policy gate.
func (service *Service) RequireTypeOperation(
	ctx context.Context,
	workspaceID uint,
	resourceID ResourceID,
	operation Operation,
) (Resource, ContentBinding, error) {
	resource, binding, err := service.ResolveResource(ctx, workspaceID, resourceID)
	if err != nil {
		return Resource{}, ContentBinding{}, err
	}
	if err := service.registry.Require(resource.TypeKey, operation); err != nil {
		return Resource{}, ContentBinding{}, err
	}
	if resource.Lifecycle != "active" || resource.ArchivedAt != nil {
		return Resource{}, ContentBinding{}, ErrNotFound
	}
	return resource, binding, nil
}
