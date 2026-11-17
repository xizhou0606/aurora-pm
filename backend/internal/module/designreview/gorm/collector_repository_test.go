package designreviewgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func TestCollectorPairingTokenRevocationAndAudit(t *testing.T) {
	db := openCleanupDB(t)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	repository.now = func() time.Time { return now }
	pairing := designreview.PairingCodeRecord{ID: "pairing", WorkspaceID: 7, UserID: 42, CodeHash: "code-hash", ExpiresAt: now.Add(10 * time.Minute)}
	if err := repository.CreatePairingCode(context.Background(), pairing); err != nil {
		t.Fatal(err)
	}
	input := designreview.MobileCollectorRecord{ID: "collector", TokenHash: "token-hash", Name: "Local Mac", Version: "0.1.0", MobilewrightVersion: "0.0.53", Hostname: "pm-mac", Architecture: "arm64", Inventory: []byte(`{"devices":[]}`)}
	collector, err := repository.PairMobileCollector(context.Background(), pairing.CodeHash, input)
	if err != nil {
		t.Fatal(err)
	}
	if collector.WorkspaceID != 7 || collector.UserID != 42 {
		t.Fatalf("collector scope=%+v", collector)
	}
	var stored MobileCollector
	if err := db.First(&stored, "id = ?", input.ID).Error; err != nil || stored.TokenHash != input.TokenHash {
		t.Fatalf("stored collector=%+v err=%v", stored, err)
	}
	if _, err := repository.PairMobileCollector(context.Background(), pairing.CodeHash, designreview.MobileCollectorRecord{ID: "second", TokenHash: "other"}); !errors.Is(err, designreview.ErrForbidden) {
		t.Fatalf("reused pairing code error=%v", err)
	}
	if _, err := repository.AuthenticateCollector(context.Background(), "wrong"); !errors.Is(err, designreview.ErrForbidden) {
		t.Fatalf("wrong token error=%v", err)
	}
	if _, err := repository.AuthenticateCollector(context.Background(), input.TokenHash); err != nil {
		t.Fatal(err)
	}
	if err := repository.RevokeMobileCollector(context.Background(), 8, input.ID, 42); !errors.Is(err, designreview.ErrNotFound) {
		t.Fatalf("cross-workspace revoke error=%v", err)
	}
	if err := repository.RevokeMobileCollector(context.Background(), 7, input.ID, 99); !errors.Is(err, designreview.ErrNotFound) {
		t.Fatalf("cross-user revoke error=%v", err)
	}
	if err := repository.RevokeMobileCollector(context.Background(), 7, input.ID, 42); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AuthenticateCollector(context.Background(), input.TokenHash); !errors.Is(err, designreview.ErrForbidden) {
		t.Fatalf("revoked token error=%v", err)
	}
	var auditActions []string
	if err := db.Model(&auditgorm.AuditLog{}).Where("workspace_id = ? AND user_id = ?", 7, 42).Order("id").Pluck("action", &auditActions).Error; err != nil {
		t.Fatal(err)
	}
	want := []string{"pairing_code_created", "paired", "revoked"}
	if len(auditActions) != len(want) {
		t.Fatalf("audit actions=%v want=%v", auditActions, want)
	}
	for index := range want {
		if auditActions[index] != want[index] {
			t.Fatalf("audit actions=%v want=%v", auditActions, want)
		}
	}
}

func TestCaptureDraftOperationsAreWorkspaceAndUserScoped(t *testing.T) {
	db := openCleanupDB(t)
	now := time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	repository.now = func() time.Time { return now }
	collector := MobileCollector{ID: "collector", WorkspaceID: 7, UserID: 42, Name: "Mac", TokenHash: "hash", Version: "0.1.0", MobilewrightVersion: "0.0.53", Hostname: "pm-mac", Architecture: "arm64", Inventory: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&collector).Error; err != nil {
		t.Fatal(err)
	}
	key := "workspace/7/design-reviews/mobile-captures/draft/screenshot.png"
	mediaType := "image/png"
	size := int64(100)
	draft := MobileCaptureDraft{ID: "draft", WorkspaceID: 7, UserID: 42, CollectorID: collector.ID, Status: "succeeded", Platform: "ios", DeviceID: "device", Warnings: []byte(`[]`), ScreenshotObjectKey: &key, ScreenshotMediaType: &mediaType, ScreenshotByteSize: &size, ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}
	for _, scope := range []struct {
		workspaceID uint
		userID      uint
	}{{8, 42}, {7, 99}} {
		if _, err := repository.GetCaptureAsset(context.Background(), scope.workspaceID, draft.ID, scope.userID); !errors.Is(err, designreview.ErrNotFound) {
			t.Fatalf("asset scope=%+v error=%v", scope, err)
		}
		if _, err := repository.ConsumeCaptureDraft(context.Background(), scope.workspaceID, draft.ID, scope.userID); !errors.Is(err, designreview.ErrNotFound) {
			t.Fatalf("consume scope=%+v error=%v", scope, err)
		}
		if _, err := repository.DeleteCaptureDraft(context.Background(), scope.workspaceID, draft.ID, scope.userID); !errors.Is(err, designreview.ErrNotFound) {
			t.Fatalf("delete scope=%+v error=%v", scope, err)
		}
	}
	var stored MobileCaptureDraft
	if err := db.First(&stored, "id = ?", draft.ID).Error; err != nil || stored.Status != "succeeded" {
		t.Fatalf("capture changed across scope: capture=%+v err=%v", stored, err)
	}
}
