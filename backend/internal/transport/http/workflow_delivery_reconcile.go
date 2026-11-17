package httpapi

import (
	"encoding/json"
	"strconv"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

// recordDeliveryReconcileTx writes a pm.delivery.reconcile_requested outbox
// event inside the caller's transaction. The delivery consumer reads the
// latest facts when it runs, so the event only carries the affected plan unit
// and a reason for traceability.
func recordDeliveryReconcileTx(tx *gorm.DB, workspaceID, planUnitID uint, reason string) error {
	payload, err := json.Marshal(map[string]any{"plan_unit_id": planUnitID, "reason": reason})
	if err != nil {
		return err
	}
	return tx.Create(&auditgorm.OutboxEvent{
		WorkspaceID:   workspaceID,
		AggregateType: "plan_unit",
		AggregateID:   planUnitID,
		AggregateRef:  "plan_unit:" + strconv.FormatUint(uint64(planUnitID), 10),
		EventType:     workflow.DeliveryReconcileEventType,
		Payload:       string(payload),
		OccurredAt:    time.Now().UTC(),
	}).Error
}

// requirementScheduledPlanUnitTx resolves the plan unit a requirement is
// currently scheduled to, inside the caller's transaction. It returns 0 when
// the requirement is unscheduled.
func requirementScheduledPlanUnitTx(tx *gorm.DB, workspaceID, requirementID uint) uint {
	var row requirementgorm.Requirement
	if tx.Select("version_task_new_id").
		Where("id = ? AND workspace_id = ?", requirementID, workspaceID).
		First(&row).Error != nil {
		return 0
	}
	if row.PlanUnitID == 0 {
		return 0
	}
	return row.PlanUnitID
}
