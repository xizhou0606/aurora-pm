package bitable

import "context"

// Directory owns presentation locations; resource metadata and access policies stay unchanged.
type ResourceDirectory interface {
	ReadBitableLocation(context.Context, Principal, string, *uint) (*uint, uint64, error)
	MoveBitableLocation(context.Context, Principal, string, *uint, uint64, string) (*uint, uint64, error)
}

func (s *Service) projectDirectoryLocation(ctx context.Context, p Principal, v Resource, binding ResourceBinding) (Resource, error) {
	directory, ok := s.folders.(ResourceDirectory)
	if !ok {
		return v, nil
	}
	parent, revision, err := directory.ReadBitableLocation(ctx, p, string(binding.ResourceID), v.ParentFolderID)
	if err != nil {
		return Resource{}, err
	}
	v.ParentFolderID, v.DirectoryRevision = parent, revision
	return v, nil
}
func (s *Service) moveDirectoryResource(ctx context.Context, p Principal, binding ResourceBinding, in ResourceMutation) (Resource, error) {
	directory, ok := s.folders.(ResourceDirectory)
	if !ok {
		return Resource{}, ErrUnavailable
	}
	if _, _, err := directory.MoveBitableLocation(ctx, p, string(binding.ResourceID), in.ParentFolderID, in.ExpectedRevision, in.OperationID); err != nil {
		return Resource{}, err
	}
	return s.Get(ctx, p, string(binding.ResourceID))
}
