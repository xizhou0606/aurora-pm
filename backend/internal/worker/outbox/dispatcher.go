package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
)

type DeliveryOutcome struct {
	Skipped bool
	Reason  string
}

type Dispatcher interface {
	Dispatch(context.Context, auditgorm.OutboxEvent) (DeliveryOutcome, error)
}

type RecipientLookup interface {
	FeishuOpenIDs(context.Context, []uint) (map[uint]string, error)
	FeishuEmails(context.Context, []uint) (map[uint]string, error)
	SaveFeishuOpenID(context.Context, uint, string) error
}

type MessageSender interface {
	SendInteractive(ctx context.Context, idempotencyKey, receiveIDType, receiveID, cardJSON string) error
}

type OpenIDTranslator interface {
	TranslateToCurrent(context.Context, string) (string, error)
}

type imDispatcher struct {
	lookup     RecipientLookup
	sender     MessageSender
	translator OpenIDTranslator
}

func newIMDispatcher(lookup RecipientLookup, sender MessageSender, translator OpenIDTranslator) *imDispatcher {
	return &imDispatcher{lookup: lookup, sender: sender, translator: translator}
}

func (d *imDispatcher) Dispatch(ctx context.Context, event auditgorm.OutboxEvent) (DeliveryOutcome, error) {
	intent, err := decodeIntent(event)
	if err != nil {
		return DeliveryOutcome{}, err
	}
	if intent.SkipExternal {
		return DeliveryOutcome{Skipped: true, Reason: "external channel disabled for this event"}, nil
	}
	body := strings.TrimSpace(intent.FallbackPlainText)
	if body == "" {
		return DeliveryOutcome{}, fmt.Errorf("notification scene %s has no PM-owned text", intent.Scene)
	}
	cardJSON, err := notification.InteractiveCardJSON(body)
	if err != nil {
		return DeliveryOutcome{}, err
	}
	openIDs, err := d.lookup.FeishuOpenIDs(ctx, intent.RecipientUserIDs)
	if err != nil {
		return DeliveryOutcome{}, err
	}
	emails, err := d.lookup.FeishuEmails(ctx, intent.RecipientUserIDs)
	if err != nil {
		return DeliveryOutcome{}, err
	}
	sent := 0
	missing := 0
	for index, userID := range intent.RecipientUserIDs {
		receiveIDType, receiveID := "open_id", strings.TrimSpace(openIDs[userID])
		if receiveID == "" {
			receiveIDType, receiveID = "email", strings.TrimSpace(emails[userID])
		}
		if receiveID == "" {
			missing++
			continue
		}
		key := fmt.Sprintf("pm-outbox-%d-card", event.ID)
		if len(intent.RecipientUserIDs) > 1 {
			key = fmt.Sprintf("pm-outbox-%d-%d-card", event.ID, index)
		}
		if err := d.sendOrRewrite(ctx, userID, key, receiveIDType, receiveID, cardJSON); err != nil {
			if recipientPermanentlyUnavailable(err) {
				continue
			}
			return DeliveryOutcome{}, err
		}
		sent++
	}
	log.Printf("notification event %d delivered=%d missing_identity=%d", event.ID, sent, missing)
	if sent == 0 {
		if missing > 0 {
			return DeliveryOutcome{}, fmt.Errorf("recipient has no deliverable Feishu identity")
		}
		return DeliveryOutcome{Skipped: true, Reason: "recipient permanently unavailable to this Feishu application"}, nil
	}
	return DeliveryOutcome{}, nil
}

func (d *imDispatcher) sendOrRewrite(ctx context.Context, userID uint, key, receiveIDType, receiveID, cardJSON string) error {
	err := d.sender.SendInteractive(ctx, key, receiveIDType, receiveID, cardJSON)
	if err == nil || receiveIDType != "open_id" || d.translator == nil || !openIDIssuedByAnotherApp(err) {
		return err
	}
	translated, translateErr := d.translator.TranslateToCurrent(ctx, receiveID)
	if translateErr != nil {
		return err
	}
	translated = strings.TrimSpace(translated)
	if translated == "" || translated == receiveID {
		return err
	}
	if d.lookup != nil {
		_ = d.lookup.SaveFeishuOpenID(ctx, userID, translated)
	}
	return d.sender.SendInteractive(ctx, key, "open_id", translated, cardJSON)
}

func decodeIntent(event auditgorm.OutboxEvent) (notification.Intent, error) {
	var intent notification.Intent
	if !notification.IsNotificationEvent(event.EventType) {
		return intent, fmt.Errorf("event %d is not a notification event", event.ID)
	}
	if err := json.Unmarshal([]byte(event.Payload), &intent); err != nil {
		return intent, fmt.Errorf("decode notification event %d: %w", event.ID, err)
	}
	if intent.WorkspaceID == 0 || intent.WorkspaceID != event.WorkspaceID ||
		strings.TrimSpace(intent.Scene) == "" || event.EventType != notification.EventType(intent.Scene) ||
		!notification.ValidScope(intent) || len(intent.RecipientUserIDs) == 0 {
		return intent, fmt.Errorf("notification event %d has invalid scope or recipients", event.ID)
	}
	intent.RecipientUserIDs = dedupeUserIDs(intent.RecipientUserIDs)
	if len(intent.RecipientUserIDs) == 0 {
		return intent, fmt.Errorf("notification event %d has no valid recipients", event.ID)
	}
	if intent.Vars == nil {
		intent.Vars = map[string]string{}
	}
	return intent, nil
}

func recipientPermanentlyUnavailable(err error) bool {
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "no recipients with feishu_open_id") ||
		strings.Contains(value, "user has no feishu_open_id") ||
		strings.Contains(value, "user not found") ||
		openIDIssuedByAnotherApp(err) ||
		strings.Contains(value, "bot has no availability to this user")
}

func openIDIssuedByAnotherApp(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "open_id cross app")
}

func dedupeUserIDs(values []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
