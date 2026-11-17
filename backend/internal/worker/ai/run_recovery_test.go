package aiworker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

func TestRecoverStaleAIRunsRequeuesOnlyExpiredClaims(t *testing.T) {
	now := time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)
	requestTimeout := 2 * time.Minute
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:worker_run_recovery_%d?mode=memory&cache=shared", time.Now().UnixNano()),
		AI: config.AI{
			Enabled:        true,
			RequestTimeout: requestTimeout,
		},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	cutoff := now.Add(-(requestTimeout + staleAIRunGrace))
	expiredStartedAt := cutoff.Add(-time.Second)
	boundaryStartedAt := cutoff
	freshStartedAt := cutoff.Add(time.Second)
	oldCompletedAt := now.Add(-time.Hour)
	runs := []aigorm.AIRun{
		{WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 1, SubjectVersion: 1, Status: "running", Model: "test", InputJSON: `{}`, RequestedByUser: 1, StartedAt: &expiredStartedAt, CompletedAt: &oldCompletedAt, ErrorMessage: "interrupted"},
		{WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 2, SubjectVersion: 1, Status: "running", Model: "test", InputJSON: `{}`, RequestedByUser: 1, StartedAt: &boundaryStartedAt},
		{WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 3, SubjectVersion: 1, Status: "running", Model: "test", InputJSON: `{}`, RequestedByUser: 1, StartedAt: &freshStartedAt},
		{WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 4, SubjectVersion: 1, Status: "running", Model: "test", InputJSON: `{}`, RequestedByUser: 1},
	}
	if err := db.Create(&runs).Error; err != nil {
		t.Fatal(err)
	}

	worker := New(db, cfg, nil)
	worker.now = func() time.Time { return now }
	reclaimed, runErr := worker.recoverStaleAIRuns()
	if runErr != nil || !reclaimed {
		t.Fatalf("reclaimed=%v err=%v", reclaimed, runErr)
	}

	var recoveredRun aigorm.AIRun
	if err := db.First(&recoveredRun, runs[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if recoveredRun.Status != "queued" || recoveredRun.StartedAt == nil || !recoveredRun.StartedAt.Equal(expiredStartedAt) || recoveredRun.CompletedAt != nil || recoveredRun.ErrorMessage != "" {
		t.Fatalf("recovered run=%+v", recoveredRun)
	}
	for _, untouched := range runs[1:] {
		var current aigorm.AIRun
		if err := db.First(&current, untouched.ID).Error; err != nil {
			t.Fatal(err)
		}
		if current.Status != "running" || current.CompletedAt != nil || current.ErrorMessage != "" {
			t.Fatalf("fresh run was modified: %+v", current)
		}
	}

	reclaimed, runErr = worker.recoverStaleAIRuns()
	if runErr != nil || reclaimed {
		t.Fatalf("second pass reclaimed=%v err=%v", reclaimed, runErr)
	}
}

func TestRunOnceClaimsRequeuedStaleAIRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"reclaimed result","changes":[],"items":[],"risks":[],"humanReviewRequired":true},"usage":{}}}`))
	}))
	defer server.Close()

	now := time.Now().UTC()
	requestTimeout := 2 * time.Minute
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:worker_reclaim_%d?mode=memory&cache=shared", time.Now().UnixNano()),
		AI: config.AI{
			Enabled:        true,
			RuntimeBaseURL: server.URL,
			InternalSecret: "worker-test-secret-at-least-32-characters",
			Model:          "test",
			RequestTimeout: requestTimeout,
		},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	staleStartedAt := now.Add(-(requestTimeout + staleAIRunGrace + time.Second))
	run := aigorm.AIRun{
		WorkspaceID: 1, Capability: "workflow.generate", SubjectType: "workflow_template", SubjectID: 1,
		SubjectVersion: 1, Status: "running", Model: cfg.AI.Model, InputJSON: `{}`, RequestedByUser: 1,
		StartedAt: &staleStartedAt,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	worker := New(db, cfg, nil)
	processed, runErr := worker.RunOnce(context.Background())
	if runErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}

	var current aigorm.AIRun
	if err := db.First(&current, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "completed" || current.StartedAt == nil || !current.StartedAt.After(staleStartedAt) || current.CompletedAt == nil || current.CompletedAt.Before(current.StartedAt.Add(-time.Second)) {
		t.Fatalf("reclaimed run=%+v", current)
	}
	var artifactCount int64
	if err := db.Model(&aigorm.AIArtifact{}).Where("run_id = ?", run.ID).Count(&artifactCount).Error; err != nil {
		t.Fatal(err)
	}
	if artifactCount != 1 {
		t.Fatalf("artifact count=%d", artifactCount)
	}
}

func TestReclaimedAIRunRejectsLateWorkerCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"late result","changes":[],"items":[],"risks":[],"humanReviewRequired":true},"usage":{}}}`))
	}))
	defer server.Close()

	now := time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)
	requestTimeout := 2 * time.Minute
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:worker_late_completion_%d?mode=memory&cache=shared", time.Now().UnixNano()),
		AI: config.AI{
			Enabled:        true,
			RuntimeBaseURL: server.URL,
			InternalSecret: "worker-test-secret-at-least-32-characters",
			Model:          "test",
			RequestTimeout: requestTimeout,
		},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	startedAt := now.Add(-(requestTimeout + staleAIRunGrace + time.Second))
	run := aigorm.AIRun{
		WorkspaceID: 1, Capability: "workflow.generate", SubjectType: "workflow_template", SubjectID: 1,
		SubjectVersion: 1, Status: "running", Model: cfg.AI.Model, InputJSON: `{}`, RequestedByUser: 1,
		StartedAt: &startedAt,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}

	recoveryWorker := New(db, cfg, nil)
	recoveryWorker.now = func() time.Time { return now }
	if recovered, err := recoveryWorker.recoverStaleAIRuns(); err != nil || !recovered {
		t.Fatalf("recovery recovered=%v err=%v", recovered, err)
	}

	var requeued aigorm.AIRun
	if err := db.First(&requeued, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if requeued.Status != "queued" || requeued.StartedAt == nil || !requeued.StartedAt.Equal(startedAt) {
		t.Fatalf("recovered run lost the old fence: %+v", requeued)
	}
	newClaimAt := nextAIRunClaimTime(now, requeued.UpdatedAt, requeued.StartedAt)
	claimed := queuedRunScope(db.Model(&aigorm.AIRun{}), &requeued).
		Updates(map[string]any{"status": "running", "started_at": newClaimAt})
	if claimed.Error != nil || claimed.RowsAffected != 1 {
		t.Fatalf("new claim rows=%d err=%v", claimed.RowsAffected, claimed.Error)
	}
	if !newClaimAt.After(startedAt) {
		t.Fatalf("replacement fence=%s did not advance past old fence=%s", newClaimAt, startedAt)
	}

	lateWorker := New(db, cfg, nil)
	lateOutput := []byte(`{"summary":"late result","changes":[],"items":[],"risks":[],"humanReviewRequired":true}`)
	if err := lateWorker.completeRunClaim(&run, "late result", lateOutput, []byte(`{}`), now, nil); !errors.Is(err, errRunNotRunning) {
		t.Fatalf("late completion error=%v", err)
	}
	var current aigorm.AIRun
	if err := db.First(&current, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "running" || current.StartedAt == nil || !current.StartedAt.Equal(newClaimAt) || current.ErrorMessage != "" || current.CompletedAt != nil {
		t.Fatalf("late worker overwrote recovered run: %+v", current)
	}
	if err := lateWorker.failRunClaim(&run, errors.New("late worker failure")); !errors.Is(err, errRunNotRunning) {
		t.Fatalf("late failure error=%v", err)
	}
	if err := db.First(&current, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "running" || current.StartedAt == nil || !current.StartedAt.Equal(newClaimAt) || current.ErrorMessage != "" || current.CompletedAt != nil {
		t.Fatalf("late worker failure overwrote new claim: %+v", current)
	}
	var artifactCount int64
	if err := db.Model(&aigorm.AIArtifact{}).Where("run_id = ?", run.ID).Count(&artifactCount).Error; err != nil {
		t.Fatal(err)
	}
	if artifactCount != 0 {
		t.Fatalf("late worker persisted %d artifacts", artifactCount)
	}
}

func TestAIRunClaimTimeRoundTripsAtMySQLPrecision(t *testing.T) {
	claim := normalizedAIRunClaimTime(time.Date(2026, time.August, 11, 12, 0, 0, 987654321, time.FixedZone("UTC+8", 8*60*60)))
	if claim.Location() != time.UTC || claim.Nanosecond() != 987000000 {
		t.Fatalf("claim=%s nanoseconds=%d", claim, claim.Nanosecond())
	}
}

func TestNextAIRunClaimTimeAdvancesPastImmediateRetry(t *testing.T) {
	queuedAt := time.Date(2026, time.August, 11, 12, 0, 0, 987000000, time.UTC)
	previous := queuedAt.Add(time.Millisecond)
	claim := nextAIRunClaimTime(queuedAt, queuedAt, &previous)
	if !claim.Equal(previous.Add(time.Millisecond)) {
		t.Fatalf("claim=%s want=%s", claim, previous.Add(time.Millisecond))
	}
}

func TestCompletionTimeDoesNotPrecedeClaimFence(t *testing.T) {
	startedAt := time.Date(2026, time.August, 11, 12, 0, 0, 1_000_000, time.UTC)
	run := aigorm.AIRun{StartedAt: &startedAt}

	if got := completionTimeAtLeastClaim(&run, startedAt.Add(-time.Nanosecond)); !got.Equal(startedAt) {
		t.Fatalf("completion=%s want claim=%s", got, startedAt)
	}

	later := startedAt.Add(time.Nanosecond)
	if got := completionTimeAtLeastClaim(&run, later); !got.Equal(later) {
		t.Fatalf("completion=%s want later timestamp=%s", got, later)
	}
}

func TestQueuedClaimFenceRejectsLateWorkerAfterSameMillisecondRetry(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:worker_claim_aba_%d?mode=memory&cache=shared", time.Now().UnixNano()),
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	base := normalizedAIRunClaimTime(time.Now().UTC())
	oldRun := aigorm.AIRun{
		WorkspaceID: 1, Capability: "workflow.generate", SubjectType: "workflow_template", SubjectID: 1,
		SubjectVersion: 1, Status: "running", Model: "test", InputJSON: `{}`, RequestedByUser: 1,
		StartedAt: &base,
	}
	if err := db.Create(&oldRun).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&aigorm.AIRun{}).Where("id = ? AND status = ?", oldRun.ID, "running").
		Update("status", "queued").Error; err != nil {
		t.Fatal(err)
	}

	var queued aigorm.AIRun
	if err := db.First(&queued, oldRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if queued.StartedAt == nil || !queued.StartedAt.Equal(base) {
		t.Fatalf("queued retry lost prior fence: %+v", queued)
	}
	newClaimAt := nextAIRunClaimTime(base, queued.UpdatedAt, queued.StartedAt)
	if !newClaimAt.After(base) {
		t.Fatalf("same-clock replacement fence=%s did not advance past %s", newClaimAt, base)
	}
	claimed := queuedRunScope(db.Model(&aigorm.AIRun{}), &queued).
		Updates(map[string]any{"status": "running", "started_at": newClaimAt})
	if claimed.Error != nil || claimed.RowsAffected != 1 {
		t.Fatalf("claim rows=%d err=%v", claimed.RowsAffected, claimed.Error)
	}

	oldOutput := []byte(`{"summary":"old result","changes":[],"items":[],"risks":[],"humanReviewRequired":true}`)
	worker := New(db, cfg, nil)
	if err := worker.completeRunClaim(&oldRun, "old result", oldOutput, []byte(`{}`), base, nil); !errors.Is(err, errRunNotRunning) {
		t.Fatalf("late completion error=%v", err)
	}
	if err := worker.failRunClaim(&oldRun, errors.New("old worker failed")); !errors.Is(err, errRunNotRunning) {
		t.Fatalf("late failure error=%v", err)
	}

	var current aigorm.AIRun
	if err := db.First(&current, oldRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "running" || current.StartedAt == nil || !current.StartedAt.Equal(newClaimAt) {
		t.Fatalf("late worker changed replacement run: %+v", current)
	}
	var artifactCount int64
	if err := db.Model(&aigorm.AIArtifact{}).Where("run_id = ?", oldRun.ID).Count(&artifactCount).Error; err != nil {
		t.Fatal(err)
	}
	if artifactCount != 0 {
		t.Fatalf("late worker persisted %d artifacts", artifactCount)
	}

	newRun := current
	newOutput := []byte(`{"summary":"replacement result","changes":[],"items":[],"risks":[],"humanReviewRequired":true}`)
	if err := worker.completeRunClaim(&newRun, "replacement result", newOutput, []byte(`{}`), newClaimAt, nil); err != nil {
		t.Fatalf("replacement completion error=%v", err)
	}
	if err := db.First(&current, oldRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "completed" || current.OutputJSON != string(newOutput) {
		t.Fatalf("replacement result was not persisted: %+v", current)
	}
}
