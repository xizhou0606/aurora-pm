package documentlibrary

import "context"

type ReferenceChecker interface {
	HasResourceReferences(context.Context, uint, string) (bool, error)
}
type ActiveResourceLocker interface {
	LockActiveResource(context.Context, uint, ResourceID) (Resource, ContentBinding, error)
}

func (s *Service) LockActiveResource(ctx context.Context, w uint, id ResourceID) error {
	locker, ok := s.store.(ActiveResourceLocker)
	if !ok {
		return ErrInvalidInput
	}
	_, _, err := locker.LockActiveResource(ctx, w, id)
	return err
}
