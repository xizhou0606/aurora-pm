package task

import (
	"context"
	"errors"
)

var ErrBIReadInvalid = errors.New("task BI read query is invalid")

type BIReadStore interface {
	ListBIFacts(context.Context, uint) ([]Snapshot, []ActivitySnapshot, error)
}

type BIReadService struct{ store BIReadStore }

func NewBIReadService(store BIReadStore) (*BIReadService, error) {
	if store == nil {
		return nil, ErrBIReadInvalid
	}
	return &BIReadService{store: store}, nil
}

func (service *BIReadService) ListBIFacts(ctx context.Context, workspaceID uint) ([]Snapshot, []ActivitySnapshot, error) {
	if workspaceID == 0 {
		return nil, nil, ErrBIReadInvalid
	}
	return service.store.ListBIFacts(ctx, workspaceID)
}
