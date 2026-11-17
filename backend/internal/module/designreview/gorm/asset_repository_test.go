package designreviewgorm

import (
	"context"
	"testing"
	"time"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func TestSaveAssetsRollsBackTheWholeReplacementBatch(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC)
	reviewID := "b1000000-0000-4000-8000-000000000001"
	review := Review{ID: reviewID, WorkspaceID: 7, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "draft", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	for _, asset := range []Asset{
		{ID: "b2000000-0000-4000-8000-000000000001", WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_original", ObjectKey: "old/original", ContentType: "image/png", ByteSize: 10, SHA256: repeatedHash("a"), Width: intPtr(320), Height: intPtr(480), CreatedAt: now},
		{ID: "b2000000-0000-4000-8000-000000000002", WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_normalized", ObjectKey: "old/normalized", ContentType: "image/png", ByteSize: 10, SHA256: repeatedHash("b"), Width: intPtr(320), Height: intPtr(480), CreatedAt: now},
	} {
		if err := db.Create(&asset).Error; err != nil {
			t.Fatal(err)
		}
	}

	repository := NewRepository(db)
	duplicateID := "b3000000-0000-4000-8000-000000000001"
	_, err := repository.SaveAssets(context.Background(), []designreview.AssetRecord{
		{ID: duplicateID, WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_original", ObjectKey: "new/original", ContentType: "image/png", ByteSize: 20, SHA256: repeatedHash("c"), Width: intPtr(640), Height: intPtr(960), CreatedAt: now, ActorUserID: 42},
		{ID: duplicateID, WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_normalized", ObjectKey: "new/normalized", ContentType: "image/png", ByteSize: 20, SHA256: repeatedHash("d"), Width: intPtr(640), Height: intPtr(960), CreatedAt: now, ActorUserID: 42},
	})
	if err == nil {
		t.Fatal("expected the second asset insert to fail")
	}

	var stored []Asset
	if err := db.Where("workspace_id = ? AND review_id = ?", 7, reviewID).Order("kind").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored[0].ObjectKey != "old/normalized" || stored[1].ObjectKey != "old/original" {
		t.Fatalf("asset batch was partially committed: %+v", stored)
	}
	var audits int64
	if err := db.Table("audit_logs").Where("workspace_id = ? AND resource = ?", 7, "design_review_asset").Count(&audits).Error; err != nil || audits != 0 {
		t.Fatalf("partial audit rows=%d err=%v", audits, err)
	}
}

func TestSaveAssetsReturnsEveryReplacedObjectKey(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 13, 15, 0, 0, time.UTC)
	reviewID := "b1000000-0000-4000-8000-000000000002"
	review := Review{ID: reviewID, WorkspaceID: 7, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "draft", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	for _, asset := range []Asset{
		{ID: "b2000000-0000-4000-8000-000000000011", WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_original", ObjectKey: "old/original", ContentType: "image/png", ByteSize: 10, SHA256: repeatedHash("a"), CreatedAt: now},
		{ID: "b2000000-0000-4000-8000-000000000012", WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_normalized", ObjectKey: "old/normalized", ContentType: "image/png", ByteSize: 10, SHA256: repeatedHash("b"), CreatedAt: now},
	} {
		if err := db.Create(&asset).Error; err != nil {
			t.Fatal(err)
		}
	}

	repository := NewRepository(db)
	oldKeys, err := repository.SaveAssets(context.Background(), []designreview.AssetRecord{
		{ID: "b3000000-0000-4000-8000-000000000011", WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_original", ObjectKey: "new/original", ContentType: "image/png", ByteSize: 20, SHA256: repeatedHash("c"), CreatedAt: now, ActorUserID: 42},
		{ID: "b3000000-0000-4000-8000-000000000012", WorkspaceID: 7, ReviewID: reviewID, Kind: "implementation_normalized", ObjectKey: "new/normalized", ContentType: "image/png", ByteSize: 20, SHA256: repeatedHash("d"), CreatedAt: now, ActorUserID: 42},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(oldKeys) != 2 || oldKeys[0] != "old/original" || oldKeys[1] != "old/normalized" {
		t.Fatalf("old keys=%v", oldKeys)
	}
	var cleanupJobs []ObjectCleanupJob
	if err := db.Order("object_key").Find(&cleanupJobs).Error; err != nil {
		t.Fatal(err)
	}
	if len(cleanupJobs) != 2 || cleanupJobs[0].ObjectKey != "old/normalized" || cleanupJobs[1].ObjectKey != "old/original" {
		t.Fatalf("cleanup jobs=%+v", cleanupJobs)
	}
	if err := repository.CompleteObjectCleanup(context.Background(), 7, "old/original"); err != nil {
		t.Fatal(err)
	}
	var remaining int64
	if err := db.Model(&ObjectCleanupJob{}).Count(&remaining).Error; err != nil || remaining != 1 {
		t.Fatalf("remaining cleanup jobs=%d err=%v", remaining, err)
	}
}

func repeatedHash(value string) string {
	result := ""
	for len(result) < 64 {
		result += value
	}
	return result[:64]
}

func intPtr(value int) *int { return &value }
