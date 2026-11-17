package resourceaccessgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
)

func TestToolRequestReceiptCommitsAndReplaysWithoutRepeatingNotification(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	seedPolicy(t, db, "r", resourceaccess.ModePrivate, nil, time.Now())
	repo.ConfigureRequestOwnerGuard(func(*gorm.DB, resourceaccess.RequestMutation) error { return nil })
	notifications := 0
	repo.ConfigureRequestNotificationAppender(func(*gorm.DB, resourceaccess.RequestMutation, uint, uint) error { notifications++; return nil })
	p := resourceaccess.Principal{WorkspaceID: 1, UserID: 8}
	callback := func(_ resourceaccess.PolicyStore, requests resourceaccess.RequestStore, _ resourceaccess.LinkStore) (json.RawMessage, error) {
		result, err := requests.CreateAccessRequest(ctx, resourceaccess.RequestMutation{Principal: p, ResourceID: "r", OwnerUserID: 7, Reason: "edit", NotifyOwner: true})
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}
	digest := strings.Repeat("a", 64)
	first, err := repo.ApplyToolMutation(ctx, p, "r", "cs-1", digest, callback)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate another workflow deciding the original request before Runtime
	// recovers. Retrying this ChangeSet must not open another request.
	if err := db.Model(&AccessRequestRecord{}).Where("resource_id = ?", "r").Update("status", "rejected").Error; err != nil {
		t.Fatal(err)
	}
	second, err := repo.ApplyToolMutation(ctx, p, "r", "cs-1", digest, callback)
	if err != nil || string(first) != string(second) || notifications != 1 {
		t.Fatalf("replay=%s err=%v notifications=%d", second, err, notifications)
	}
	for _, altered := range []struct {
		p      resourceaccess.Principal
		digest string
	}{{resourceaccess.Principal{WorkspaceID: 1, UserID: 9}, digest}, {p, strings.Repeat("b", 64)}} {
		if _, err := repo.ApplyToolMutation(ctx, altered.p, "r", "cs-1", altered.digest, callback); !errors.Is(err, resourceaccess.ErrIdempotencyConflict) {
			t.Fatalf("receipt identity mismatch: %v", err)
		}
	}
	other := p
	other.WorkspaceID = 2
	_, err = repo.ApplyToolMutation(ctx, other, "r", "cs-1", digest, func(resourceaccess.PolicyStore, resourceaccess.RequestStore, resourceaccess.LinkStore) (json.RawMessage, error) {
		return nil, resourceaccess.ErrNotFound
	})
	if !errors.Is(err, resourceaccess.ErrNotFound) {
		t.Fatalf("cross-workspace replay: %v", err)
	}
}

func TestToolReceiptAndDomainWriteRollbackTogether(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	repo.ConfigureRequestOwnerGuard(func(*gorm.DB, resourceaccess.RequestMutation) error { return nil })
	seedPolicy(t, db, "r", resourceaccess.ModePrivate, nil, time.Now())
	p := resourceaccess.Principal{WorkspaceID: 1, UserID: 8}
	ctx := context.Background()
	_, err := repo.ApplyToolMutation(ctx, p, "r", "cs-1", strings.Repeat("a", 64), func(_ resourceaccess.PolicyStore, requests resourceaccess.RequestStore, _ resourceaccess.LinkStore) (json.RawMessage, error) {
		_, err := requests.CreateAccessRequest(ctx, resourceaccess.RequestMutation{Principal: p, ResourceID: "r", OwnerUserID: 7, Reason: "edit"})
		if err != nil {
			return nil, err
		}
		return nil, resourceaccess.ErrUnavailable
	})
	if !errors.Is(err, resourceaccess.ErrUnavailable) {
		t.Fatal(err)
	}
	for _, model := range []any{&AccessRequestRecord{}, &MutationReceiptRecord{}, &AuditRecord{}, &OutboxRecord{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("rollback %T count=%d err=%v", model, count, err)
		}
	}
}
