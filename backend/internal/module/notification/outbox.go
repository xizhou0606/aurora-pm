package notification

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Enqueue records one independently retryable notification event per recipient.
// The caller owns the scene, recipients, variables, and final message body; this
// package only provides PM's durable delivery boundary.
func Enqueue(
	tx *gorm.DB,
	aggregateType string,
	aggregateID uint,
	intent Intent,
) error {
	if tx == nil {
		return fmt.Errorf("notification outbox transaction is required")
	}
	aggregateType = strings.TrimSpace(aggregateType)
	intent.Scene = strings.TrimSpace(intent.Scene)
	intent.FallbackPlainText = strings.TrimSpace(intent.FallbackPlainText)
	if aggregateType == "" || aggregateID == 0 || intent.WorkspaceID == 0 ||
		!ValidScope(intent) || intent.Scene == "" {
		return fmt.Errorf("notification outbox scope is invalid")
	}
	if intent.FallbackPlainText == "" {
		return fmt.Errorf("notification scene %s has no PM-owned text", intent.Scene)
	}
	recipients := dedupeUserIDs(intent.RecipientUserIDs)
	if len(recipients) == 0 {
		return nil
	}
	if intent.Vars == nil {
		intent.Vars = map[string]string{}
	}
	occurredAt := time.Now().UTC()
	for _, recipientID := range recipients {
		intent.RecipientUserIDs = []uint{recipientID}
		raw, err := json.Marshal(intent)
		if err != nil {
			return err
		}
		event := auditgorm.OutboxEvent{
			WorkspaceID: intent.WorkspaceID, AggregateType: aggregateType,
			AggregateID: aggregateID, EventType: EventType(intent.Scene),
			Payload: string(raw), OccurredAt: occurredAt,
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if category := InboxCategory(intent.Scene); category != "" && !selfMutationReceipt(intent, recipientID) {
			key := intent.Vars["notification_event_key"]
			if key == "" {
				key = fmt.Sprintf("outbox:%d", event.ID)
			}
			row := records.InboxReceipt{WorkspaceID: intent.WorkspaceID, RecipientUserID: recipientID, EventID: event.ID, EventKey: intent.Scene + ":" + key, Category: category}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				// A retry must not leave another dispatchable event.
				if err := tx.Delete(&event).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// A user's own creation/status change is not an inbox notification. Keep this
// at the shared receipt boundary so GUI and Tool producers follow one rule.
// Existing external delivery and approval receipt policies remain independent.
func selfMutationReceipt(intent Intent, recipientID uint) bool {
	if intent.ActorUserID == 0 || intent.ActorUserID != recipientID {
		return false
	}
	switch intent.Scene {
	case SceneQualityBugCreated, SceneQualityBugReviewRequested, SceneQualityBugReturned, SceneWorkTaskStatusChanged:
		return true
	default:
		return false
	}
}
