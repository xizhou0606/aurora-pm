package designreviewgorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func TestRecheckCompletionMaterializesOneToOneLineage(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 2, 0, 0, 0, time.UTC)
	rootReviewID := "10000000-0000-4000-8000-000000000001"
	childReviewID := "10000000-0000-4000-8000-000000000002"
	differenceID := "20000000-0000-4000-8000-000000000001"
	lineageID := "30000000-0000-4000-8000-000000000001"
	jobID := "40000000-0000-4000-8000-000000000001"
	region := &designreview.Region{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}
	designAnchor := &designreview.IdentityAnchor{Kind: "figma", StableID: "42:7", Region: *region}
	implementationAnchor := &designreview.IdentityAnchor{Kind: "implementation", StableID: "stable-button", Region: *region}
	seedReview := func(review Review) {
		validReviewRecord(&review)
		if err := db.Create(&review).Error; err != nil {
			t.Fatal(err)
		}
	}
	seedReview(Review{ID: rootReviewID, WorkspaceID: 7, ChainRootID: &rootReviewID, RoundNumber: 1, CreatedByUserID: 9, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now})
	seedReview(Review{ID: childReviewID, WorkspaceID: 7, ChainRootID: &rootReviewID, SourceReviewID: &rootReviewID, RoundNumber: 2, CreatedByUserID: 9, ProjectSetIDSnapshot: 11, Status: "analyzing", CreatedAt: now, UpdatedAt: now})
	if err := db.Create(&DifferenceLineage{ID: lineageID, WorkspaceID: 7, ChainRootReviewID: rootReviewID, VerificationStatus: "pending", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Difference{ID: differenceID, WorkspaceID: 7, ReviewID: rootReviewID, LineageID: &lineageID, LineageRootDifferenceID: &differenceID, Ordinal: 1, Source: "ai", AnalysisKind: "full", Category: "layout", Severity: "major", Title: "间距错误", Description: "间距不一致", Recommendation: "调整间距", Confidence: 0.95, VerificationStatus: "pending", ResolutionStatus: "unresolved", DesignIdentityAnchor: jsonBytes(designAnchor), ImplementationIdentityAnchor: jsonBytes(implementationAnchor), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AnalysisJob{ID: jobID, WorkspaceID: 7, ReviewID: childReviewID, Status: "running", AvailableAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	snapshot := designreview.RecheckSnapshot{Version: "1.0", SourceReviewID: rootReviewID, ChainRootReviewID: rootReviewID,
		Items:    []designreview.RecheckHistoryItem{{HistoricalDifferenceID: differenceID, LatestDifferenceID: differenceID, LineageRootDifferenceID: differenceID}},
		Lineages: []designreview.RecheckLineage{{LineageID: lineageID, LatestDifferenceID: differenceID, LineageRootDifferenceID: differenceID, VerificationStatus: "pending", Category: "layout", Title: "间距错误", DesignRegion: region, ImplementationRegion: region, DesignAnchor: designAnchor, ImplementationAnchor: implementationAnchor}}}
	candidateID := "current-1"
	analysis := designreview.RecheckAnalysis{Summary: "复查完成", CoverageComplete: true,
		CurrentDifferences:      []designreview.RecheckCandidate{{CandidateID: candidateID, DiscoverySource: "full_page", Category: "layout", Severity: "major", Title: "间距错误", Description: "仍有偏差", Recommendation: "调整", Confidence: 0.96, DesignRegion: region, ImplementationRegion: region, EvidenceNotes: "双侧可见", DesignAnchor: designAnchor, ImplementationAnchor: implementationAnchor}},
		HistoricalVerifications: []designreview.HistoricalVerificationResult{{HistoricalDifferenceID: differenceID, Status: "still_present", CurrentDifferenceCandidateID: &candidateID, Confidence: 0.96, Reason: "仍存在", EvidenceSummary: "位置一致", CurrentDesignRegion: region, CurrentImplementationRegion: region}},
		LineageMatches:          []designreview.LineageMatchSuggestion{{CandidateID: candidateID, LineageID: &lineageID, Confidence: 0.96, Reason: "稳定匹配"}}}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now.Add(time.Minute) }
	job := designreview.ClaimedAnalysisJob{ID: jobID, WorkspaceID: 7, ReviewID: childReviewID}
	metadata := designreview.AnalysisRunMetadata{Model: "vision-model", ReasoningEffort: "high", PromptVersion: "design-recheck-v1", ResponseID: "response-1", DurationMS: 2400}
	if err := repository.CompleteRecheckJob(context.Background(), job, snapshot, analysis, metadata); err != nil {
		t.Fatal(err)
	}
	var current Difference
	if err := db.Where("workspace_id = ? AND review_id = ?", 7, childReviewID).First(&current).Error; err != nil {
		t.Fatal(err)
	}
	if current.LineageID == nil || *current.LineageID != lineageID || current.PreviousDifferenceID == nil || *current.PreviousDifferenceID != differenceID {
		t.Fatalf("lineage not inherited: %+v", current)
	}
	if len(current.DesignIdentityAnchor) == 0 || len(current.ImplementationIdentityAnchor) == 0 {
		t.Fatalf("identity anchors were not persisted: %+v", current)
	}
	var verification RecheckVerification
	if err := db.Where("workspace_id = ? AND review_id = ?", 7, childReviewID).First(&verification).Error; err != nil {
		t.Fatal(err)
	}
	if verification.CurrentDifferenceID == nil || *verification.CurrentDifferenceID != current.ID {
		t.Fatalf("verification current=%v, want %s", verification.CurrentDifferenceID, current.ID)
	}
	var completed Review
	if err := db.Where("id = ?", childReviewID).First(&completed).Error; err != nil {
		t.Fatal(err)
	}
	if completed.Model == nil || *completed.Model != metadata.Model || completed.ReasoningEffort == nil || *completed.ReasoningEffort != metadata.ReasoningEffort || completed.PromptVersion == nil || *completed.PromptVersion != metadata.PromptVersion || completed.ResponseID == nil || *completed.ResponseID != metadata.ResponseID || completed.AnalysisDurationMS == nil || *completed.AnalysisDurationMS != metadata.DurationMS {
		t.Fatalf("analysis metadata not persisted: %+v", completed)
	}
	if err := repository.CompleteRecheckJob(context.Background(), job, snapshot, analysis, metadata); err == nil {
		t.Fatal("duplicate recheck result was accepted")
	}
}

func TestBuildRecheckSnapshotScopesWorkspace(t *testing.T) {
	db := openRecheckDB(t)
	root := "50000000-0000-4000-8000-000000000001"
	child := "50000000-0000-4000-8000-000000000002"
	now := time.Now().UTC()
	for _, row := range []Review{
		{ID: root, WorkspaceID: 1, ChainRootID: &root, RoundNumber: 1, Status: "completed", CreatedAt: now, UpdatedAt: now},
		{ID: child, WorkspaceID: 1, ChainRootID: &root, SourceReviewID: &root, RoundNumber: 2, Status: "draft", CreatedAt: now, UpdatedAt: now},
	} {
		validReviewRecord(&row)
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := NewRepository(db)
	if _, err := repository.BuildRecheckSnapshot(context.Background(), 2, child); err == nil {
		t.Fatal("cross-workspace snapshot read was accepted")
	}
}

func validReviewRecord(row *Review) {
	row.ImplementationKind = "web"
	row.ImplementationTarget = []byte(`{"kind":"web","original_url":"https://example.test","actual_url":"https://example.test"}`)
	row.CaptureWarnings = []byte("[]")
	row.AnalysisScope = []byte(`{"enabled":false}`)
	row.MobileTopbar = []byte(`{"enabled":false}`)
	row.DOMNormalization = []byte(`{"enabled":false}`)
	row.ViewportWidth, row.ViewportHeight = 1440, 900
}

func openRecheckDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := []any{&Review{}, &Asset{}, &DifferenceLineage{}, &Difference{}, &LineageEvent{}, &RecheckMatch{}, &RecheckVerification{}, &ResolutionEvent{}, &AnalysisJob{}, &DeepJob{}, &ObjectCleanupJob{}, &auditgorm.AuditLog{}}
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	return db
}
