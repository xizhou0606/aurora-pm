package project

import (
	"context"
	"errors"
)

var ErrBIReadInvalid = errors.New("project BI read query is invalid")

type BIContext struct {
	ProjectSets []ProjectSetSnapshot
	PlanUnits   []PlanUnitSnapshot
}

type BIReadStore interface {
	LoadBIContext(context.Context, uint) (BIContext, error)
}

type BIReadService struct{ store BIReadStore }

func NewBIReadService(store BIReadStore) (*BIReadService, error) {
	if store == nil {
		return nil, ErrBIReadInvalid
	}
	return &BIReadService{store: store}, nil
}

func (service *BIReadService) LoadBIContext(ctx context.Context, workspaceID uint) (BIContext, error) {
	if workspaceID == 0 {
		return BIContext{}, ErrBIReadInvalid
	}
	return service.store.LoadBIContext(ctx, workspaceID)
}
