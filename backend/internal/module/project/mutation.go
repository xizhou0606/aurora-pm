package project

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrMutationInvalid  = errors.New("project mutation is invalid")
	ErrMutationNotFound = errors.New("project mutation target not found")
	ErrMutationConflict = errors.New("project mutation conflicts with current state")
)

type MutationOperation string

const (
	DeliveryLabelCreate MutationOperation = "delivery_label.create"
	DeliveryLabelUpdate MutationOperation = "delivery_label.update"
	DeliveryLabelDelete MutationOperation = "delivery_label.delete"
)

type DeliveryLabelMutation struct {
	Operation   MutationOperation
	WorkspaceID uint
	ApplyKey    string
	InputDigest string
	ID          uint
	Name        string
	ClientKey   string
	SystemKey   string
	SortOrder   int
}

type DeliveryLabelMutationResult struct {
	ID           uint   `json:"id"`
	WorkspaceID  uint   `json:"workspace_id,omitempty"`
	ProjectSetID uint   `json:"project_set_id,omitempty"`
	PlanUnitID   uint   `json:"plan_unit_id,omitempty"`
	ClientKey    string `json:"client_key,omitempty"`
	Name         string `json:"name,omitempty"`
	SystemKey    string `json:"system_key,omitempty"`
	SortOrder    int    `json:"sort_order"`
	Deleted      bool   `json:"deleted"`
}

type MutationStore interface {
	ApplyDeliveryLabel(context.Context, DeliveryLabelMutation) (DeliveryLabelMutationResult, error)
}

type MutationService struct{ store MutationStore }

func NewMutationService(store MutationStore) (*MutationService, error) {
	if store == nil {
		return nil, ErrMutationInvalid
	}
	return &MutationService{store: store}, nil
}

func (service *MutationService) ApplyDeliveryLabel(
	ctx context.Context,
	command DeliveryLabelMutation,
) (DeliveryLabelMutationResult, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.ClientKey = strings.TrimSpace(command.ClientKey)
	command.SystemKey = strings.TrimSpace(command.SystemKey)
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.ID == 0 ||
		strings.TrimSpace(command.ApplyKey) == "" || len(command.InputDigest) != 64 {
		return DeliveryLabelMutationResult{}, ErrMutationInvalid
	}
	switch command.Operation {
	case DeliveryLabelCreate:
		if command.Name == "" || len(command.Name) > 100 || command.SortOrder < 0 {
			return DeliveryLabelMutationResult{}, ErrMutationInvalid
		}
	case DeliveryLabelUpdate:
		if command.Name == "" || len(command.Name) > 100 {
			return DeliveryLabelMutationResult{}, ErrMutationInvalid
		}
	case DeliveryLabelDelete:
	default:
		return DeliveryLabelMutationResult{}, ErrMutationInvalid
	}
	return service.store.ApplyDeliveryLabel(ctx, command)
}
