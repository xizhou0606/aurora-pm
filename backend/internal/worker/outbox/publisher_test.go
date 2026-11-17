package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

type fakeLookup struct {
	openIDs map[uint]string
	emails  map[uint]string
	saved   map[uint]string
	err     error
}

func (f *fakeLookup) FeishuOpenIDs(_ context.Context, _ []uint) (map[uint]string, error) {
	if f == nil {
		return map[uint]string{}, nil
	}
	if f.err != nil {
		return nil, f.err
	}
	if f.openIDs == nil {
		return map[uint]string{}, nil
	}
	return f.openIDs, nil
}

func (f *fakeLookup) FeishuEmails(_ context.Context, _ []uint) (map[uint]string, error) {
	if f == nil {
		return map[uint]string{}, nil
	}
	if f.err != nil {
		return nil, f.err
	}
	if f.emails == nil {
		return map[uint]string{}, nil
	}
	return f.emails, nil
}

func (f *fakeLookup) SaveFeishuOpenID(_ context.Context, userID uint, openID string) error {
	if f.saved == nil {
		f.saved = map[uint]string{}
	}
	f.saved[userID] = openID
	if f.openIDs == nil {
		f.openIDs = map[uint]string{}
	}
	f.openIDs[userID] = openID
	return nil
}

type fakeSendCall struct {
	key    string
	idType string
	openID string
	card   string
}

type fakeSender struct {
	calls   []fakeSendCall
	err     error
	failFor map[string]error
}

func (f *fakeSender) SendInteractive(_ context.Context, idempotencyKey, receiveIDType, receiveID, cardJSON string) error {
	f.calls = append(f.calls, fakeSendCall{key: idempotencyKey, idType: receiveIDType, openID: receiveID, card: cardJSON})
	if f.failFor != nil {
		if err := f.failFor[receiveID]; err != nil {
			return err
		}
	}
	return f.err
}

type fakeTranslator struct {
	from  string
	to    string
	err   error
	calls int
}

func (f *fakeTranslator) TranslateToCurrent(_ context.Context, source string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	if source != f.from {
		return "", errors.New("unexpected source open_id")
	}
	return f.to, nil
}

type fakeDispatcher struct {
	mu       sync.Mutex
	failures map[uint]error
	calls    []uint
}

func (f *fakeDispatcher) Dispatch(_ context.Context, event auditgorm.OutboxEvent) (DeliveryOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, event.ID)
	if err := f.failures[event.ID]; err != nil {
		return DeliveryOutcome{}, err
	}
	return DeliveryOutcome{}, nil
}

