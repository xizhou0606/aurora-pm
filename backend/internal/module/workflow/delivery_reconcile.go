package workflow

import "context"

// DeliveryReconcileEventType is the dedicated outbox event that asks the
// delivery consumer to re-evaluate the linked requirement evidence of a plan
// unit (阶段 D). It is a hint, not a fact: the consumer always reads the
// latest scope and evidence, so duplicate or out-of-order events are safe.
const DeliveryReconcileEventType = "pm.delivery.reconcile_requested"

// DeliveryReconcileWriter records reconcile events inside the caller's
// transaction. HTTP paths write the outbox row directly; the Tool channel
// implements the same semantics through the application composition root.
type DeliveryReconcileWriter interface {
	RecordDeliveryReconcile(ctx context.Context, workspaceID, planUnitID uint, reason string) error
}
