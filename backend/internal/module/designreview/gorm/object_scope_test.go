package designreviewgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func TestDifferenceMutationsRequireMatchingReview(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Repository, string, string) error
	}{
		{
			name: "update",
			mutate: func(repository *Repository, reviewID, differenceID string) error {
				_, err := repository.UpdateDifference(context.Background(), 7, reviewID, differenceID, 42, designreview.DifferenceInput{
					Category: "content", Severity: "minor", Title: "changed", Description: "changed", Recommendation: "changed",
				})
				return err
			},
		},
		{
			name: "delete",
			mutate: func(repository *Repository, reviewID, differenceID string) error {
				return repository.DeleteDifference(context.Background(), 7, reviewID, differenceID, 42)
			},
		},
		{
			name: "transition",
			mutate: func(repository *Repository, reviewID, differenceID string) error {
				_, err := repository.TransitionDifference(context.Background(), 7, reviewID, differenceID, 42, "resolve", "")
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openRecheckDB(t)
			now := time.Date(2026, 9, 4, 7, 0, 0, 0, time.UTC)
			requestedReviewID := "81000000-0000-4000-8000-000000000001"
			targetReviewID := "81000000-0000-4000-8000-000000000002"
			differenceID := "82000000-0000-4000-8000-000000000001"
			for _, id := range []string{requestedReviewID, targetReviewID} {
				review := Review{ID: id, WorkspaceID: 7, ChainRootID: &id, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now}
				validReviewRecord(&review)
				if err := db.Create(&review).Error; err != nil {
					t.Fatal(err)
				}
			}
			actor := uint(42)
			target := Difference{ID: differenceID, WorkspaceID: 7, ReviewID: targetReviewID, Ordinal: 1, Source: "manual", AnalysisKind: "full", CreatedByUserID: &actor, Category: "layout", Severity: "major", Title: "original", Description: "original", Recommendation: "original", EvidenceBasis: "human", VerificationStatus: "confirmed", ResolutionStatus: "unresolved", CreatedAt: now, UpdatedAt: now}
			if err := db.Create(&target).Error; err != nil {
				t.Fatal(err)
			}

			repository := NewRepository(db)
			if err := test.mutate(repository, requestedReviewID, differenceID); !errors.Is(err, designreview.ErrNotFound) {
				t.Fatalf("error=%v, want not found", err)
			}
			var stored Difference
			if err := db.Where("workspace_id = ? AND review_id = ? AND id = ?", 7, targetReviewID, differenceID).First(&stored).Error; err != nil {
				t.Fatalf("target difference was removed: %v", err)
			}
			if stored.Title != "original" || stored.ResolutionStatus != "unresolved" {
				t.Fatalf("target difference changed: %+v", stored)
			}
			assertNoObjectScopeSideEffects(t, db)
		})
	}
}

func TestDeepJobTransitionsRequireMatchingReview(t *testing.T) {
	for _, operation := range []string{"cancel", "retry"} {
		t.Run(operation, func(t *testing.T) {
			db := openRecheckDB(t)
			now := time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC)
			requestedReviewID := "83000000-0000-4000-8000-000000000001"
			targetReviewID := "83000000-0000-4000-8000-000000000002"
			jobID := "84000000-0000-4000-8000-000000000001"
			for _, id := range []string{requestedReviewID, targetReviewID} {
				review := Review{ID: id, WorkspaceID: 7, ChainRootID: &id, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now}
				validReviewRecord(&review)
				if err := db.Create(&review).Error; err != nil {
					t.Fatal(err)
				}
			}
			status := "pending"
			if operation == "retry" {
				status = "failed"
			}
			target := DeepJob{ID: jobID, WorkspaceID: 7, ReviewID: targetReviewID, CreatedByUserID: 42, Status: status, DesignRegion: []byte(`{"x":0.1,"y":0.1,"width":0.2,"height":0.2}`), ImplementationRegion: []byte(`{"x":0.1,"y":0.1,"width":0.2,"height":0.2}`), AvailableAt: now, CreatedAt: now, UpdatedAt: now}
			if err := db.Create(&target).Error; err != nil {
				t.Fatal(err)
			}

			repository := NewRepository(db)
			if err := repository.TransitionDeepJob(context.Background(), 7, requestedReviewID, jobID, 42, operation); !errors.Is(err, designreview.ErrNotFound) {
				t.Fatalf("error=%v, want not found", err)
			}
			var stored DeepJob
			if err := db.Where("workspace_id = ? AND review_id = ? AND id = ?", 7, targetReviewID, jobID).First(&stored).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status != status || stored.CancelRequested {
				t.Fatalf("target deep job changed: %+v", stored)
			}
			var retries int64
			if err := db.Model(&DeepJob{}).Where("retry_of_id = ?", jobID).Count(&retries).Error; err != nil || retries != 0 {
				t.Fatalf("retry side effect count=%d err=%v", retries, err)
			}
			assertNoObjectScopeSideEffects(t, db)
		})
	}
}

