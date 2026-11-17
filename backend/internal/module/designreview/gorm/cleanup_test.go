package designreviewgorm

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
)

func TestReviewDeletionCollectsObjectsAndPurgesChainMetadata(t *testing.T) {
	db := openCleanupDB(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	rootID := "91000000-0000-4000-8000-000000000001"
	childID := "91000000-0000-4000-8000-000000000002"
	for _, row := range []Review{
		{ID: rootID, WorkspaceID: 7, ChainRootID: &rootID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now},
		{ID: childID, WorkspaceID: 7, ChainRootID: &rootID, SourceReviewID: &rootID, RoundNumber: 2, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now},
	} {
		validReviewRecord(&row)
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	lineageID := "92000000-0000-4000-8000-000000000001"
	differenceID := "93000000-0000-4000-8000-000000000001"
	verificationID := "94000000-0000-4000-8000-000000000001"
	records := []any{
		&Asset{ID: "asset-root", WorkspaceID: 7, ReviewID: rootID, Kind: "design_original", ObjectKey: "workspace/7/design/root.png", ContentType: "image/png", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CreatedAt: now},
		&Asset{ID: "asset-child", WorkspaceID: 7, ReviewID: childID, Kind: "implementation_original", ObjectKey: "workspace/7/design/child.png", ContentType: "image/png", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CreatedAt: now},
		&DifferenceLineage{ID: lineageID, WorkspaceID: 7, ChainRootReviewID: rootID, VerificationStatus: "pending", CreatedAt: now, UpdatedAt: now},
		&Difference{ID: differenceID, WorkspaceID: 7, ReviewID: rootID, LineageID: &lineageID, Ordinal: 1, Source: "ai", AnalysisKind: "full", Category: "layout", Severity: "major", Title: "difference", EvidenceBasis: "visual", VerificationStatus: "pending", ResolutionStatus: "unresolved", CreatedAt: now, UpdatedAt: now},
		&LineageEvent{ID: "lineage-event", WorkspaceID: 7, LineageID: lineageID, ReviewID: rootID, DifferenceID: &differenceID, PreviousStatus: "pending", NextStatus: "confirmed", CreatedAt: now},
		&RecheckMatch{ID: "match", WorkspaceID: 7, ReviewID: childID, CandidateID: "candidate", SuggestedLineageID: lineageID, Candidate: []byte(`{}`), Status: "pending", CreatedAt: now, UpdatedAt: now},
		&RecheckVerification{ID: verificationID, WorkspaceID: 7, ReviewID: childID, HistoricalDifferenceID: differenceID, LineageRootDifferenceID: differenceID, LineageID: &lineageID, AIStatus: "resolved", CreatedAt: now, UpdatedAt: now},
		&ResolutionEvent{ID: "resolution", WorkspaceID: 7, ReviewID: childID, DifferenceID: &differenceID, RecheckVerificationID: &verificationID, ActorUserID: 42, EventType: "resolved", CreatedAt: now},
		&AnalysisJob{ID: "analysis", WorkspaceID: 7, ReviewID: rootID, Status: "succeeded", AvailableAt: now, CreatedAt: now, UpdatedAt: now},
		&DeepJob{ID: "deep", WorkspaceID: 7, ReviewID: childID, CreatedByUserID: 42, Status: "pending", DesignRegion: []byte(`{}`), ImplementationRegion: []byte(`{}`), AvailableAt: now, CreatedAt: now, UpdatedAt: now},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed %T: %v", record, err)
		}
	}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now.Add(time.Minute) }

	if err := repository.MarkDeleting(context.Background(), 7, rootID, 42); err != nil {
		t.Fatal(err)
	}
	var deep DeepJob
	if err := db.First(&deep, "id = ?", "deep").Error; err != nil || deep.Status != "cancelled" || !deep.CancelRequested {
		t.Fatalf("deep job was not cancelled: job=%+v err=%v", deep, err)
	}
	batch, err := repository.NextDeletion(context.Background())
	if err != nil || batch == nil {
		t.Fatalf("deletion batch=%+v err=%v", batch, err)
	}
	sort.Strings(batch.ObjectKeys)
	wantKeys := []string{"workspace/7/design/child.png", "workspace/7/design/root.png"}
	if len(batch.ObjectKeys) != len(wantKeys) || batch.ObjectKeys[0] != wantKeys[0] || batch.ObjectKeys[1] != wantKeys[1] {
		t.Fatalf("object keys=%v want=%v", batch.ObjectKeys, wantKeys)
	}
	if err := repository.PurgeDeletion(context.Background(), *batch); err != nil {
		t.Fatal(err)
	}
	for _, model := range []any{&Review{}, &Asset{}, &DifferenceLineage{}, &Difference{}, &LineageEvent{}, &RecheckMatch{}, &RecheckVerification{}, &ResolutionEvent{}, &AnalysisJob{}, &DeepJob{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("remaining %T count=%d err=%v", model, count, err)
		}
	}
	var auditCount int64
	if err := db.Model(&auditgorm.AuditLog{}).Where("workspace_id = ? AND action = ? AND target = ?", 7, "delete_requested", rootID).Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("delete audit count=%d err=%v", auditCount, err)
	}
}

