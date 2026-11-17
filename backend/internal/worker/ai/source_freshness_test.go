package aiworker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

const validQATestCaseRuntimeEnvelope = `{"data":{"output":{"summary":"已生成用例","changes":[],"skillProvenance":{"name":"qa-test-case-design","repository":"https://github.com/QA19816-glitch/QA-agent","path":"codex-skills/skills/qa-test-case-design","commit":"d75606531f6ce414bbcd95fb203033ae86653852","packageSha256":"07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935","mode":"advanced","resources":["output-templates/structured-test-cases.md","prompts/advanced-test-case-design.md","references/case-writing-rules.md"]}},"usage":{}}}`

type changingFeishuReader struct {
	mu       sync.Mutex
	document feishu.Document
	err      error
	calls    int
}

type mappedFeishuReader struct {
	documents map[string]feishu.Document
	calls     []string
}

func (r *mappedFeishuReader) Read(_ context.Context, rawURL string) (feishu.Document, error) {
	r.calls = append(r.calls, rawURL)
	return r.documents[rawURL], nil
}

func (r *changingFeishuReader) Read(context.Context, string) (feishu.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.document, r.err
}

func (r *changingFeishuReader) set(document feishu.Document, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.document = document
	r.err = err
}

func (r *changingFeishuReader) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func TestQATestCaseRunFailsWhenFeishuSourceChangesWhileRuntimeIsBlocked(t *testing.T) {
	initial := feishu.Document{
		URL: "https://acme.feishu.cn/docx/token", Revision: "7", Title: "需求",
		Content: "生成前完整正文", ContentHash: "hash-before", BlockRefs: []string{"b1"},
	}
	tests := []struct {
		name      string
		document  feishu.Document
		readErr   error
		wantError string
	}{
		{name: "revision changed", document: feishu.Document{URL: initial.URL, Revision: "8", Content: initial.Content, ContentHash: initial.ContentHash}, wantError: "生成期间发生变化"},
		{name: "content hash changed", document: feishu.Document{URL: initial.URL, Revision: initial.Revision, Content: "生成期间修改后的正文", ContentHash: "hash-after"}, wantError: "生成期间发生变化"},
		{name: "second read failed", document: initial, readErr: context.DeadlineExceeded, wantError: "生成后复核飞书需求来源失败"},
		{name: "second read truncated", document: feishu.Document{URL: initial.URL, Revision: initial.Revision, Content: initial.Content, ContentHash: initial.ContentHash, ContentTruncated: true}, wantError: "超过读取上限"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestStarted := make(chan struct{})
			releaseRuntime := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				close(requestStarted)
				<-releaseRuntime
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(validQATestCaseRuntimeEnvelope))
			}))
			defer server.Close()

			cfg := config.Config{
				DatabaseDriver: "sqlite",
				DatabaseDSN:    "file:worker_source_freshness_" + strings.ReplaceAll(test.name, " ", "_") + "?mode=memory&cache=shared",
				AI: config.AI{
					Enabled: true, RuntimeBaseURL: server.URL,
					InternalSecret: "worker-test-secret-at-least-32-characters",
					Model:          "qwen3.6-max-preview", RequestTimeout: 2 * time.Second,
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
			run := aigorm.AIRun{
				WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 2,
				SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
				InputJSON: `{"subject":{"title":"结算页"},"test_case_delivery":{"instance_id":17,"node_id":"qa-node"}}`, RequestedByUser: 1,
			}
			if err := db.Create(&run).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&aigorm.AIRunSource{
				WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: initial.URL,
			}).Error; err != nil {
				t.Fatal(err)
			}

			reader := &changingFeishuReader{document: initial}
			result := make(chan error, 1)
			go func() {
				_, runErr := New(db, cfg, reader).RunOnce(context.Background())
				result <- runErr
			}()
			select {
			case <-requestStarted:
			case <-time.After(time.Second):
				t.Fatal("runtime request did not start")
			}
			reader.set(test.document, test.readErr)
			close(releaseRuntime)

			select {
			case runErr := <-result:
				if runErr == nil || !strings.Contains(runErr.Error(), test.wantError) {
					t.Fatalf("run error=%v, want %q", runErr, test.wantError)
				}
			case <-time.After(time.Second):
				t.Fatal("worker did not finish after runtime returned")
			}
			if reader.callCount() != 2 {
				t.Fatalf("Feishu reads=%d, want 2", reader.callCount())
			}
			if err := db.First(&run, run.ID).Error; err != nil {
				t.Fatal(err)
			}
			if run.Status != "failed" || !strings.Contains(run.ErrorMessage, test.wantError) || run.OutputJSON != "" {
				t.Fatalf("run status=%s error=%q output=%q", run.Status, run.ErrorMessage, run.OutputJSON)
			}
			if strings.TrimSpace(test.document.Content) != "" && strings.Contains(run.ErrorMessage, test.document.Content) {
				t.Fatal("freshness error exposed the re-read Feishu document body")
			}
			var artifactCount int64
			if err := db.Model(&aigorm.AIArtifact{}).Where("workspace_id = ? AND run_id = ?", run.WorkspaceID, run.ID).Count(&artifactCount).Error; err != nil {
				t.Fatal(err)
			}
			if artifactCount != 0 {
				t.Fatalf("stale-source run persisted %d AI artifacts", artifactCount)
			}
			var deliveredArtifactCount int64
			if err := db.Model(&workflowgorm.WorkflowNodeArtifact{}).Where("workspace_id = ? AND ai_run_id = ?", run.WorkspaceID, run.ID).Count(&deliveredArtifactCount).Error; err != nil {
				t.Fatal(err)
			}
			if deliveredArtifactCount != 0 {
				t.Fatalf("stale-source run delivered %d workflow artifacts", deliveredArtifactCount)
			}
		})
	}
}

