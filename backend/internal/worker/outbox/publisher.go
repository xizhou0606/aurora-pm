// Package outbox owns the PM notification outbox delivery loop.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type Publisher struct {
	db         *gorm.DB
	cfg        config.Notification
	dispatcher Dispatcher
	workerID   string
	now        func() time.Time
}

func NewPublisher(cfg config.Notification, db *gorm.DB, lookup RecipientLookup, sender MessageSender, translator OpenIDTranslator) (*Publisher, error) {
	if cfg.Enabled && (lookup == nil || sender == nil) {
		return nil, fmt.Errorf("notification delivery lookup and sender are required")
	}
	hostname, _ := os.Hostname()
	return &Publisher{
		db: db, cfg: cfg, dispatcher: newIMDispatcher(lookup, sender, translator),
		workerID: fmt.Sprintf("%s-%d", hostname, os.Getpid()),
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

func (p *Publisher) Run(ctx context.Context) {
	if !p.cfg.Enabled {
		return
	}
	p.publishAndLog(ctx)
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.publishAndLog(ctx)
		}
	}
}

func (p *Publisher) publishAndLog(ctx context.Context) {
	if _, err := p.PublishOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("notification outbox publish failed: %v", err)
	}
}

func (p *Publisher) PublishOnce(ctx context.Context) (int, error) {
	processed := 0
	var failures []error
	for processed < p.cfg.BatchSize {
		event, found, err := p.claimNext(ctx)
		if err != nil {
			return processed, err
		}
		if !found {
			break
		}
		processed++
		outcome, dispatchErr := p.dispatcher.Dispatch(ctx, event)
		if dispatchErr != nil {
			if err := p.markFailed(event, dispatchErr); err != nil {
				failures = append(failures, err)
			} else {
				failures = append(failures, fmt.Errorf("event %d: %w", event.ID, dispatchErr))
			}
			continue
		}
		if err := p.markPublished(event, outcome); err != nil {
			failures = append(failures, err)
		}
	}
	return processed, errors.Join(failures...)
}

func (p *Publisher) claimNext(ctx context.Context) (auditgorm.OutboxEvent, bool, error) {
	now := p.now()
	staleBefore := now.Add(-p.cfg.ClaimTTL)
	var event auditgorm.OutboxEvent
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("published_at IS NULL").
			Where("event_type LIKE ?", notification.EventTypePrefix+"%").
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
				"claimed_at": now, "claimed_by": p.workerID, "next_attempt_at": nil,
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
	event.ClaimedBy = p.workerID
	return event, true, nil
}

func (p *Publisher) markPublished(event auditgorm.OutboxEvent, outcome DeliveryOutcome) error {
	now := p.now()
	lastError := ""
	if outcome.Skipped {
		lastError = truncateError("skipped: " + outcome.Reason)
	}
	result := p.db.Model(&auditgorm.OutboxEvent{}).
		Where("id = ? AND claimed_by = ? AND published_at IS NULL", event.ID, p.workerID).
		Updates(map[string]any{
			"published_at": now, "claimed_at": nil, "claimed_by": "",
			"next_attempt_at": nil, "last_error": lastError,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("notification event %d publish claim was lost", event.ID)
	}
	return nil
}

func (p *Publisher) markFailed(event auditgorm.OutboxEvent, publishErr error) error {
	nextAttempt := p.now().Add(retryDelay(event.Attempts))
	result := p.db.Model(&auditgorm.OutboxEvent{}).
		Where("id = ? AND claimed_by = ? AND published_at IS NULL", event.ID, p.workerID).
		Updates(map[string]any{
			"claimed_at": nil, "claimed_by": "", "next_attempt_at": nextAttempt,
			"last_error": truncateError(publishErr.Error()),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("notification event %d failure claim was lost", event.ID)
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

func truncateError(value string) string {
	const limit = 2000
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
