package aiworker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

// With worker concurrency above one, several queued runs are generated in
// parallel (overlapping runtime requests), each run is processed exactly
// once, and the whole batch drains without manual polling.
func TestRunProcessesQueueConcurrently(t *testing.T) {
	const runCount = 3
	var inFlight, maxInFlight, completed atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		current := inFlight.Add(1)
		for {
			observed := maxInFlight.Load()
			if current <= observed || maxInFlight.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"并发拆分","changes":[],"items":[],"risks":[],"humanReviewRequired":true},"usage":{"inputTokens":5,"outputTokens":5}}}`))
		inFlight.Add(-1)
		completed.Add(1)
	}))
	defer server.Close()

	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "file:worker_concurrency_test?mode=memory&cache=shared",
		AI: config.AI{
			Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: "worker-test-secret-at-least-32-characters",
			Model: "test-model", RequestTimeout: 5 * time.Second, WorkerConcurrency: runCount,
		},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < runCount; index++ {
		run := aigorm.AIRun{
			WorkspaceID: 1, Capability: "requirement.acceptance_criteria", SubjectType: "requirement",
			SubjectID: uint(10 + index), SubjectVersion: 1, Status: "queued",
			Model: cfg.AI.Model, InputJSON: `{"subject":{"title":"并发"}}`, RequestedByUser: 1,
		}
		if err := db.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for completed.Load() < runCount {
			time.Sleep(20 * time.Millisecond)
		}
		// Give the worker a beat to finish post-processing, then stop it.
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	New(db, cfg, fakeReader{}).Run(ctx, 10*time.Millisecond, cfg.AI.WorkerConcurrency)

	if maxInFlight.Load() < 2 {
		t.Fatalf("max concurrent generations=%d, want >= 2", maxInFlight.Load())
	}
	var rows []aigorm.AIRun
	if err := db.Where("capability = ?", "requirement.acceptance_criteria").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(rows)
	for _, row := range rows {
		if row.Status != "completed" {
			t.Fatalf("run %d status=%s rows=%s", row.ID, row.Status, encoded)
		}
	}
	if completed.Load() != runCount {
		t.Fatalf("runtime calls=%d want=%d", completed.Load(), runCount)
	}
}
