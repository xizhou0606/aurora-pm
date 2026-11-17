package ai

import (
	"context"
	"errors"
	"strings"
)

var ErrRunReadInvalid = errors.New("AI run read query is invalid")

type RunStatus struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

type RunReadStore interface {
	ListRunStatuses(context.Context, uint, []uint) ([]RunStatus, error)
}

type RunReadService struct{ store RunReadStore }

func NewRunReadService(store RunReadStore) (*RunReadService, error) {
	if store == nil {
		return nil, ErrRunReadInvalid
	}
	return &RunReadService{store: store}, nil
}

func (service *RunReadService) ListRunStatuses(ctx context.Context, workspaceID uint, ids []uint) ([]RunStatus, error) {
	if workspaceID == 0 {
		return nil, ErrRunReadInvalid
	}
	unique := make([]uint, 0, len(ids))
	seen := map[uint]struct{}{}
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return []RunStatus{}, nil
	}
	items, err := service.store.ListRunStatuses(ctx, workspaceID, unique)
	if err == nil {
		for index := range items {
			items[index].Status = strings.ToLower(strings.TrimSpace(items[index].Status))
		}
	}
	return items, err
}
