package aiworker

import (
	"context"
	"errors"
	"log"
	"time"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
)

const (
	testCaseDeliveryTimeout      = time.Minute
	deliveryScanBatchSize        = 100
	deliveryScanInterval         = 30 * time.Second
	deliveryScanAfterGenerations = 5
)

type deliveryRetry struct {
	Attempts  int
	Next      time.Time
	Permanent bool
}

func (w *Worker) tryTestCaseDelivery(ctx context.Context, run aigorm.AIRun) {
	target, found, err := qatestcase.ParseTarget(run.InputJSON)
	if err != nil {
		w.markDeliveryPermanent(run.ID)
		log.Printf("AI test-case delivery rejected permanently workspace_id=%d ai_run_id=%d error_type=%T", run.WorkspaceID, run.ID, err)
		return
	}
	if !found || w.delivery == nil {
		return
	}
	_, _ = w.materializeTestCaseDelivery(ctx, run, target)
}

func (w *Worker) materializeTestCaseDelivery(ctx context.Context, run aigorm.AIRun, target qatestcase.Target) (bool, error) {
	deliveryCtx, cancel := context.WithTimeout(ctx, testCaseDeliveryTimeout)
	defer cancel()
	_, created, err := w.delivery.Materialize(deliveryCtx, run)
	if err == nil {
		w.clearDeliveryBackoff(run.ID)
		return created, nil
	}
	if permanentDeliveryError(err) {
		w.markDeliveryPermanent(run.ID)
	} else {
		w.markDeliveryBackoff(run.ID)
	}
	log.Printf(
		"AI test-case delivery deferred workspace_id=%d instance_id=%d node_id=%q ai_run_id=%d error_type=%T",
		run.WorkspaceID, target.InstanceID, target.NodeID, run.ID, err,
	)
	return false, err
}

func permanentDeliveryError(err error) bool {
	return errors.Is(err, qatestcase.ErrRunNotEligible) ||
		errors.Is(err, qatestcase.ErrTargetInvalid) ||
		errors.Is(err, qatestcase.ErrOutputInvalid) ||
		errors.Is(err, qatestcase.ErrSubjectStale)
}

// compensateTestCaseDelivery examines at most one cursor page and performs at
// most one upload attempt. RunOnce alternates an attempted delivery with queued
// generation work so neither side can starve the other.
func (w *Worker) compensateTestCaseDelivery(ctx context.Context) (bool, error) {
	beforeID := w.deliveryScanCursor()
	var runs []aigorm.AIRun
	query := w.db.Where("capability = ? AND status IN ?", "qa.test_cases", []string{"completed", "applied"})
	if beforeID != 0 {
		query = query.Where("id < ?", beforeID)
	}
	if err := query.Order("id DESC").Limit(deliveryScanBatchSize).Find(&runs).Error; err != nil {
		return false, err
	}
	for _, run := range runs {
		w.advanceDeliveryScan(run.ID)
		target, found, err := qatestcase.ParseTarget(run.InputJSON)
		if err != nil {
			w.markDeliveryPermanent(run.ID)
			continue
		}
		if !found || w.delivery == nil || !w.deliveryRetryReady(run.ID) {
			continue
		}
		created, deliveryErr := w.materializeTestCaseDelivery(ctx, run, target)
		if deliveryErr != nil && permanentDeliveryError(deliveryErr) {
			continue
		}
		if created || deliveryErr != nil {
			w.yieldDeliveryScanToQueue()
			return true, nil
		}
	}
	if len(runs) < deliveryScanBatchSize {
		w.completeDeliveryScan()
	}
	return false, nil
}

func (w *Worker) deliveryRetryReady(runID uint) bool {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	retry, found := w.deliveryBackoff[runID]
	return !found || (!retry.Permanent && !w.now().Before(retry.Next))
}

func (w *Worker) markDeliveryBackoff(runID uint) {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	if w.deliveryBackoff == nil {
		w.deliveryBackoff = map[uint]deliveryRetry{}
	}
	retry := w.deliveryBackoff[runID]
	if retry.Permanent {
		return
	}
	retry.Attempts++
	delay := 5 * time.Second
	for index := 1; index < retry.Attempts && delay < 5*time.Minute; index++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	retry.Next = w.now().Add(delay)
	w.deliveryBackoff[runID] = retry
}

func (w *Worker) markDeliveryPermanent(runID uint) {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	if w.deliveryBackoff == nil {
		w.deliveryBackoff = map[uint]deliveryRetry{}
	}
	w.deliveryBackoff[runID] = deliveryRetry{Permanent: true}
}

func (w *Worker) clearDeliveryBackoff(runID uint) {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	delete(w.deliveryBackoff, runID)
}

func (w *Worker) deliveryScanScheduled() bool {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	if w.deliveryYieldToQueue {
		w.deliveryYieldToQueue = false
		return false
	}
	return w.deliveryScanBeforeID != 0 || w.deliveryScanNextAt.IsZero() ||
		!w.now().Before(w.deliveryScanNextAt) || w.deliveryRunsSinceScan >= deliveryScanAfterGenerations
}

func (w *Worker) deliveryScanCursor() uint {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	return w.deliveryScanBeforeID
}

func (w *Worker) advanceDeliveryScan(beforeID uint) {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	w.deliveryScanBeforeID = beforeID
}

func (w *Worker) completeDeliveryScan() {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	w.deliveryScanBeforeID = 0
	w.deliveryScanNextAt = w.now().Add(deliveryScanInterval)
	w.deliveryRunsSinceScan = 0
}

func (w *Worker) yieldDeliveryScanToQueue() {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	w.deliveryYieldToQueue = true
}

func (w *Worker) noteGenerationRunProcessed() {
	w.deliveryMu.Lock()
	defer w.deliveryMu.Unlock()
	w.deliveryRunsSinceScan++
}