func TestPublisherClaimsPublishesRetriesAndIgnoresDomainEvents(t *testing.T) {
	db := notificationTestDB(t)
	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	first := notificationTestEvent(t, 1, now)
	second := notificationTestEvent(t, 2, now)
	generic := auditgorm.OutboxEvent{
		WorkspaceID: 1, AggregateType: "quality_bug", AggregateID: 3,
		EventType: "pm.quality_bug.updated", Payload: `{}`, OccurredAt: now,
	}
	for _, event := range []*auditgorm.OutboxEvent{&first, &second, &generic} {
		if err := db.Create(event).Error; err != nil {
			t.Fatal(err)
		}
	}
	dispatcher := &fakeDispatcher{failures: map[uint]error{second.ID: errors.New("hub unavailable")}}
	publisher := &Publisher{
		db: db, cfg: notificationTestConfig(), dispatcher: dispatcher,
		workerID: "worker-a", now: func() time.Time { return now },
	}

	processed, err := publisher.PublishOnce(context.Background())
	if processed != 2 || err == nil || !strings.Contains(err.Error(), "hub unavailable") {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	var storedFirst, storedSecond, storedGeneric auditgorm.OutboxEvent
	if db.First(&storedFirst, first.ID).Error != nil || db.First(&storedSecond, second.ID).Error != nil ||
		db.First(&storedGeneric, generic.ID).Error != nil {
		t.Fatal("load outbox events")
	}
	if storedFirst.PublishedAt == nil || storedFirst.Attempts != 1 || storedFirst.ClaimedAt != nil {
		t.Fatalf("published event=%+v", storedFirst)
	}
	if storedSecond.PublishedAt != nil || storedSecond.Attempts != 1 || storedSecond.NextAttemptAt == nil ||
		!strings.Contains(storedSecond.LastError, "hub unavailable") || storedSecond.ClaimedAt != nil {
		t.Fatalf("failed event=%+v", storedSecond)
	}
	if storedGeneric.PublishedAt != nil || storedGeneric.Attempts != 0 {
		t.Fatalf("generic event was claimed=%+v", storedGeneric)
	}

	delete(dispatcher.failures, second.ID)
	now = storedSecond.NextAttemptAt.Add(time.Second)
	processed, err = publisher.PublishOnce(context.Background())
	if processed != 1 || err != nil {
		t.Fatalf("retry processed=%d err=%v", processed, err)
	}
	if err := db.First(&storedSecond, second.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedSecond.PublishedAt == nil || storedSecond.Attempts != 2 || storedSecond.LastError != "" {
		t.Fatalf("retried event=%+v", storedSecond)
	}
}

func TestIMDispatcherSendsPMOwnedTextToLookedUpOpenID(t *testing.T) {
	sender := &fakeSender{}
	dispatcher := newIMDispatcher(&fakeLookup{openIDs: map[uint]string{77: "ou_alice"}}, sender, nil)
	event := notificationTestEvent(t, 42, time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC))
	intent := decodeEventIntent(t, event)
	intent.RecipientUserIDs = []uint{77}
	intent.FallbackPlainText = "PM quality bug created"
	event.Payload = mustJSON(t, intent)
	outcome, err := dispatcher.Dispatch(context.Background(), event)
	if err != nil || outcome.Skipped {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if len(sender.calls) != 1 || sender.calls[0].key != "pm-outbox-42-card" ||
		sender.calls[0].idType != "open_id" || sender.calls[0].openID != "ou_alice" ||
		!strings.Contains(sender.calls[0].card, notification.CardTitle) ||
		!strings.Contains(sender.calls[0].card, "PM quality bug created") ||
		strings.Contains(sender.calls[0].card, "One-2-All") {
		t.Fatalf("calls=%+v", sender.calls)
	}
}

func TestIMDispatcherSendsViaEmailWhenOpenIDMissing(t *testing.T) {
	sender := &fakeSender{}
	dispatcher := newIMDispatcher(&fakeLookup{emails: map[uint]string{77: "alice@example.com"}}, sender, nil)
	event := notificationTestEvent(t, 48, time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC))
	intent := decodeEventIntent(t, event)
	intent.RecipientUserIDs = []uint{77}
	intent.FallbackPlainText = "PM task commented"
	event.Payload = mustJSON(t, intent)
	outcome, err := dispatcher.Dispatch(context.Background(), event)
	if err != nil || outcome.Skipped {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if len(sender.calls) != 1 || sender.calls[0].idType != "email" ||
		sender.calls[0].openID != "alice@example.com" ||
		sender.calls[0].key != "pm-outbox-48-card" {
		t.Fatalf("calls=%+v", sender.calls)
	}
}

func TestIMDispatcherRetriesRecipientWithoutIdentity(t *testing.T) {
	sender := &fakeSender{}
	dispatcher := newIMDispatcher(&fakeLookup{openIDs: map[uint]string{}}, sender, nil)
	event := notificationTestEvent(t, 44, time.Now().UTC())
	outcome, err := dispatcher.Dispatch(context.Background(), event)
	if err == nil || outcome.Skipped || !strings.Contains(err.Error(), "no deliverable Feishu identity") {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if len(sender.calls) != 0 {
		t.Fatalf("sender was called: %+v", sender.calls)
	}
}

func TestRecipientPermanentlyUnavailable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  string
		want bool
	}{
		{err: "user has no feishu_open_id", want: true},
		{err: "user not found", want: true},
		{err: `notification rpc status=502 body={"error":"feishu im http 400: {\"msg\":\"open_id cross app\"}"}`, want: true},
		{err: `feishu im http 400: {"code":230013,"msg":"Bot has NO availability to this user."}`, want: true},
		{err: "hub unavailable", want: false},
	}
	for _, tc := range cases {
		if got := recipientPermanentlyUnavailable(errors.New(tc.err)); got != tc.want {
			t.Fatalf("err=%q got=%v want=%v", tc.err, got, tc.want)
		}
	}
}

func TestIMDispatcherRejectsMissingPMOwnedText(t *testing.T) {
	dispatcher := newIMDispatcher(&fakeLookup{}, &fakeSender{}, nil)
	event := notificationTestEvent(t, 43, time.Now().UTC())
	intent := decodeEventIntent(t, event)
	intent.FallbackPlainText = "  "
	event.Payload = mustJSON(t, intent)
	outcome, err := dispatcher.Dispatch(context.Background(), event)
	if err == nil || outcome.Skipped || !strings.Contains(err.Error(), "no PM-owned text") {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}

func TestIMDispatcherSkipsPermanentFeishuDeliveryErrors(t *testing.T) {
	dispatcher := newIMDispatcher(
		&fakeLookup{openIDs: map[uint]string{8: "ou_bob"}},
		&fakeSender{err: errors.New(`feishu im http 400: {"code":230013,"msg":"open_id cross app"}`)},
		nil,
	)
	event := notificationTestEvent(t, 45, time.Now().UTC())
	outcome, err := dispatcher.Dispatch(context.Background(), event)
	if err != nil || !outcome.Skipped {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}

func TestIMDispatcherRewritesPreviousOpenIDAndSavesCurrent(t *testing.T) {
	lookup := &fakeLookup{openIDs: map[uint]string{33: "ou_old"}}
	sender := &fakeSender{failFor: map[string]error{
		"ou_old": errors.New(`feishu im http 400: {"code":230013,"msg":"open_id cross app"}`),
	}}
	translator := &fakeTranslator{from: "ou_old", to: "ou_new"}
	dispatcher := newIMDispatcher(lookup, sender, translator)
	event := notificationTestEvent(t, 46, time.Now().UTC())
	intent := decodeEventIntent(t, event)
	intent.RecipientUserIDs = []uint{33}
	event.Payload = mustJSON(t, intent)
	outcome, err := dispatcher.Dispatch(context.Background(), event)
	if err != nil || outcome.Skipped {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if translator.calls != 1 || lookup.saved[33] != "ou_new" {
		t.Fatalf("saved=%v translator_calls=%d", lookup.saved, translator.calls)
	}
	if len(sender.calls) != 2 || sender.calls[0].idType != "open_id" ||
		sender.calls[0].openID != "ou_old" || sender.calls[1].openID != "ou_new" {
		t.Fatalf("calls=%+v", sender.calls)
	}
}

func TestNewPublisherIgnoresRetiredHubRPCURL(t *testing.T) {
	publisher, err := NewPublisher(config.Notification{
		Enabled: true, HubRPCBaseURL: "http://one2all-integration-hub.one2all.svc.cluster.local:8080",
		InternalToken: "leftover-token", PublicBaseURL: "https://pm-main.example.invalid",
		PollInterval: time.Second, RequestTimeout: time.Second, ClaimTTL: time.Minute, BatchSize: 20,
	}, notificationTestDB(t), &fakeLookup{openIDs: map[uint]string{8: "ou_current"}}, &fakeSender{}, nil)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	if _, ok := publisher.dispatcher.(*imDispatcher); !ok {
		t.Fatalf("dispatcher=%T want *imDispatcher", publisher.dispatcher)
	}
}

func TestIMDispatcherSkipsCrossAppWhenRewriteFails(t *testing.T) {
	dispatcher := newIMDispatcher(
		&fakeLookup{openIDs: map[uint]string{8: "ou_old"}},
		&fakeSender{err: errors.New(`feishu im http 400: {"code":230013,"msg":"open_id cross app"}`)},
		&fakeTranslator{err: errors.New("previous Feishu application did not return a translatable identity")},
	)
	event := notificationTestEvent(t, 47, time.Now().UTC())
	outcome, err := dispatcher.Dispatch(context.Background(), event)
	if err != nil || !outcome.Skipped {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}

func notificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite", DatabaseDSN: "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared",
		AuthMode: "local", LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters",
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auditgorm.OutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func notificationTestConfig() config.Notification {
	return config.Notification{
		Enabled: true, PublicBaseURL: "https://pm-main.example.invalid", PollInterval: time.Second,
		RequestTimeout: time.Second, ClaimTTL: time.Minute, BatchSize: 20,
	}
}

func notificationTestEvent(t *testing.T, id uint, occurredAt time.Time) auditgorm.OutboxEvent {
	t.Helper()
	intent := notification.Intent{
		WorkspaceID: 1, Scene: notification.SceneQualityBugCreated, TeamID: 10, ProjectID: 12,
		ActorUserID: 7, RecipientUserIDs: []uint{8},
		Vars: map[string]string{"title": "quality bug"}, FallbackPlainText: "quality bug created",
	}
	return auditgorm.OutboxEvent{
		ID: id, WorkspaceID: 1, AggregateType: "quality_bug_notification", AggregateID: id,
		EventType: notification.EventType(intent.Scene), Payload: mustJSON(t, intent), OccurredAt: occurredAt,
	}
}

func decodeEventIntent(t *testing.T, event auditgorm.OutboxEvent) notification.Intent {
	t.Helper()
	var intent notification.Intent
	if err := json.Unmarshal([]byte(event.Payload), &intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
