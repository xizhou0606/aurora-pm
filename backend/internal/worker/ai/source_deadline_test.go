package aiworker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

type phaseBlockingFeishuReader struct {
	mu        sync.Mutex
	document  feishu.Document
	blockCall int
	calls     int
}

func (r *phaseBlockingFeishuReader) Read(ctx context.Context, _ string) (feishu.Document, error) {
	r.mu.Lock()
	r.calls++
	call := r.calls
	document := r.document
	r.mu.Unlock()
	if call == r.blockCall {
		<-ctx.Done()
		return feishu.Document{}, ctx.Err()
	}
	return document, nil
}

func (r *phaseBlockingFeishuReader) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func TestQASourceFreshnessTimeoutPreservesRuntimeMargin(t *testing.T) {
	if qaSourceFreshnessTimeout <= 0 || qaSourceFreshnessTimeout >= 30*time.Second {
		t.Fatalf("QA source freshness timeout=%s, want a positive duration below 30s", qaSourceFreshnessTimeout)
	}
}

func TestWorkerReleasesBlockedFeishuReadsAndProcessesNextRun(t *testing.T) {
	tests := []struct {
		name                 string
		blockCall            int
		sourceLoadTimeout    time.Duration
		freshnessTimeout     time.Duration
		wantError            string
		wantQARuntimeCalls   int32
		wantFirstReaderCalls int
	}{
		{
			name: "initial source load deadline", blockCall: 1,
			sourceLoadTimeout: 30 * time.Millisecond, freshnessTimeout: 200 * time.Millisecond,
			wantError: "读取飞书需求来源超时", wantQARuntimeCalls: 0, wantFirstReaderCalls: 1,
		},
		{
			name: "post generation freshness deadline", blockCall: 2,
			sourceLoadTimeout: 200 * time.Millisecond, freshnessTimeout: 30 * time.Millisecond,
			wantError: "生成后复核飞书需求来源超时", wantQARuntimeCalls: 1, wantFirstReaderCalls: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var qaRuntimeCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				defer request.Body.Close()
				var payload struct {
					Capability string `json:"capability"`
				}
				if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
					t.Error(err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				if payload.Capability == "qa.test_cases" {
					qaRuntimeCalls.Add(1)
					_, _ = writer.Write([]byte(validQATestCaseRuntimeEnvelope))
					return
				}
				_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"验收标准","changes":[{"field":"acceptance_criteria"}]},"usage":{}}}`))
			}))
			defer server.Close()

			cfg := config.Config{
				DatabaseDriver: "sqlite",
				DatabaseDSN:    "file:worker_source_deadline_" + strings.ReplaceAll(test.name, " ", "_") + "?mode=memory&cache=shared",
				AI: config.AI{
					Enabled: true, RuntimeBaseURL: server.URL,
					InternalSecret: "worker-test-secret-at-least-32-characters",
					Model:          "test-model", RequestTimeout: 2 * time.Second,
				},
				Feishu: config.Feishu{Enabled: true},
			}
			db, err := database.Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := database.Migrate(db); err != nil {
				t.Fatal(err)
			}
			qaRun := aigorm.AIRun{
				WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 2,
				SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
				InputJSON: `{"subject":{"title":"结算页"}}`, RequestedByUser: 1,
			}
			if err := db.Create(&qaRun).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&aigorm.AIRunSource{
				WorkspaceID: 1, RunID: qaRun.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token",
			}).Error; err != nil {
				t.Fatal(err)
			}
			reader := &phaseBlockingFeishuReader{
				blockCall: test.blockCall,
				document: feishu.Document{
					URL: "https://acme.feishu.cn/docx/token", Revision: "7", Content: "完整正文", ContentHash: "hash-7",
				},
			}
			worker := New(db, cfg, reader)
			worker.sourceLoadTimeout = test.sourceLoadTimeout
			worker.sourceFreshnessTimeout = test.freshnessTimeout

			startedAt := time.Now()
			processed, runErr := worker.RunOnce(context.Background())
			if !processed || runErr == nil || !strings.Contains(runErr.Error(), test.wantError) {
				t.Fatalf("processed=%v err=%v", processed, runErr)
			}
			if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
				t.Fatalf("blocked source held worker for %s", elapsed)
			}
			if reader.callCount() != test.wantFirstReaderCalls {
				t.Fatalf("Feishu reads=%d, want %d", reader.callCount(), test.wantFirstReaderCalls)
			}
			if qaRuntimeCalls.Load() != test.wantQARuntimeCalls {
				t.Fatalf("QA runtime calls=%d, want %d", qaRuntimeCalls.Load(), test.wantQARuntimeCalls)
			}
			if err := db.First(&qaRun, qaRun.ID).Error; err != nil {
				t.Fatal(err)
			}
			if qaRun.Status != "failed" || !strings.Contains(qaRun.ErrorMessage, test.wantError) {
				t.Fatalf("QA run status=%s error=%q", qaRun.Status, qaRun.ErrorMessage)
			}

			followUp := aigorm.AIRun{
				WorkspaceID: 1, Capability: "requirement.acceptance_criteria", SubjectType: "requirement", SubjectID: 3,
				SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
				InputJSON: `{"subject":{"title":"后续需求"}}`, RequestedByUser: 1,
			}
			if err := db.Create(&followUp).Error; err != nil {
				t.Fatal(err)
			}
			processed, runErr = worker.RunOnce(context.Background())
			if !processed || runErr != nil {
				t.Fatalf("follow-up processed=%v err=%v", processed, runErr)
			}
			if err := db.First(&followUp, followUp.ID).Error; err != nil {
				t.Fatal(err)
			}
			if followUp.Status != "completed" {
				t.Fatalf("follow-up status=%s error=%q", followUp.Status, followUp.ErrorMessage)
			}
		})
	}
}
