package documentlibrary

import "context"

type Store interface {
	Get(context.Context, uint, ResourceID) (Resource, ContentBinding, error)
	ResolveContent(context.Context, uint, TypeKey, string) (Resource, ContentBinding, error)
	ResolveContents(context.Context, uint, TypeKey, []string) (map[string]Resource, error)
	List(context.Context, uint) ([]Resource, error)
	Create(context.Context, Principal, TypeKey, string, string, *string) (Resource, ContentBinding, error)
	UpdateMetadata(context.Context, Principal, ResourceID, uint64, string, *string, *uint) (Resource, error)
	ReplaceMetadata(context.Context, Principal, ResourceID, uint64, string, *string, uint) (Resource, error)
	Archive(context.Context, Principal, ResourceID, uint64) error
}

type Resolver interface {
	ResolveResource(context.Context, uint, ResourceID) (Resource, ContentBinding, error)
	ResolveContentResource(context.Context, uint, TypeKey, string) (Resource, ContentBinding, error)
	ResolveContentResources(context.Context, uint, TypeKey, []string) (map[string]Resource, error)
	RequireTypeOperation(context.Context, uint, ResourceID, Operation) (Resource, ContentBinding, error)
}
