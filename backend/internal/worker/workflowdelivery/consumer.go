// Package workflowdelivery consumes pm.delivery.reconcile_requested outbox
// events and re-evaluates the linked requirement evidence of the affected
// plan units (阶段 D 交付重算). Events are hints: the consumer always reads
// the latest scope and evidence, so duplicate or out-of-order delivery is
// safe and every gate pass is CAS-idempotent.
package workflowdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

const (
	consumerBatchSize   = 20
	consumerPollEvery   = 2 * time.Second
	consumerMaxAttempts = 12
)

// planRequirementScopeReader adapts the requirement module scope query for
// the reconcile transaction.
type planRequirementScopeReader struct{ db *gorm.DB }

func (reader planRequirementScopeReader) PlanUnitRequirementRefs(
	ctx context.Context,
	workspaceID, planUnitID uint,
) ([]workflow.PlanRequirementRef, error) {
	raw, err := requirementgorm.PlanUnitRequirementWorkflowRefs(reader.db.WithContext(ctx), workspaceID, planUnitID)
	if err != nil {
		return nil, err
	}
	refs := make([]workflow.PlanRequirementRef, 0, len(raw))
	for _, ref := range raw {
		item := workflow.PlanRequirementRef{RequirementID: ref.RequirementID, Status: ref.Status}
		if ref.WorkflowInstanceID != nil {
			item.WorkflowInstanceID = *ref.WorkflowInstanceID
		}
		if ref.TemplateVersionID != nil {
			item.TemplateVersionID = *ref.TemplateVersionID
		}
		refs = append(refs, item)
	}
	return refs, nil
}

// Consumer is the dedicated pm.delivery.* outbox consumer. It never competes
// with the notification publisher: the claim query matches the exact event
// type only.
type Consumer struct {
	db        *gorm.DB
	reconcile *workflow.DeliveryReconcileService
	workerID  string
	now       func() time.Time
}

// NewConsumer assembles the consumer with its own transaction-scoped
// requirement scope reader.
func NewConsumer(db *gorm.DB) (*Consumer, error) {
	reconcileRepo, err := workflowgorm.NewDeliveryReconcileRepository(db, func(tx *gorm.DB) (workflowgorm.MutationDependencies, error) {
		return workflowgorm.MutationDependencies{
			PlanRequirements: planRequirementScopeReader{db: tx},
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("delivery reconcile repository bootstrap error: %w", err)
	}
	reconcile, err := workflow.NewDeliveryReconcileService(reconcileRepo)
	if err != nil {
		return nil, fmt.Errorf("delivery reconcile service bootstrap error: %w", err)
	}
	hostname, _ := os.Hostname()
	return &Consumer{
		db: db, reconcile: reconcile,
		workerID: fmt.Sprintf("%s-%d", hostname, os.Getpid()),
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

func (consumer *Consumer) Run(ctx context.Context) {
	consumer.consumeAndLog(ctx)
	ticker := time.NewTicker(consumerPollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			consumer.consumeAndLog(ctx)
		}
	}
}

func (consumer *Consumer) consumeAndLog(ctx context.Context) {
	if _, err := consumer.ConsumeOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("delivery reconcile consume failed: %v", err)
	}
}

// ConsumeOnce claims and processes up to one batch of reconcile events.
func (consumer *Consumer) ConsumeOnce(ctx context.Context) (int, error) {
	processed := 0
	var failures []error
	for processed < consumerBatchSize {
		event, found, err := consumer.claimNext(ctx)
		if err != nil {
			return processed, err
		}
		if !found {
			break
		}
		processed++
		if err := consumer.dispatch(ctx, event); err != nil {
			if markErr := consumer.markFailed(event, err); markErr != nil {
				failures = append(failures, markErr)
			}
			failures = append(failures, fmt.Errorf("delivery event %d: %w", event.ID, err))
			continue
		}
		if err := consumer.markPublished(event); err != nil {
			failures = append(failures, err)
		}
	}
	return processed, errors.Join(failures...)
}

func (consumer *Consumer) dispatch(ctx context.Context, event auditgorm.OutboxEvent) error {
	var payload struct {
		PlanUnitID uint   `json:"plan_unit_id"`
		Reason     string `json:"reason"`
	}
	if json.Unmarshal([]byte(event.Payload), &payload) != nil || payload.PlanUnitID == 0 {
		// Malformed payload: record and drop, retries cannot fix it.
		return fmt.Errorf("delivery reconcile payload is invalid")
	}
	if event.Attempts > consumerMaxAttempts {
		return fmt.Errorf("delivery reconcile payload dropped after %d attempts", event.Attempts)
	}
	_, err := consumer.reconcile.ReconcilePlanUnitGates(ctx, event.WorkspaceID, payload.PlanUnitID)
	return err
}

func (consumer *Consumer) claimNext(ctx context.Context) (auditgorm.OutboxEvent, bool, error) {
	now := consumer.now()
	staleBefore := now.Add(-15 * time.Minute)
	var event auditgorm.OutboxEvent
	err := consumer.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("published_at IS NULL").
			Where("event_type = ?", workflow.DeliveryReconcileEventType).
			Where("(next_attempt_at IS NULL OR next_attempt_at <= ?)", now).
			Where("(claimed_at IS NULL OR claimed_at < ?)", staleBefore).
			Order("id ASC")
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		found := query.Limit(1).Find(&event)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		result := tx.Model(&auditgorm.OutboxEvent{}).
			Where("id = ? AND published_at IS NULL AND (claimed_at IS NULL OR claimed_at < ?)", event.ID, staleBefore).
			Updates(map[string]any{
				"claimed_at": now, "claimed_by": consumer.workerID, "next_attempt_at": nil,
				"attempts": gorm.Expr("attempts + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auditgorm.OutboxEvent{}, false, nil
	}
	if err != nil {
		return auditgorm.OutboxEvent{}, false, err
	}
	event.Attempts++
	event.ClaimedAt = &now
	event.ClaimedBy = consumer.workerID
	return event, true, nil
}

func (consumer *Consumer) markPublished(event auditgorm.OutboxEvent) error {
	result := consumer.db.Model(&auditgorm.OutboxEvent{}).
		Where("id = ? AND claimed_by = ? AND published_at IS NULL", event.ID, consumer.workerID).
		Updates(map[string]any{
			"published_at": consumer.now(), "claimed_at": nil, "claimed_by": "",
			"next_attempt_at": nil, "last_error": "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("delivery event %d publish claim was lost", event.ID)
	}
	return nil
}

func (consumer *Consumer) markFailed(event auditgorm.OutboxEvent, consumeErr error) error {
	nextAttempt := consumer.now().Add(retryDelay(event.Attempts))
	result := consumer.db.Model(&auditgorm.OutboxEvent{}).
		Where("id = ? AND claimed_by = ? AND published_at IS NULL", event.ID, consumer.workerID).
		Updates(map[string]any{
			"claimed_at": nil, "claimed_by": "", "next_attempt_at": nextAttempt,
			"last_error": truncateDeliveryError(consumeErr.Error()),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("delivery event %d failure claim was lost", event.ID)
	}
	return nil
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<uint(attempt-1)) * 2 * time.Second
}

func truncateDeliveryError(value string) string {
	const limit = 2000
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
