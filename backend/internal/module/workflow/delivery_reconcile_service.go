package workflow

import (
	"context"
)

// DeliveryReconcileService is the domain entrypoint for the delivery
// consumer: it re-evaluates the linked requirement evidence of a plan unit
// and auto-advances satisfied requirement gates.
type DeliveryReconcileService struct {
	store DeliveryReconcileStore
}

// DeliveryReconcileStore persists the gate auto-advance inside one
// transaction per plan unit.
type DeliveryReconcileStore interface {
	ReconcilePlanUnitGates(ctx context.Context, workspaceID, planUnitID uint) (int, error)
}

func NewDeliveryReconcileService(store DeliveryReconcileStore) (*DeliveryReconcileService, error) {
	if store == nil {
		return nil, ErrMutationInvalid
	}
	return &DeliveryReconcileService{store: store}, nil
}

// ReconcilePlanUnitGates returns the number of gate passes committed. A zero
// result is a valid, idempotent outcome: the event may be stale or the gates
// simply still wait for evidence.
func (service *DeliveryReconcileService) ReconcilePlanUnitGates(ctx context.Context, workspaceID, planUnitID uint) (int, error) {
	if service == nil || service.store == nil || workspaceID == 0 || planUnitID == 0 {
		return 0, ErrMutationInvalid
	}
	return service.store.ReconcilePlanUnitGates(ctx, workspaceID, planUnitID)
}