func TestExpiredCaptureCleanupRetainsObjectMetadataUntilDeletion(t *testing.T) {
	db := openCleanupDB(t)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	objectKey := "workspace/7/design-reviews/mobile-captures/draft/screenshot.png"
	if err := db.Create(&MobileCaptureDraft{
		ID: "draft", WorkspaceID: 7, UserID: 42, CollectorID: "collector", Status: "succeeded",
		Platform: "ios", DeviceID: "device", Warnings: []byte("[]"), ScreenshotObjectKey: &objectKey,
		ExpiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&MobilePairingCode{ID: "code", WorkspaceID: 7, UserID: 42, CodeHash: "hash", ExpiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	objects, err := repository.ExpireCaptureDrafts(context.Background(), now)
	if err != nil || len(objects) != 1 || objects[0].ObjectKey != objectKey {
		t.Fatalf("objects=%+v err=%v", objects, err)
	}
	var draft MobileCaptureDraft
	if err := db.First(&draft, "id = ?", "draft").Error; err != nil || draft.Status != "expired" || draft.ScreenshotObjectKey == nil {
		t.Fatalf("draft=%+v err=%v", draft, err)
	}
	if err := repository.CompleteCaptureObjectCleanup(context.Background(), draft.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := repository.PurgeExpiredCaptureState(context.Background(), now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, model := range []any{&MobileCaptureDraft{}, &MobilePairingCode{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("remaining %T count=%d err=%v", model, count, err)
		}
	}
}

func TestObjectCleanupUsesLeaseAndBackoffState(t *testing.T) {
	db := openCleanupDB(t)
	now := time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC)
	job := ObjectCleanupJob{ID: "cleanup", WorkspaceID: 7, ObjectKey: "workspace/7/replaced.png", AvailableAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now }

	claimed, err := repository.ClaimObjectCleanup(context.Background(), "worker-a", time.Minute)
	if err != nil || claimed == nil || claimed.Attempts != 1 || claimed.LeaseOwner == nil || *claimed.LeaseOwner != "worker-a" {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	if duplicate, err := repository.ClaimObjectCleanup(context.Background(), "worker-b", time.Minute); err != nil || duplicate != nil {
		t.Fatalf("duplicate claim=%+v err=%v", duplicate, err)
	}
	retryAt := now.Add(2 * time.Minute)
	if err := repository.FailObjectCleanup(context.Background(), claimed.ID, "worker-a", "temporary failure", retryAt); err != nil {
		t.Fatal(err)
	}
	if early, err := repository.ClaimObjectCleanup(context.Background(), "worker-b", time.Minute); err != nil || early != nil {
		t.Fatalf("early retry=%+v err=%v", early, err)
	}
	repository.now = func() time.Time { return retryAt }
	retried, err := repository.ClaimObjectCleanup(context.Background(), "worker-b", time.Minute)
	if err != nil || retried == nil || retried.Attempts != 2 {
		t.Fatalf("retried=%+v err=%v", retried, err)
	}
	if err := repository.CompleteObjectCleanup(context.Background(), 7, retried.ObjectKey); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&ObjectCleanupJob{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("cleanup job count=%d err=%v", count, err)
	}
}

func openCleanupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := append(MigrationRecords(), &auditgorm.AuditLog{})
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	return db
}
