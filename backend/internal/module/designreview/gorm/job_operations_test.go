package designreviewgorm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func TestAnalysisJobLeaseRecoveryBackoffAndRetryLimit(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 4, 0, 0, 0, time.UTC)
	reviewID := "60000000-0000-4000-8000-000000000001"
	jobID := "70000000-0000-4000-8000-000000000001"
	review := Review{ID: reviewID, WorkspaceID: 7, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "queued", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AnalysisJob{ID: jobID, WorkspaceID: 7, ReviewID: reviewID, Status: "pending", AvailableAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	current := now
	repository.now = func() time.Time { return current }

	first, err := repository.ClaimJob(context.Background(), "worker-a", time.Minute)
	if err != nil || first == nil || first.Attempts != 1 {
		t.Fatalf("first claim=%+v err=%v", first, err)
	}
	if duplicate, err := repository.ClaimJob(context.Background(), "worker-b", time.Minute); err != nil || duplicate != nil {
		t.Fatalf("active lease was claimed twice: claim=%+v err=%v", duplicate, err)
	}

	current = now.Add(2 * time.Minute)
	second, err := repository.ClaimJob(context.Background(), "worker-b", time.Minute)
	if err != nil || second == nil || second.Attempts != 2 {
		t.Fatalf("expired lease was not recovered: claim=%+v err=%v", second, err)
	}
	if err := repository.FailJob(context.Background(), *second, errors.New("temporary"), true); err != nil {
		t.Fatal(err)
	}
	current = current.Add(3 * time.Second)
	if early, err := repository.ClaimJob(context.Background(), "worker-c", time.Minute); err != nil || early != nil {
		t.Fatalf("backoff was ignored: claim=%+v err=%v", early, err)
	}
	current = current.Add(time.Second)
	third, err := repository.ClaimJob(context.Background(), "worker-c", time.Minute)
	if err != nil || third == nil || third.Attempts != 3 {
		t.Fatalf("third claim=%+v err=%v", third, err)
	}
	if err := repository.FailJob(context.Background(), *third, errors.New("temporary"), true); err != nil {
		t.Fatal(err)
	}
	var storedJob AnalysisJob
	if err := db.First(&storedJob, "id = ?", jobID).Error; err != nil || storedJob.Status != "failed" || storedJob.Attempts != 3 {
		t.Fatalf("job did not stop after three attempts: job=%+v err=%v", storedJob, err)
	}
	var storedReview Review
	if err := db.First(&storedReview, "id = ?", reviewID).Error; err != nil || storedReview.Status != "failed" || storedReview.LastErrorStage == nil || *storedReview.LastErrorStage != "analysis" {
		t.Fatalf("review failure was not persisted: review=%+v err=%v", storedReview, err)
	}
}

func TestAnalysisJobPermanentFailureDoesNotRetry(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 5, 0, 0, 0, time.UTC)
	reviewID := "60000000-0000-4000-8000-000000000002"
	jobID := "70000000-0000-4000-8000-000000000002"
	review := Review{ID: reviewID, WorkspaceID: 8, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "queued", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AnalysisJob{ID: jobID, WorkspaceID: 8, ReviewID: reviewID, Status: "pending", AvailableAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now }
	claimed, err := repository.ClaimJob(context.Background(), "worker-a", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim=%+v err=%v", claimed, err)
	}
	if err := repository.FailJob(context.Background(), *claimed, designreview.ErrInvalid, false); err != nil {
		t.Fatal(err)
	}
	var stored AnalysisJob
	if err := db.First(&stored, "id = ?", jobID).Error; err != nil || stored.Status != "failed" || stored.Attempts != 1 {
		t.Fatalf("permanent failure was retried: job=%+v err=%v", stored, err)
	}
}

func TestClaimJobExpiresCrashedThirdAttempt(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 6, 0, 0, 0, time.UTC)
	reviewID := "60000000-0000-4000-8000-000000000003"
	jobID := "70000000-0000-4000-8000-000000000003"
	leaseUntil := now.Add(-time.Second)
	owner := "dead-worker"
	review := Review{ID: reviewID, WorkspaceID: 9, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "analyzing", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AnalysisJob{ID: jobID, WorkspaceID: 9, ReviewID: reviewID, Status: "running", Attempts: 3, AvailableAt: now, LeaseOwner: &owner, LeaseUntil: &leaseUntil, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now }
	claimed, err := repository.ClaimJob(context.Background(), "worker-a", time.Minute)
	if err != nil || claimed != nil {
		t.Fatalf("exhausted job was reclaimed: claim=%+v err=%v", claimed, err)
	}
	var stored AnalysisJob
	if err := db.First(&stored, "id = ?", jobID).Error; err != nil || stored.Status != "failed" || stored.LeaseOwner != nil || stored.LeaseUntil != nil {
		t.Fatalf("exhausted lease was not finalized: job=%+v err=%v", stored, err)
	}
}

func TestAnalysisFailureDoesNotOverrideAbandonedReview(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 7, 0, 0, 0, time.UTC)
	reviewID := "60000000-0000-4000-8000-000000000004"
	jobID := "70000000-0000-4000-8000-000000000004"
	review := Review{ID: reviewID, WorkspaceID: 10, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "abandoned", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AnalysisJob{ID: jobID, WorkspaceID: 10, ReviewID: reviewID, Status: "cancelled", AvailableAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now }
	job := ClaimedJob{ID: jobID, WorkspaceID: 10, ReviewID: reviewID, Kind: "full", Attempts: 1}
	if err := repository.FailJob(context.Background(), job, errors.New("late worker failure"), false); err != nil {
		t.Fatal(err)
	}
	var stored Review
	if err := db.First(&stored, "id = ?", reviewID).Error; err != nil || stored.Status != "abandoned" {
		t.Fatalf("review=%+v err=%v", stored, err)
	}
}

func TestQueueAnalysisAcceptsDraftAndRejectsTerminalRounds(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	repository.now = func() time.Time { return now }
	for index, status := range []string{"draft", "abandoned", "completed"} {
		reviewID := fmt.Sprintf("61000000-0000-4000-8000-%012d", index+1)
		review := Review{ID: reviewID, WorkspaceID: 11, ChainRootID: &reviewID, RoundNumber: index + 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: status, CreatedAt: now, UpdatedAt: now}
		validReviewRecord(&review)
		if err := db.Create(&review).Error; err != nil {
			t.Fatal(err)
		}
		err := repository.QueueAnalysis(context.Background(), 11, reviewID, 42)
		if status == "draft" && err != nil {
			t.Fatalf("draft queue err=%v", err)
		}
		if status != "draft" && !errors.Is(err, designreview.ErrConflict) {
			t.Fatalf("status=%s err=%v", status, err)
		}
	}
}

func TestCompleteDeepJobPersistsRuntimeMetadata(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	reviewID := "62000000-0000-4000-8000-000000000001"
	jobID := "72000000-0000-4000-8000-000000000001"
	review := Review{ID: reviewID, WorkspaceID: 12, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&DeepJob{ID: jobID, WorkspaceID: 12, ReviewID: reviewID, CreatedByUserID: 42, Status: "running", DesignRegion: []byte(`{"x":0,"y":0,"width":1,"height":1}`), ImplementationRegion: []byte(`{"x":0,"y":0,"width":1,"height":1}`), AvailableAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now.Add(time.Minute) }
	metadata := designreview.AnalysisRunMetadata{Model: "vision-model", ReasoningEffort: "medium", PromptVersion: "design-review-v1", ResponseID: "response-deep", DurationMS: 1800}
	job := ClaimedJob{ID: jobID, WorkspaceID: 12, ReviewID: reviewID, Kind: "deep", Attempts: 1}
	if err := repository.CompleteJob(context.Background(), job, "深度走查完成", true, nil, metadata); err != nil {
		t.Fatal(err)
	}
	var stored DeepJob
	if err := db.First(&stored, "id = ?", jobID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "succeeded" || stored.Model == nil || *stored.Model != metadata.Model || stored.ReasoningEffort == nil || *stored.ReasoningEffort != metadata.ReasoningEffort || stored.PromptVersion == nil || *stored.PromptVersion != metadata.PromptVersion || stored.ResponseID == nil || *stored.ResponseID != metadata.ResponseID || stored.DurationMS == nil || *stored.DurationMS != metadata.DurationMS {
		t.Fatalf("deep metadata not persisted: %+v", stored)
	}
}

func TestCompleteFullJobPersistsRuntimeMetadata(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 9, 30, 0, 0, time.UTC)
	reviewID := "62000000-0000-4000-8000-000000000002"
	jobID := "72000000-0000-4000-8000-000000000002"
	review := Review{ID: reviewID, WorkspaceID: 13, ChainRootID: &reviewID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "analyzing", CreatedAt: now, UpdatedAt: now}
	validReviewRecord(&review)
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AnalysisJob{ID: jobID, WorkspaceID: 13, ReviewID: reviewID, Status: "running", AvailableAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	repository.now = func() time.Time { return now.Add(time.Minute) }
	metadata := designreview.AnalysisRunMetadata{Model: "vision-model", ReasoningEffort: "high", PromptVersion: "design-review-v1", ResponseID: "response-full", DurationMS: 2200}
	job := ClaimedJob{ID: jobID, WorkspaceID: 13, ReviewID: reviewID, Kind: "full", Attempts: 1}
	if err := repository.CompleteJob(context.Background(), job, "完整走查完成", true, nil, metadata); err != nil {
		t.Fatal(err)
	}
	var stored Review
	if err := db.First(&stored, "id = ?", reviewID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "completed" || stored.Model == nil || *stored.Model != metadata.Model || stored.ReasoningEffort == nil || *stored.ReasoningEffort != metadata.ReasoningEffort || stored.PromptVersion == nil || *stored.PromptVersion != metadata.PromptVersion || stored.ResponseID == nil || *stored.ResponseID != metadata.ResponseID || stored.AnalysisDurationMS == nil || *stored.AnalysisDurationMS != metadata.DurationMS {
		t.Fatalf("full metadata not persisted: %+v", stored)
	}
	detail, err := repository.GetReview(context.Background(), 13, reviewID)
	if err != nil || detail.AnalysisDurationMS == nil || *detail.AnalysisDurationMS != metadata.DurationMS || detail.AnalysisAttempts != stored.AnalysisAttempts {
		t.Fatalf("detail metadata=%+v err=%v", detail, err)
	}
}