func TestQATestCaseRunRejectsIncompleteFeishuSnapshotBeforeRuntime(t *testing.T) {
	tests := []struct {
		name     string
		document feishu.Document
	}{
		{
			name: "missing revision",
			document: feishu.Document{
				URL: "https://acme.feishu.cn/docx/token", Content: "完整正文", ContentHash: "content-hash",
			},
		},
		{
			name: "missing content hash",
			document: feishu.Document{
				URL: "https://acme.feishu.cn/docx/token", Revision: "7", Content: "完整正文",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstreamCalled := false
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				upstreamCalled = true
				writer.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			cfg := config.Config{
				DatabaseDriver: "sqlite",
				DatabaseDSN:    "file:worker_incomplete_source_snapshot_" + strings.ReplaceAll(test.name, " ", "_") + "?mode=memory&cache=shared",
				AI: config.AI{
					Enabled: true, RuntimeBaseURL: server.URL,
					InternalSecret: "worker-test-secret-at-least-32-characters",
					Model:          "test-model", RequestTimeout: time.Second,
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
			run := aigorm.AIRun{
				WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 2,
				SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
				InputJSON: `{"subject":{"title":"结算页"}}`, RequestedByUser: 1,
			}
			if err := db.Create(&run).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&aigorm.AIRunSource{
				WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: test.document.URL,
			}).Error; err != nil {
				t.Fatal(err)
			}
			reader := &changingFeishuReader{document: test.document}
			processed, runErr := New(db, cfg, reader).RunOnce(context.Background())
			if !processed || runErr == nil || !strings.Contains(runErr.Error(), "缺少版本或内容哈希") {
				t.Fatalf("processed=%v err=%v", processed, runErr)
			}
			if upstreamCalled {
				t.Fatal("runtime was called without a complete Feishu source snapshot")
			}
			if reader.callCount() != 1 {
				t.Fatalf("Feishu reads=%d, want 1", reader.callCount())
			}
			if err := db.First(&run, run.ID).Error; err != nil {
				t.Fatal(err)
			}
			if run.Status != "failed" || !strings.Contains(run.ErrorMessage, "缺少版本或内容哈希") {
				t.Fatalf("run status=%s error=%q", run.Status, run.ErrorMessage)
			}
		})
	}
}

func TestVerifyQATestCaseSourceFreshnessChecksEveryFeishuSource(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_all_source_freshness?mode=memory&cache=shared",
		Feishu: config.Feishu{Enabled: true},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{WorkspaceID: 1, Capability: "qa.test_cases", Status: "running", Model: "test-model", InputJSON: `{}`}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	urls := []string{
		"https://acme.feishu.cn/docx/one",
		"https://acme.feishu.cn/docx/two",
	}
	revisions := []string{"revision-1", "revision-2"}
	hashes := []string{"hash-1", "hash-2"}
	for index, rawURL := range urls {
		if err := db.Create(&aigorm.AIRunSource{
			WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: rawURL,
			Revision: revisions[index], ContentHash: hashes[index],
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	reader := &mappedFeishuReader{documents: map[string]feishu.Document{
		urls[0]: {Revision: "revision-1", Content: "正文一", ContentHash: "hash-1"},
		urls[1]: {Revision: "revision-2", Content: "正文二", ContentHash: "hash-2"},
	}}
	worker := New(db, cfg, reader)
	if err := worker.verifyQATestCaseSourceFreshness(context.Background(), &run); err != nil {
		t.Fatalf("unchanged sources rejected: %v", err)
	}
	if len(reader.calls) != len(urls) || reader.calls[0] != urls[0] || reader.calls[1] != urls[1] {
		t.Fatalf("Feishu source reads=%v, want %v", reader.calls, urls)
	}
}

func TestNonQARunDoesNotReReadFeishuSourceAfterRuntime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"验收标准","changes":[{"field":"acceptance_criteria"}]},"usage":{}}}`))
	}))
	defer server.Close()
	cfg := config.Config{
		DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_non_qa_source_freshness?mode=memory&cache=shared",
		AI: config.AI{
			Enabled: true, RuntimeBaseURL: server.URL,
			InternalSecret: "worker-test-secret-at-least-32-characters",
			Model:          "test-model", RequestTimeout: time.Second,
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
	run := aigorm.AIRun{
		WorkspaceID: 1, Capability: "requirement.acceptance_criteria", SubjectType: "requirement", SubjectID: 2,
		SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
		InputJSON: `{"subject":{"title":"结算页"}}`, RequestedByUser: 1,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&aigorm.AIRunSource{
		WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token",
	}).Error; err != nil {
		t.Fatal(err)
	}
	reader := &changingFeishuReader{document: feishu.Document{
		URL: "https://acme.feishu.cn/docx/token", Revision: "7", Content: "完整正文", ContentHash: "hash", BlockRefs: []string{"b1"},
	}}
	processed, runErr := New(db, cfg, reader).RunOnce(context.Background())
	if runErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if reader.callCount() != 1 {
		t.Fatalf("non-QA Feishu reads=%d, want 1", reader.callCount())
	}
}
