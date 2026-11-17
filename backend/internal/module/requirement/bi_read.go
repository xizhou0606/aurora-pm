package requirement

import (
	"context"
	"errors"
)

var ErrBIReadInvalid = errors.New("requirement BI read query is invalid")

type BIReadStore interface {
	ListBIFacts(context.Context, uint) ([]Snapshot, error)
}

type BIReadService struct{ store BIReadStore }

func NewBIReadService(store BIReadStore) (*BIReadService, error) {
	if store == nil {
		return nil, ErrBIReadInvalid
	}
	return &BIReadService{store: store}, nil
}

func (service *BIReadService) ListBIFacts(ctx context.Context, workspaceID uint) ([]Snapshot, error) {
	if workspaceID == 0 {
		return nil, ErrBIReadInvalid
	}
	return service.store.ListBIFacts(ctx, workspaceID)
}
