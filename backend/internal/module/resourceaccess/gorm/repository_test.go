package resourceaccessgorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type policyEventAppenderStub struct {
	events []resourceaccess.PolicyRevisionEvent
	err    error
}

func (stub *policyEventAppenderStub) AppendPolicyRevisionEvent(
	_ context.Context,
	event resourceaccess.PolicyRevisionEvent,
) error {
	stub.events = append(stub.events, event)
	return stub.err
}

func TestReplacePolicyCASReceiptAuditOutboxPreservesIndependentLinks(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	seedPolicy(t, db, "dr_policy", resourceaccess.ModeLink, nil, now)
	if err := db.Create(&ShareLinkRecord{
		ID: "lnk_existing", WorkspaceID: 1, ResourceID: "dr_policy", SecretHash: strings.Repeat("a", 64),
		Level: string(resourceaccess.LevelViewer), Version: 1, CreatedBy: 7, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	events := &policyEventAppenderStub{}
	repository.ConfigurePolicyRevisionEventAppenderFactory(func(*gorm.DB) (resourceaccess.PolicyRevisionEventAppender, error) {
		return events, nil
	})
	mutation := resourceaccess.PolicyMutation{
		Principal:  resourceaccess.Principal{WorkspaceID: 1, UserID: 7},
		ResourceID: "dr_policy", Mode: resourceaccess.ModeRestricted,
		Rules:            []resourceaccess.Rule{{SubjectKind: identity.SubjectRole, SubjectID: "2", Level: resourceaccess.LevelEditor}},
		ExpectedRevision: 1, IdempotencyKey: "policy-key", InputDigest: strings.Repeat("b", 64), Now: now.Add(time.Minute),
	}

	policy, err := repository.ReplacePolicy(context.Background(), mutation)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Revision != 2 || policy.Mode != resourceaccess.ModeRestricted || len(policy.Rules) != 1 {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	replayed, err := repository.ReplacePolicy(context.Background(), mutation)
	if err != nil || replayed.Revision != 2 {
		t.Fatalf("same idempotent request must replay result: %+v %v", replayed, err)
	}
	changed := mutation
	changed.InputDigest = strings.Repeat("c", 64)
	if _, err := repository.ReplacePolicy(context.Background(), changed); !errors.Is(err, resourceaccess.ErrIdempotencyConflict) {
		t.Fatalf("changed request with same key must conflict, got %v", err)
	}
	stale := mutation
	stale.IdempotencyKey = "policy-stale"
	stale.InputDigest = strings.Repeat("d", 64)
	if _, err := repository.ReplacePolicy(context.Background(), stale); !errors.Is(err, resourceaccess.ErrConflict) {
		t.Fatalf("stale revision must conflict, got %v", err)
	}

	var link ShareLinkRecord
	if err := db.First(&link, "id = ?", "lnk_existing").Error; err != nil || link.RevokedAt != nil {
		t.Fatalf("member policy edits must preserve independent links: %+v %v", link, err)
	}
	assertCount(t, db, &RuleRecord{}, 1)
	assertCount(t, db, &MutationReceiptRecord{}, 1)
	assertCount(t, db, &AuditRecord{}, 1)
	assertCount(t, db, &OutboxRecord{}, 1)
	if len(events.events) != 1 || events.events[0].PreviousRevision != 1 ||
		events.events[0].PolicyRevision != 2 || events.events[0].Mode != resourceaccess.ModeRestricted {
		t.Fatalf("unexpected policy revision events: %+v", events.events)
	}
}

func TestLockPolicyRevisionRejectsStaleMutationContract(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	seedPolicy(t, db, "dr_locked", resourceaccess.ModePrivate, nil, now)
	repository := NewRepository(db)
	policy, err := repository.LockPolicyRevision(context.Background(), 1, "dr_locked", 1)
	if err != nil || policy.Revision != 1 || policy.Mode != resourceaccess.ModePrivate {
		t.Fatalf("policy=%+v err=%v", policy, err)
	}
	if _, err := repository.LockPolicyRevision(
		context.Background(), 1, documentlibrary.ResourceID("dr_locked"), 2,
	); !errors.Is(err, resourceaccess.ErrConflict) {
		t.Fatalf("stale revision error=%v", err)
	}
}

func TestLockCurrentPolicyReturnsFreshRevisionInsideCallerTransaction(t *testing.T) {
	db := testDB(t)
	seedPolicy(t, db, "dr_current", resourceaccess.ModePrivate, nil, time.Now().UTC())
	if err := db.Transaction(func(tx *gorm.DB) error {
		policy, err := NewRepository(tx).LockCurrentPolicy(context.Background(), 1, "dr_current")
		if err != nil {
			return err
		}
		if policy.Revision != 1 || policy.Mode != resourceaccess.ModePrivate {
			t.Fatalf("locked policy=%+v", policy)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestShareLinkSecretHashReceiptAndContinuousInvalidation(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)
	seedPolicy(t, db, "dr_link", resourceaccess.ModeLink, nil, now)
	repository := NewRepository(db)
	shareSecret, shareHash := "share-secret-never-store", hashString("share-secret-never-store")
	mutation := resourceaccess.LinkMutation{
		Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 7}, ResourceID: "dr_link", LinkID: "lnk_one",
		SecretHash: shareHash, Level: resourceaccess.LevelViewer, ExpiresAt: timePointer(now.Add(24 * time.Hour)),
		IdempotencyKey: "create-link-key", InputDigest: strings.Repeat("e", 64), Now: now,
	}
	link, err := repository.CreateShareLink(context.Background(), mutation)
	if err != nil {
		t.Fatal(err)
	}
	if link.Version != 1 {
		t.Fatalf("unexpected created link: %+v", link)
	}
	if _, err := repository.CreateShareLink(context.Background(), mutation); !errors.Is(err, resourceaccess.ErrIdempotencyConflict) {
		t.Fatalf("replayed create must not mint a second secret/link, got %v", err)
	}
	assertCount(t, db, &ShareLinkRecord{}, 1)

	var stored ShareLinkRecord
	if err := db.First(&stored, "id = ?", link.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SecretHash != shareHash || strings.Contains(stored.SecretHash, shareSecret) {
		t.Fatalf("only a strong hash may be stored: %+v", stored)
	}
	var auditRows []AuditRecord
	if err := db.Find(&auditRows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range auditRows {
		if strings.Contains(row.BeforeJSON+row.AfterJSON, shareSecret) {
			t.Fatal("plaintext link secret leaked to audit")
		}
	}

	principal := resourceaccess.Principal{WorkspaceID: 1, UserID: 9}
	accessHash := hashString("access-context-one")
	if err := repository.CreateLinkAccessContext(
		context.Background(), principal, link, shareHash, accessHash, now, now.Add(15*time.Minute),
	); err != nil {
		t.Fatal(err)
	}
	levels, err := repository.ResolveLinkAccess(context.Background(), principal, []documentlibrary.ResourceID{"dr_link"}, accessHash, now.Add(time.Minute))
	if err != nil || levels["dr_link"].Level != resourceaccess.LevelViewer || !levels["dr_link"].ExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("live access context not resolved: %+v %v", levels, err)
	}

	rotated, err := repository.RotateShareLink(context.Background(), resourceaccess.LinkMutation{
		Principal: mutation.Principal, ResourceID: "dr_link", LinkID: link.ID,
		SecretHash: hashString("share-secret-two"), Now: now.Add(2 * time.Minute),
	})
	if err != nil || rotated.Version != 2 {
		t.Fatalf("rotate failed: %+v %v", rotated, err)
	}
	levels, err = repository.ResolveLinkAccess(context.Background(), principal, []documentlibrary.ResourceID{"dr_link"}, accessHash, now.Add(3*time.Minute))
	if err != nil || len(levels) != 0 {
		t.Fatalf("rotation must continuously invalidate old access context: %+v %v", levels, err)
	}
	if err := repository.CreateLinkAccessContext(
		context.Background(), principal, link, shareHash, hashString("late-context"), now.Add(3*time.Minute), now.Add(4*time.Minute),
	); !errors.Is(err, resourceaccess.ErrLinkInvalid) {
		t.Fatalf("old link version must fail closed after rotation, got %v", err)
	}
	if err := repository.RevokeShareLink(context.Background(), resourceaccess.LinkMutation{
		Principal: mutation.Principal, ResourceID: "dr_link", LinkID: link.ID, Now: now.Add(4 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.FindShareLinkBySecretHash(context.Background(), rotatedSecretHash(rotated, db, t)); err != nil {
		// Finding the row is allowed; liveness is deliberately enforced at exchange.
		t.Fatal(err)
	}
}

func TestLinkExchangeLimiterIsDurableAcrossInstances(t *testing.T) {
	db := testDB(t)
	clock := &mutableClock{now: time.Date(2026, 9, 8, 13, 30, 10, 0, time.UTC)}
	left, err := NewLinkExchangeLimiter(db, clock)
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewLinkExchangeLimiter(db, clock)
	if err != nil {
		t.Fatal(err)
	}
	principal := resourceaccess.Principal{WorkspaceID: 1, UserID: 9}
	for attempt := 1; attempt <= int(defaultLinkExchangeLimit); attempt++ {
		limiter := left
		if attempt%2 == 0 {
			limiter = right
		}
		allowed, err := limiter.AllowDocumentShareLinkExchange(context.Background(), principal)
		if err != nil || !allowed {
			t.Fatalf("attempt %d should be allowed: %v %v", attempt, allowed, err)
		}
	}
	allowed, err := right.AllowDocumentShareLinkExchange(context.Background(), principal)
	if err != nil || allowed {
		t.Fatalf("shared durable counter must deny attempt 21: %v %v", allowed, err)
	}
	clock.now = clock.now.Add(defaultLinkExchangeWindow)
	allowed, err = left.AllowDocumentShareLinkExchange(context.Background(), principal)
	if err != nil || !allowed {
		t.Fatalf("next fixed window should allow again: %v %v", allowed, err)
	}
}

func rotatedSecretHash(link resourceaccess.ShareLink, db *gorm.DB, t *testing.T) string {
	t.Helper()
	var row ShareLinkRecord
	if err := db.First(&row, "id = ?", link.ID).Error; err != nil {
		t.Fatal(err)
	}
	return row.SecretHash
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(MigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedPolicy(t *testing.T, db *gorm.DB, resourceID string, mode resourceaccess.Mode, workspaceLevel *string, now time.Time) {
	t.Helper()
	if err := db.Create(&PolicyRecord{
		WorkspaceID: 1, ResourceID: resourceID, Mode: string(mode), WorkspaceLevel: workspaceLevel,
		Revision: 1, CreatedBy: 7, UpdatedBy: 7, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertCount(t *testing.T, db *gorm.DB, model any, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(model).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("unexpected %T count: got %d want %d", model, count, expected)
	}
}

func hashString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func timePointer(value time.Time) *time.Time { return &value }

type mutableClock struct{ now time.Time }

func (clock *mutableClock) Now() time.Time { return clock.now }