func TestDifferenceEditCreatorAndHistoryPolicies(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 8, 30, 0, 0, time.UTC)
	reviewID := "85000000-0000-4000-8000-000000000001"
	review := Review{ID: reviewID, WorkspaceID: 7, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 9, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	manualCreator := uint(8)
	lineageID := "88000000-0000-4000-8000-000000000001"
	if err := db.Create(&DifferenceLineage{ID: lineageID, WorkspaceID: 7, ChainRootReviewID: reviewID, VerificationStatus: "pending", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []Difference{
		{ID: "86000000-0000-4000-8000-000000000001", WorkspaceID: 7, ReviewID: reviewID, LineageID: &lineageID, Ordinal: 1, Source: "ai", AnalysisKind: "full", Category: "layout", Severity: "major", Title: "ai", EvidenceBasis: "visual", VerificationStatus: "pending", ResolutionStatus: "unresolved", CreatedAt: now, UpdatedAt: now},
		{ID: "86000000-0000-4000-8000-000000000002", WorkspaceID: 7, ReviewID: reviewID, Ordinal: 2, Source: "manual", AnalysisKind: "full", CreatedByUserID: &manualCreator, Category: "layout", Severity: "major", Title: "manual", EvidenceBasis: "human", VerificationStatus: "confirmed", ResolutionStatus: "unresolved", CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := NewRepository(db)
	input := designreview.DifferenceInput{Category: "content", Severity: "minor", Title: "changed", Description: "changed", Recommendation: "changed"}
	if _, err := repository.UpdateDifference(context.Background(), 7, reviewID, "86000000-0000-4000-8000-000000000001", 42, input); err != nil {
		t.Fatalf("authorized collaborator could not edit AI difference: %v", err)
	}
	var lineage DifferenceLineage
	if err := db.First(&lineage, "id = ?", lineageID).Error; err != nil || lineage.VerificationStatus != "confirmed" {
		t.Fatalf("lineage was not confirmed with AI edit: lineage=%+v err=%v", lineage, err)
	}
	var lineageEvents int64
	if err := db.Model(&LineageEvent{}).Where("lineage_id = ? AND previous_status = ? AND next_status = ?", lineageID, "pending", "confirmed").Count(&lineageEvents).Error; err != nil || lineageEvents != 1 {
		t.Fatalf("lineage event count=%d err=%v", lineageEvents, err)
	}
	if _, err := repository.UpdateDifference(context.Background(), 7, reviewID, "86000000-0000-4000-8000-000000000002", 42, input); !errors.Is(err, designreview.ErrCreatorOnly) {
		t.Fatalf("manual difference creator policy error=%v", err)
	}
	if _, err := repository.UpdateDifference(context.Background(), 7, reviewID, "86000000-0000-4000-8000-000000000002", manualCreator, input); err != nil {
		t.Fatalf("manual difference creator could not edit: %v", err)
	}

	childID := "85000000-0000-4000-8000-000000000002"
	child := Review{ID: childID, WorkspaceID: 7, ChainRootID: &reviewID, SourceReviewID: &reviewID, RoundNumber: 2, CreatedByUserID: 9, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&child)
	if err := db.Create(&child).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.TransitionDifference(context.Background(), 7, reviewID, "86000000-0000-4000-8000-000000000001", 42, "resolve", ""); !errors.Is(err, designreview.ErrConflict) {
		t.Fatalf("historical difference mutation error=%v", err)
	}
}

func TestDeepJobCapacityAndCreatorPolicies(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 8, 45, 0, 0, time.UTC)
	reviewID := "87000000-0000-4000-8000-000000000001"
	review := Review{ID: reviewID, WorkspaceID: 7, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 9, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	for index := 0; index < maxActiveDeepJobs; index++ {
		row := DeepJob{ID: newID(), WorkspaceID: 7, ReviewID: reviewID, CreatedByUserID: 8, Status: "pending", DesignRegion: []byte(`{}`), ImplementationRegion: []byte(`{}`), AvailableAt: now, CreatedAt: now, UpdatedAt: now}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := NewRepository(db)
	region := designreview.Region{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}
	_, err := repository.CreateDeepJob(context.Background(), designreview.DeepJobRecord{ID: newID(), WorkspaceID: 7, ReviewID: reviewID, CreatedByUserID: 42, DesignRegion: region, ImplementationRegion: region})
	if !errors.Is(err, designreview.ErrConflict) {
		t.Fatalf("capacity error=%v", err)
	}
	var existing DeepJob
	if err := db.Where("workspace_id = ? AND review_id = ?", 7, reviewID).First(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&existing).Updates(map[string]any{"status": "failed"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.TransitionDeepJob(context.Background(), 7, reviewID, existing.ID, 42, "retry"); !errors.Is(err, designreview.ErrCreatorOnly) {
		t.Fatalf("deep job creator policy error=%v", err)
	}
	if _, err := repository.CreateDeepJob(context.Background(), designreview.DeepJobRecord{ID: newID(), WorkspaceID: 7, ReviewID: reviewID, CreatedByUserID: 42, DesignRegion: region, ImplementationRegion: region}); err != nil {
		t.Fatalf("capacity was not released by terminal job: %v", err)
	}
}

func assertNoObjectScopeSideEffects(t *testing.T, db *gorm.DB) {
	t.Helper()
	var auditCount, eventCount int64
	if err := db.Model(&auditgorm.AuditLog{}).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ResolutionEvent{}).Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 0 || eventCount != 0 {
		t.Fatalf("unexpected side effects: audits=%d resolution_events=%d", auditCount, eventCount)
	}
}
