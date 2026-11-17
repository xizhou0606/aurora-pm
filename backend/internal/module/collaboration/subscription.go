package collaboration

import (
	"context"
	"encoding/json"
	"time"
)

type StreamItem struct {
	Kind     string      `json:"kind"`
	Event    *Event      `json:"event,omitempty"`
	Presence []Presence  `json:"presence,omitempty"`
	Replay   *ReplayPage `json:"replay,omitempty"`
}

type StreamOptions struct {
	PollInterval        time.Duration
	ReauthorizeInterval time.Duration
	ReplayLimit         int
}

type SubscriptionService struct {
	events   EventReader
	presence PresenceStore
	access   AccessEvaluator
	options  StreamOptions
}

func NewSubscriptionService(
	events EventReader,
	presence PresenceStore,
	access AccessEvaluator,
	options StreamOptions,
) (*SubscriptionService, error) {
	if events == nil || presence == nil || access == nil || options.PollInterval <= 0 ||
		options.ReauthorizeInterval <= 0 || options.ReplayLimit <= 0 {
		return nil, ErrInvalidInput
	}
	return &SubscriptionService{events: events, presence: presence, access: access, options: options}, nil
}

func (service *SubscriptionService) Stream(
	ctx context.Context,
	principal Principal,
	room Room,
	after uint64,
	emit func(StreamItem) error,
) error {
	if service == nil || principal.WorkspaceID == 0 || principal.WorkspaceID != room.WorkspaceID || emit == nil {
		return ErrInvalidInput
	}
	decision, err := service.authorize(ctx, principal, room)
	if err != nil {
		return err
	}
	lastAuthorization := time.Now()
	cursor := after
	presenceFingerprint := ""
	poll := time.NewTicker(service.options.PollInterval)
	defer poll.Stop()
	for {
		if time.Since(lastAuthorization) >= service.options.ReauthorizeInterval || !decision.ExpiresAt.After(time.Now()) {
			decision, err = service.authorize(ctx, principal, room)
			if err != nil {
				return err
			}
			lastAuthorization = time.Now()
		}
		page, err := service.events.Replay(ctx, room, cursor, service.options.ReplayLimit)
		if err != nil {
			return err
		}
		if page.ResetRequired {
			if err := emit(StreamItem{Kind: "resync_required", Replay: &page}); err != nil {
				return err
			}
			return ErrResetRequired
		}
		for index := range page.Events {
			event := page.Events[index]
			if event.Sequence <= cursor {
				continue
			}
			if err := emit(StreamItem{Kind: "committed", Event: &event}); err != nil {
				return err
			}
			cursor = event.Sequence
		}
		active, err := service.presence.ListActive(ctx, room)
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(active)
		if fingerprint := string(encoded); fingerprint != presenceFingerprint {
			presenceFingerprint = fingerprint
			if err := emit(StreamItem{Kind: "presence", Presence: active}); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-poll.C:
		}
	}
}

func (service *SubscriptionService) authorize(ctx context.Context, principal Principal, room Room) (AccessDecision, error) {
	decision, err := service.access.EvaluateCollaborationAccess(ctx, principal, room)
	if err != nil {
		return AccessDecision{}, err
	}
	if !decision.CanView || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(time.Now()) {
		return AccessDecision{}, ErrForbidden
	}
	return decision, nil
}
