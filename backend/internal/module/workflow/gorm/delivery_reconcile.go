package workflowgorm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeliveryReconcileRepository auto-advances satisfied requirement gates for a
// plan unit (阶段 D). It runs with a system actor: only requirement_gate
// nodes may auto-complete, tokens are consumed by CAS so duplicate events or
// concurrent passes stay idempotent, and every pass reads the latest scope
// and evidence inside its own transaction.
type DeliveryReconcileRepository struct {
	db           *gorm.DB
	dependencies MutationDependencyFactory
}

func NewDeliveryReconcileRepository(db *gorm.DB, dependencies MutationDependencyFactory) (*DeliveryReconcileRepository, error) {
	if db == nil || dependencies == nil {
		return nil, workflow.ErrMutationInvalid
	}
	return &DeliveryReconcileRepository{db: db, dependencies: dependencies}, nil
}

// ReconcilePlanUnitGates re-evaluates every running plan-unit workflow of the
// unit and auto-completes requirement_gate nodes whose guard is satisfied.
// It returns the number of gate passes committed.
func (repository *DeliveryReconcileRepository) ReconcilePlanUnitGates(
	ctx context.Context,
	workspaceID, planUnitID uint,
) (int, error) {
	advanced := 0
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		advanced = 0
		deps, err := repository.dependencies(tx)
		if err != nil {
			return err
		}
		var instances []WorkflowInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND subject_type = ? AND subject_id = ? AND status = ?",
				workspaceID, "plan_unit", planUnitID, "running").
			Find(&instances).Error; err != nil {
			return err
		}
		for _, instance := range instances {
			count, err := reconcileInstanceGates(ctx, tx, deps, instance)
			if err != nil {
				return err
			}
			advanced += count
		}
		return nil
	})
	return advanced, err
}

func reconcileInstanceGates(ctx context.Context, tx *gorm.DB, deps MutationDependencies, instance WorkflowInstance) (int, error) {
	var version WorkflowTemplateVersion
	if err := tx.Where("id = ? AND workspace_id = ? AND template_id = ?",
		instance.TemplateVersionID, instance.WorkspaceID, instance.TemplateID).First(&version).Error; err != nil {
		return 0, err
	}
	definition, err := workflow.ParseDefinition(version.Definition)
	if err != nil {
		return 0, err
	}
	advanced := 0
	for attempt := 0; attempt <= len(definition.Nodes); attempt++ {
		current := currentMutationNodeIDs(instance.CurrentNodeIDs)
		passed := false
		for _, nodeID := range current {
			node, found := definition.Node(nodeID)
			if !found {
				continue
			}
			// Auto-advance nodes: requirement_gate plus stage-driven
			// human task/form nodes (需求阶段驱动).
			guard, isAuto := mutationAutoAdvanceGuard(instance, node)
			if !isAuto {
				continue
			}
			ready, err := mutationGuardReady(ctx, tx, &deps, instance, guard)
			if err != nil {
				return 0, err
			}
			if !ready {
				continue
			}
			tokenClaim := tx.Model(&WorkflowToken{}).
				Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
					instance.WorkspaceID, instance.ID, node.ID, []string{"active", "waiting"}).
				Update("status", "completed")
			if tokenClaim.Error != nil {
				return 0, tokenClaim.Error
			}
			if tokenClaim.RowsAffected == 0 {
				continue
			}
			if err := tx.Create(&WorkflowNodeRun{
				WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
				NodeType: node.Type, Status: "completed", OutputJSON: `{"system_actor":true}`,
				StartedAt: time.Now().UTC(), CompletedAt: nowUTC(),
			}).Error; err != nil {
				return 0, err
			}
			next := definition.Next(node.ID, "complete")
			newCurrent := uniqueMutationNodes(append(removeMutationNode(current, node.ID), next...))
			newCurrent, err = advanceAutomaticMutationNodes(ctx, tx, &deps, instance, definition, newCurrent, nil)
			if err != nil {
				return 0, err
			}
			if err := commitGateAdvance(tx, instance, definition, node.ID, newCurrent); err != nil {
				return 0, err
			}
			instance.CurrentNodeIDs = mustEncodeMutationNodes(newCurrent)
			if len(newCurrent) == 0 {
				instance.Status = "completed"
			}
			advanced++
			passed = true
			break
		}
		if !passed {
			break
		}
	}
	return advanced, nil
}

// commitGateAdvance CAS-updates the instance cursor, syncs the plan unit
// status and writes the system-actor audit trail.
func commitGateAdvance(tx *gorm.DB, instance WorkflowInstance, definition workflow.Definition, gateNodeID string, newCurrent []string) error {
	encoded := mustEncodeMutationNodes(newCurrent)
	status := instance.Status
	var completedAt *time.Time
	now := time.Now().UTC()
	if len(newCurrent) == 0 {
		status, completedAt = "completed", &now
	}
	updated := tx.Model(&WorkflowInstance{}).
		Where("id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ?",
			instance.ID, instance.WorkspaceID, instance.Status, instance.CurrentNodeIDs).
		Updates(map[string]any{"status": status, "current_node_ids": encoded, "completed_at": completedAt})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return workflow.ErrMutationConflict
	}
	if err := syncPlanUnitStatusForReconcile(tx, instance, definition, newCurrent, status); err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"node_id": gateNodeID, "system_actor": true, "current_node_ids": newCurrent})
	return tx.Create(&auditgorm.AuditLog{
		WorkspaceID: instance.WorkspaceID, UserID: 0, StatusCode: 200,
		Resource: "workflow_instance", ResourceID: instance.ID,
		ResourceRef: fmt.Sprintf("workflow_instance:%d", instance.ID),
		Action:      "auto_completed", Target: gateNodeID,
		Detail: string(detail), CreatedAt: now,
	}).Error
}

func mustEncodeMutationNodes(nodeIDs []string) string {
	encoded, _ := json.Marshal(nodeIDs)
	return string(encoded)
}

func nowUTC() *time.Time { value := time.Now().UTC(); return &value }

func syncPlanUnitStatusForReconcile(
	tx *gorm.DB,
	instance WorkflowInstance,
	definition workflow.Definition,
	current []string,
	status string,
) error {
	derived := ""
	if status == "completed" {
		derived = workflow.SubjectStageStatus("plan_unit", workflow.DefinitionNode{}, status)
	} else if len(current) > 0 {
		nodes := make([]workflow.DefinitionNode, 0, len(current))
		for _, nodeID := range current {
			node, found := definition.Node(nodeID)
			if !found {
				continue
			}
			nodes = append(nodes, node)
		}
		derived = workflow.SubjectStatusForCurrent("plan_unit", status, nodes)
	}
	if derived == "" {
		return nil
	}
	return tx.Exec("UPDATE plan_units SET status = ? WHERE workspace_id = ? AND id = ?",
		derived, instance.WorkspaceID, instance.SubjectID).Error
}
