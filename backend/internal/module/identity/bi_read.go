package identity

import (
	"context"
	"errors"
)

var ErrBIReadInvalid = errors.New("identity BI read query is invalid")

type BIWorkspace struct {
	ID   uint
	Name string
}

type BIUser struct {
	ID          uint
	Username    string
	DisplayName string
	AvatarURL   string
}

type BIReadStore interface {
	GetBIWorkspace(context.Context, uint) (BIWorkspace, error)
	FindActiveBIUsersByFactIDs(context.Context, []uint) (map[uint]BIUser, error)
}

type BIReadService struct{ store BIReadStore }

func NewBIReadService(store BIReadStore) (*BIReadService, error) {
	if store == nil {
		return nil, ErrBIReadInvalid
	}
	return &BIReadService{store: store}, nil
}

func (service *BIReadService) GetBIWorkspace(ctx context.Context, workspaceID uint) (BIWorkspace, error) {
	if workspaceID == 0 {
		return BIWorkspace{}, ErrBIReadInvalid
	}
	return service.store.GetBIWorkspace(ctx, workspaceID)
}

func (service *BIReadService) FindActiveBIUsersByFactIDs(ctx context.Context, ids []uint) (map[uint]BIUser, error) {
	for _, id := range ids {
		if id == 0 {
			return nil, ErrBIReadInvalid
		}
	}
	return service.store.FindActiveBIUsersByFactIDs(ctx, ids)
}
