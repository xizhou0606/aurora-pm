package documentlibrary

import "sync"

type Registry struct {
	mu      sync.RWMutex
	entries map[TypeKey]TypeDescriptor
}

func NewRegistry(descriptors ...TypeDescriptor) (*Registry, error) {
	registry := &Registry{entries: map[TypeKey]TypeDescriptor{}}
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func DefaultRegistry() *Registry {
	registry, _ := NewRegistry(
		TypeDescriptor{TypeKey: TypeRichText, Capabilities: capabilities(
			OperationView, OperationEdit, OperationViewAccess, OperationManageAccess,
			OperationViewLinks, OperationManageLinks, OperationExchangeLink, OperationMove, OperationCopy,
			OperationDelete, OperationExport, OperationAssetRead, OperationAssetWrite,
			OperationComment, OperationStructure, OperationCollaborate,
		)},
		TypeDescriptor{TypeKey: TypeBitable, Capabilities: capabilities(
			OperationView, OperationEdit, OperationViewAccess, OperationManageAccess,
			OperationViewLinks, OperationManageLinks, OperationExchangeLink, OperationMove, OperationCopy,
			OperationDelete, OperationExport, OperationAssetRead, OperationAssetWrite,
			OperationComment, OperationStructure, OperationCollaborate, OperationCellLease,
		)},
	)
	return registry
}

func capabilities(operations ...Operation) Capabilities {
	result := Capabilities{Operations: make(map[Operation]bool, len(operations))}
	for _, operation := range operations {
		result.Operations[operation] = true
	}
	return result
}

func (registry *Registry) Register(descriptor TypeDescriptor) error {
	if registry == nil || descriptor.TypeKey == "" || len(descriptor.Capabilities.Operations) == 0 {
		return ErrInvalidInput
	}
	for operation, supported := range descriptor.Capabilities.Operations {
		if operation == "" || !supported {
			return ErrInvalidInput
		}
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.entries[descriptor.TypeKey]; exists {
		return ErrConflict
	}
	registry.entries[descriptor.TypeKey] = cloneDescriptor(descriptor)
	return nil
}

func (registry *Registry) Resolve(typeKey TypeKey) (TypeDescriptor, error) {
	if registry == nil {
		return TypeDescriptor{}, ErrTypeUnavailable
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	descriptor, exists := registry.entries[typeKey]
	if !exists {
		return TypeDescriptor{}, ErrTypeUnavailable
	}
	return cloneDescriptor(descriptor), nil
}

func cloneDescriptor(descriptor TypeDescriptor) TypeDescriptor {
	operations := make(map[Operation]bool, len(descriptor.Capabilities.Operations))
	for operation, supported := range descriptor.Capabilities.Operations {
		operations[operation] = supported
	}
	descriptor.Capabilities.Operations = operations
	return descriptor
}

func (registry *Registry) Require(typeKey TypeKey, operation Operation) error {
	descriptor, err := registry.Resolve(typeKey)
	if err != nil {
		return err
	}
	if !descriptor.Capabilities.Supports(operation) {
		return ErrOperationUnsupported
	}
	return nil
}
