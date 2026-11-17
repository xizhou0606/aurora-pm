package designreviewgorm

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestConclusionRequiresCompleteEvidence(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	reviewID := "a1000000-0000-4000-8000-000000000001"
	review := Review{
		ID: reviewID, WorkspaceID: 7, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42,
		ProjectSetIDSnapshot: 11, Status: "completed", DimensionsMatch: true, DesignStructureAvailable: true,
		ImplementationStructureComplete: true, ModelCoverageComplete: true, HeatmapDifferenceRatio: 0.001,
		CreatedAt: now, UpdatedAt: now,
	}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := refreshConclusion(db.WithContext(context.Background()), 7, reviewID); err != nil {
		t.Fatal(err)
	}
	assertReviewConclusion(t, db, reviewID, "passed")

	if err := db.Model(&Review{}).Where("id = ?", reviewID).Update("model_coverage_complete", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := refreshConclusion(db, 7, reviewID); err != nil {
		t.Fatal(err)
	}
	assertReviewConclusion(t, db, reviewID, "needs_review")

	if err := db.Create(&Difference{ID: "a2000000-0000-4000-8000-000000000001", WorkspaceID: 7, ReviewID: reviewID, Ordinal: 1, Source: "manual", AnalysisKind: "full", Category: "layout", Severity: "major", Title: "blocking", Confidence: 1, EvidenceBasis: "visual", VerificationStatus: "pending", ResolutionStatus: "unresolved", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := refreshConclusion(db, 7, reviewID); err != nil {
		t.Fatal(err)
	}
	assertReviewConclusion(t, db, reviewID, "needs_fix")
}

func TestConclusionTreatsPartialDOMNormalizationAsIncomplete(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 12, 30, 0, 0, time.UTC)
	reviewID := "a1000000-0000-4000-8000-000000000002"
	review := Review{ID: reviewID, WorkspaceID: 7, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42,
		ProjectSetIDSnapshot: 11, Status: "completed", DimensionsMatch: true, DesignStructureAvailable: true,
		ImplementationStructureComplete: true, ModelCoverageComplete: true, HeatmapDifferenceRatio: 0,
		CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	review.DOMNormalization = []byte(`{"enabled":true,"status":"partial","report":{"counts":{"applied":1,"skipped":1}}}`)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := refreshConclusion(db, 7, reviewID); err != nil {
		t.Fatal(err)
	}
	assertReviewConclusion(t, db, reviewID, "needs_review")
}

func assertReviewConclusion(t *testing.T, db *gorm.DB, reviewID, want string) {
	t.Helper()
	var review Review
	if err := db.First(&review, "id = ?", reviewID).Error; err != nil || review.Conclusion == nil || *review.Conclusion != want {
		t.Fatalf("conclusion=%v want=%s err=%v", review.Conclusion, want, err)
	}
}
