package aiworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

type fakeReader struct{}

type failingReader struct{}

type emptyReader struct{}

type truncatedReader struct{}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type contextBoundResponseBody struct {
	ctx       context.Context
	reader    *strings.Reader
	readDelay time.Duration
	maxChunk  int
	closed    chan struct{}
}

func (body *contextBoundResponseBody) Read(buffer []byte) (int, error) {
	if body.readDelay > 0 {
		timer := time.NewTimer(body.readDelay)
		select {
		case <-body.ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return 0, body.ctx.Err()
		case <-timer.C:
			body.readDelay = 0
		}
	}
	select {
	case <-body.ctx.Done():
		return 0, body.ctx.Err()
	default:
		if body.maxChunk > 0 && len(buffer) > body.maxChunk {
			buffer = buffer[:body.maxChunk]
		}
		return body.reader.Read(buffer)
	}
}

func (body *contextBoundResponseBody) Close() error {
	if body.closed != nil {
		select {
		case <-body.closed:
		default:
			close(body.closed)
		}
	}
	return nil
}

func (fakeReader) Read(context.Context, string) (feishu.Document, error) {
	return feishu.Document{URL: "https://acme.feishu.cn/docx/token", Revision: "7", Title: "需求", Content: "敏感正文只进内存", ContentHash: "abc", BlockRefs: []string{"b1"}}, nil
}

func (failingReader) Read(context.Context, string) (feishu.Document, error) {
	return feishu.Document{}, context.DeadlineExceeded
}

func (emptyReader) Read(context.Context, string) (feishu.Document, error) {
	return feishu.Document{URL: "https://acme.feishu.cn/docx/token", Title: "空需求", Content: "  \n\t"}, nil
}

func (truncatedReader) Read(context.Context, string) (feishu.Document, error) {
	return feishu.Document{
		URL: "https://acme.feishu.cn/docx/token", Title: "超长需求", Content: "不完整正文",
		OriginalContentChars: 250001, ContentTruncated: true,
	}, nil
}

func TestCallRuntimeRetriesEOFOnce(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	cfg := config.Config{AI: config.AI{RuntimeBaseURL: "http://ai-runtime.test", InternalSecret: secret, RequestTimeout: 2 * time.Second}}
	worker := New(nil, cfg, nil)
	attempts := 0
	worker.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, io.EOF
		}
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if request.Header.Get("X-PM-Signature") != sign(raw, request.Header.Get("X-PM-Timestamp"), secret) {
			t.Fatal("retry request has an invalid signature")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{}}`)),
			Request:    request,
		}, nil
	})
	status, raw, err := worker.callRuntime(context.Background(), []byte(`{"runId":54}`))
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || string(raw) != `{"data":{}}` {
		t.Fatalf("status=%d response=%q", status, raw)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}

func TestCallRuntimeRetriesWithinOneOverallDeadline(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	timeout := 650 * time.Millisecond
	cfg := config.Config{AI: config.AI{RuntimeBaseURL: "http://ai-runtime.test", InternalSecret: secret, RequestTimeout: timeout}}
	worker := New(nil, cfg, nil)
	attempts := 0
	deadlines := make([]time.Time, 0, 2)
	worker.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Fatal("runtime attempt is missing an overall deadline")
		}
		deadlines = append(deadlines, deadline)
		if attempts == 1 {
			return nil, io.EOF
		}
		<-request.Context().Done()
		return nil, request.Context().Err()
	})

	started := time.Now()
	_, _, err := worker.callRuntime(context.Background(), []byte(`{"runId":55}`))
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want context deadline exceeded", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
	if len(deadlines) != 2 || !deadlines[0].Equal(deadlines[1]) {
		t.Fatalf("attempt deadlines=%v, want one shared deadline", deadlines)
	}
	if elapsed > timeout+250*time.Millisecond {
		t.Fatalf("elapsed=%s exceeded shared timeout=%s", elapsed, timeout)
	}
}

func TestCallRuntimeKeepsDeadlineAliveUntilResponseBodyIsConsumed(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	cfg := config.Config{AI: config.AI{
		RuntimeBaseURL: "http://ai-runtime.test", InternalSecret: secret,
		RequestTimeout: 2 * time.Second,
	}}
	worker := New(nil, cfg, nil)
	var requestCtx context.Context
	bodyClosed := make(chan struct{})
	worker.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCtx = request.Context()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: &contextBoundResponseBody{
				ctx: request.Context(), reader: strings.NewReader(`{"data":{"output":{}}}`), closed: bodyClosed,
			},
			Request: request,
		}, nil
	})

	status, raw, err := worker.callRuntime(context.Background(), []byte(`{"runId":56}`))
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || string(raw) != `{"data":{"output":{}}}` {
		t.Fatalf("status=%d response=%q", status, raw)
	}
	select {
	case <-bodyClosed:
	default:
		t.Fatal("runtime response body was not closed")
	}
	select {
	case <-requestCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("runtime request deadline was not released after response EOF")
	}
}

func TestCallRuntimeReadsDelayedChunkedBodyAfterHeaders(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	cfg := config.Config{AI: config.AI{
		RuntimeBaseURL: "http://ai-runtime.test", InternalSecret: secret,
		RequestTimeout: time.Second,
	}}
	worker := New(nil, cfg, nil)
	worker.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: &contextBoundResponseBody{
				ctx: request.Context(), reader: strings.NewReader(`{"data":{"output":{"summary":"完成"}}}`),
				readDelay: 40 * time.Millisecond, maxChunk: 5,
			},
			Request: request,
		}, nil
	})

	started := time.Now()
	status, raw, err := worker.callRuntime(context.Background(), []byte(`{"runId":57}`))
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 35*time.Millisecond {
		t.Fatalf("body was not consumed before return: elapsed=%s", elapsed)
	}
	if status != http.StatusOK || string(raw) != `{"data":{"output":{"summary":"完成"}}}` {
		t.Fatalf("status=%d response=%q", status, raw)
	}
}

func TestCallRuntimeDeadlineCoversResponseBodyRead(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	cfg := config.Config{AI: config.AI{
		RuntimeBaseURL: "http://ai-runtime.test", InternalSecret: secret,
		RequestTimeout: 40 * time.Millisecond,
	}}
	worker := New(nil, cfg, nil)
	worker.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: &contextBoundResponseBody{
				ctx: request.Context(), reader: strings.NewReader(`{"data":{}}`),
				readDelay: time.Second,
			},
			Request: request,
		}, nil
	})

	status, raw, err := worker.callRuntime(context.Background(), []byte(`{"runId":58}`))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want context deadline exceeded", err)
	}
	if status != http.StatusOK || raw != nil {
		t.Fatalf("status=%d response=%q after body timeout", status, raw)
	}
}

func TestReadRuntimeResponseRejectsPayloadOverFourMiB(t *testing.T) {
	withinLimit := bytes.Repeat([]byte{'a'}, maxRuntimeResponseBytes)
	raw, err := readRuntimeResponse(bytes.NewReader(withinLimit))
	if err != nil {
		t.Fatalf("read exact-limit response: %v", err)
	}
	if len(raw) != maxRuntimeResponseBytes {
		t.Fatalf("response bytes=%d, want %d", len(raw), maxRuntimeResponseBytes)
	}

	overLimit := bytes.Repeat([]byte{'a'}, maxRuntimeResponseBytes+1)
	if _, err := readRuntimeResponse(bytes.NewReader(overLimit)); err == nil || !strings.Contains(err.Error(), "exceeds the 4 MiB limit") {
		t.Fatalf("overflow error=%v", err)
	}
}

func TestValidateRuntimeRequestSizeRejectsOversizedQATestCaseInputBeforeHTTP(t *testing.T) {
	if err := validateRuntimeRequestSize("qa.test_cases", maxQATestCaseRuntimeRequestBytes); err != nil {
		t.Fatalf("exact limit rejected: %v", err)
	}
	if err := validateRuntimeRequestSize("qa.test_cases", maxQATestCaseRuntimeRequestBytes+1); err == nil || !strings.Contains(err.Error(), "7 MiB") {
		t.Fatalf("overflow error=%v", err)
	}
	if err := validateRuntimeRequestSize("qa.test_cases", (2<<20)+1); err != nil {
		t.Fatalf("QA source bundles above the old 2 MiB ceiling must reach runtime batching: %v", err)
	}
	if err := validateRuntimeRequestSize("requirement.draft", maxRuntimeRequestBytes); err != nil {
		t.Fatalf("non-QA input should accept the transport limit: %v", err)
	}
}

func TestWorkerSignsRequestAndDoesNotPersistFeishuContent(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		raw := make([]byte, request.ContentLength)
		_, _ = request.Body.Read(raw)
		if request.Header.Get("X-PM-Signature") != sign(raw, request.Header.Get("X-PM-Timestamp"), secret) {
			t.Error("invalid HMAC signature")
		}
		if json.Unmarshal(raw, &payload) != nil {
			t.Error("invalid payload")
		}
		input, _ := payload["input"].(map[string]any)
		sources, _ := input["sources"].([]any)
		if len(sources) != 1 {
			t.Errorf("runtime sources=%v, want one successful Feishu source", sources)
		} else if row, _ := sources[0].(map[string]any); row["available"] != true || row["content"] != "敏感正文只进内存" {
			t.Errorf("runtime source=%v, want available in-memory content", row)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"验收标准","changes":[{"field":"acceptance_criteria","value":"可验证","reason":"补全"}],"items":[],"risks":[],"humanReviewRequired":true},"usage":{"inputTokens":12,"outputTokens":8}}}`))
	}))
	defer server.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_test?mode=memory&cache=shared", AI: config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: time.Second}, Feishu: config.Feishu{Enabled: true}}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{WorkspaceID: 1, Capability: "requirement.acceptance_criteria", SubjectType: "requirement", SubjectID: 2, SubjectVersion: 1, Status: "queued", Model: "qwen3.6-max-preview", InputJSON: `{"subject":{"title":"A"}}`, RequestedByUser: 1}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	source := aigorm.AIRunSource{WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token"}
	if err := db.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	worker := New(db, cfg, fakeReader{})
	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" {
		t.Fatalf("run status=%s", run.Status)
	}
	if err := db.First(&source, source.ID).Error; err != nil {
		t.Fatal(err)
	}
	if source.Revision != "7" || source.BlockRefs == "" || source.ContentHash != "abc" {
		t.Fatalf("source manifest=%+v", source)
	}
	var leaked int64
	db.Raw("SELECT COUNT(*) FROM ai_run_sources WHERE block_refs LIKE ?", "%敏感正文%").Scan(&leaked)
	if leaked != 0 {
		t.Fatal("Feishu content was persisted")
	}
}

func TestTaskPlanGenerationPersistsDraftWithoutCreatingTasks(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"已拆分前后端任务","changes":[],"items":[],"risks":[],"testCases":[],"humanReviewRequired":true,"completenessReport":null,"workflowProposal":null,"taskPlan":{"chains":[{"lane":"frontend","summary":"交付需求页面","tasks":[{"title":"实现需求页面","description":"完成页面状态与接口接入","acceptanceCriteria":["页面可操作"]}]},{"lane":"backend","summary":"交付需求接口","tasks":[{"title":"实现需求接口","description":"完成租户隔离和持久化","acceptanceCriteria":["接口测试通过"]}]}]}},"usage":{"inputTokens":20,"outputTokens":30}}}`))
	}))
	defer server.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_task_plan_test?mode=memory&cache=shared", AI: config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "test-model", RequestTimeout: time.Second}}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 7, Name: "项目", Code: "DEMO"}
	requirement := requirementgorm.Requirement{ID: 134, WorkspaceID: 1, TeamID: 7, ProjectSetID: project.ID, PlanUnitID: 25, Version: 9, Title: "结算优化", Priority: "high"}
	if err := db.Create(&project).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{WorkspaceID: 1, Capability: "requirement.task_plan", SubjectType: "requirement", SubjectID: requirement.ID, SubjectVersion: requirement.Version, Status: "queued", Model: cfg.AI.Model, InputJSON: `{"subject":{"id":134,"title":"结算优化"}}`, RequestedByUser: 42}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	processed, runErr := New(db, cfg, nil).RunOnce(context.Background())
	if !processed || runErr != nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	var tasks []taskgorm.WorkTask
	if err := db.Where("workspace_id = ? AND requirement_id = ?", 1, requirement.ID).Order("id").Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("generation created %d tasks before confirmation: %+v", len(tasks), tasks)
	}
	var outboxCount, auditCount, activityCount int64
	db.Model(&auditgorm.OutboxEvent{}).Where("workspace_id = ? AND event_type = ?", 1, "pm.work_task.created").Count(&outboxCount)
	db.Model(&auditgorm.AuditLog{}).Where("workspace_id = ? AND resource = ? AND action = ?", 1, "task", "create").Count(&auditCount)
	db.Model(&taskgorm.WorkTaskActivity{}).Where("workspace_id = ?", 1).Count(&activityCount)
	if outboxCount != 0 || auditCount != 0 || activityCount != 0 {
		t.Fatalf("generation side effects: outbox=%d audit=%d activity=%d", outboxCount, auditCount, activityCount)
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" || !strings.Contains(run.OutputJSON, `"taskPlan"`) || strings.Contains(run.OutputJSON, `"createdTasks"`) {
		t.Fatalf("completed task plan output=%s status=%s", run.OutputJSON, run.Status)
	}
	var artifact aigorm.AIArtifact
	if err := db.Where("workspace_id = ? AND run_id = ?", 1, run.ID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(artifact.ContentJSON, `"taskPlan"`) || strings.Contains(artifact.ContentJSON, `"createdTasks"`) {
		t.Fatalf("task plan artifact was not preserved as a draft: %s", artifact.ContentJSON)
	}
}

func TestTaskPlanWithoutBothChainsFailsWithoutCreatingTasks(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"不完整拆解","changes":[],"taskPlan":{"chains":[{"lane":"frontend","summary":"只有前端","tasks":[{"title":"实现页面","description":"实现页面","acceptanceCriteria":["页面完成"]}]},{"lane":"frontend","summary":"重复前端","tasks":[{"title":"实现交互","description":"实现交互","acceptanceCriteria":["交互完成"]}]}]}},"usage":{}}}`))
	}))
	defer server.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_invalid_task_plan_test?mode=memory&cache=shared", AI: config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "test-model", RequestTimeout: time.Second}}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{WorkspaceID: 1, Capability: "requirement.task_plan", SubjectType: "requirement", SubjectID: 134, SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model, InputJSON: `{}`, RequestedByUser: 42}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	processed, runErr := New(db, cfg, nil).RunOnce(context.Background())
	if !processed || runErr == nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	var taskCount int64
	db.Model(&taskgorm.WorkTask{}).Count(&taskCount)
	if taskCount != 0 {
		t.Fatalf("invalid plan created %d tasks", taskCount)
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || !strings.Contains(run.ErrorMessage, "one frontend and one backend") {
		t.Fatalf("run status=%s error=%q", run.Status, run.ErrorMessage)
	}
}

func TestDocumentBackedRequirementRunsFailClosedWhenFeishuDocumentIsNotReadable(t *testing.T) {
	for _, capability := range []string{"requirement.completeness", "requirement.task_plan"} {
		t.Run(capability, func(t *testing.T) {
			secret := "worker-test-secret-at-least-32-characters"
			upstreamCalled := false
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				upstreamCalled = true
				writer.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			cfg := config.Config{
				DatabaseDriver: "sqlite",
				DatabaseDSN:    "file:worker_feishu_required_" + strings.ReplaceAll(capability, ".", "_") + "?mode=memory&cache=shared",
				AI:             config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: time.Second},
				Feishu:         config.Feishu{Enabled: true},
			}
			db, err := database.Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := database.Migrate(db); err != nil {
				t.Fatal(err)
			}
			run := aigorm.AIRun{
				WorkspaceID: 1, Capability: capability, SubjectType: "requirement",
				SubjectID: 2, SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
				InputJSON: `{"subject":{"product_doc_url":"https://acme.feishu.cn/docx/token"}}`, RequestedByUser: 1,
			}
			if err := db.Create(&run).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&aigorm.AIRunSource{WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token"}).Error; err != nil {
				t.Fatal(err)
			}
			processed, runErr := New(db, cfg, nil).RunOnce(context.Background())
			if !processed || runErr == nil {
				t.Fatalf("processed=%v err=%v", processed, runErr)
			}
			if upstreamCalled {
				t.Fatal("AI runtime must not be called without the primary Feishu document")
			}
			if err := db.First(&run, run.ID).Error; err != nil {
				t.Fatal(err)
			}
			if run.Status != "failed" || !strings.Contains(run.ErrorMessage, "飞书需求正文未读取") {
				t.Fatalf("run status=%s error=%q", run.Status, run.ErrorMessage)
			}
		})
	}
}

func TestCompletenessRunUsesLocalSubjectWhenFeishuIsDisabled(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	upstreamCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalled = true
		defer request.Body.Close()
		raw, _ := io.ReadAll(request.Body)
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			t.Error("invalid payload")
		}
		input, _ := payload["input"].(map[string]any)
		subject, _ := input["subject"].(map[string]any)
		if subject["title"] != "本地需求" || subject["description"] != "本地正文" {
			t.Errorf("runtime subject=%v, want local title and description", subject)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":{"summary":"完整性检查完成","changes":[],"items":[],"risks":[],"humanReviewRequired":true,"completenessReport":{"score":40,"threshold":70,"canProceed":false,"missingItems":[],"questions":[]},"workflowProposal":null},"usage":{"inputTokens":8,"outputTokens":6}}}`))
	}))
	defer server.Close()
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "file:worker_feishu_disabled_local_subject?mode=memory&cache=shared",
		AI:             config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: time.Second},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{
		WorkspaceID: 1, Capability: "requirement.completeness", SubjectType: "requirement",
		SubjectID: 128, SubjectVersion: 4, Status: "queued", Model: cfg.AI.Model,
		InputJSON:       `{"subject":{"title":"本地需求","description":"本地正文","product_doc_url":"https://acme.feishu.cn/docx/token"}}`,
		RequestedByUser: 1,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&aigorm.AIRunSource{WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token"}).Error; err != nil {
		t.Fatal(err)
	}
	processed, runErr := New(db, cfg, nil).RunOnce(context.Background())
	if !processed || runErr != nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if !upstreamCalled {
		t.Fatal("AI runtime must run against local requirement fields when Feishu is disabled")
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" {
		t.Fatalf("run status=%s error=%q", run.Status, run.ErrorMessage)
	}
}

func TestCompletenessRunFailsClosedWhenFeishuDocumentBodyIsEmpty(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	upstreamCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalled = true
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "file:worker_feishu_empty_test?mode=memory&cache=shared",
		AI:             config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: time.Second},
		Feishu:         config.Feishu{Enabled: true},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{
		WorkspaceID: 1, Capability: "requirement.completeness", SubjectType: "requirement",
		SubjectID: 2, SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
		InputJSON: `{"subject":{"product_doc_url":"https://acme.feishu.cn/docx/token"}}`, RequestedByUser: 1,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&aigorm.AIRunSource{WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token"}).Error; err != nil {
		t.Fatal(err)
	}
	worker := New(db, cfg, emptyReader{})
	processed, runErr := worker.RunOnce(context.Background())
	if !processed || runErr == nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if upstreamCalled {
		t.Fatal("AI runtime must not be called with an empty primary Feishu document")
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || !strings.Contains(run.ErrorMessage, "飞书需求正文为空") {
		t.Fatalf("run status=%s error=%q", run.Status, run.ErrorMessage)
	}
}

func TestTestCaseRunFailsClosedWhenConfiguredFeishuSourceIsUnavailable(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	upstreamCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalled = true
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "file:worker_optional_feishu_test?mode=memory&cache=shared",
		AI:             config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: time.Second},
		Feishu:         config.Feishu{Enabled: true},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{
		WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement",
		SubjectID: 2, SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
		InputJSON: `{"subject":{"title":"结算页"}}`, RequestedByUser: 1,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&aigorm.AIRunSource{WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token"}).Error; err != nil {
		t.Fatal(err)
	}
	worker := New(db, cfg, failingReader{})
	processed, runErr := worker.RunOnce(context.Background())
	if !processed || runErr == nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if upstreamCalled {
		t.Fatal("AI runtime must not be called when a configured QA source is unavailable")
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || !strings.Contains(run.ErrorMessage, "read Feishu source") {
		t.Fatalf("run status=%s error=%q", run.Status, run.ErrorMessage)
	}
}

func TestTestCaseRunFailsClosedWhenFeishuSourceIsTruncated(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	upstreamCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalled = true
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := config.Config{
		DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_truncated_feishu_test?mode=memory&cache=shared",
		AI:     config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: time.Second},
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
		WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement",
		SubjectID: 2, SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model,
		InputJSON: `{"subject":{"title":"结算页","product_doc_url":"https://acme.feishu.cn/docx/token"},"test_case_context":{"requirements":[{"product_doc_url":"https://acme.feishu.cn/docx/token"}]}}`, RequestedByUser: 1,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&aigorm.AIRunSource{WorkspaceID: 1, RunID: run.ID, SourceType: "feishu_link", SourceURL: "https://acme.feishu.cn/docx/token"}).Error; err != nil {
		t.Fatal(err)
	}
	worker := New(db, cfg, truncatedReader{})
	processed, runErr := worker.RunOnce(context.Background())
	if !processed || runErr == nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if upstreamCalled {
		t.Fatal("AI runtime must not be called with a truncated QA source")
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || !strings.Contains(run.ErrorMessage, "无法保证测试用例完整性") {
		t.Fatalf("run status=%s error=%q", run.Status, run.ErrorMessage)
	}
}

func TestValidateQATestCaseSkillProvenance(t *testing.T) {
	valid := json.RawMessage(`{"skillProvenance":{"name":"qa-test-case-design","repository":"https://github.com/QA19816-glitch/QA-agent","path":"codex-skills/skills/qa-test-case-design","commit":"d75606531f6ce414bbcd95fb203033ae86653852","packageSha256":"07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935","mode":"advanced","resources":["output-templates/structured-test-cases.md","prompts/advanced-test-case-design.md","references/case-writing-rules.md"]}}`)
	if err := validateQATestCaseSkillProvenance("qa.test_cases", valid); err != nil {
		t.Fatalf("valid provenance rejected: %v", err)
	}
	if err := validateQATestCaseSkillProvenance("requirement.draft", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("non-QA capability should not require QA provenance: %v", err)
	}

	for name, raw := range map[string]json.RawMessage{
		"missing":                   json.RawMessage(`{}`),
		"invalid package hash":      json.RawMessage(`{"skillProvenance":{"name":"qa-test-case-design","repository":"https://github.com/QA19816-glitch/QA-agent","path":"codex-skills/skills/qa-test-case-design","commit":"d75606531f6ce414bbcd95fb203033ae86653852","packageSha256":"not-a-sha256","mode":"advanced","resources":["output-templates/structured-test-cases.md","prompts/advanced-test-case-design.md","references/case-writing-rules.md"]}}`),
		"missing advanced resource": json.RawMessage(`{"skillProvenance":{"name":"qa-test-case-design","repository":"https://github.com/QA19816-glitch/QA-agent","path":"codex-skills/skills/qa-test-case-design","commit":"d75606531f6ce414bbcd95fb203033ae86653852","packageSha256":"07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935","mode":"advanced","resources":["output-templates/structured-test-cases.md","references/case-writing-rules.md"]}}`),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateQATestCaseSkillProvenance("qa.test_cases", raw); err == nil {
				t.Fatal("invalid provenance was accepted")
			}
		})
	}
}

func TestWorkerSurfacesRuntimeFailureMessage(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(`{"error":"model generation failed","message":"test case expected result is not observable: TC-1"}`))
	}))
	defer server.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_runtime_message_test?mode=memory&cache=shared", AI: config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: time.Second}}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "requirement", SubjectID: 2, SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model, InputJSON: `{"subject":{"title":"结算"}}`, RequestedByUser: 1}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	worker := New(db, cfg, nil)
	processed, runErr := worker.RunOnce(context.Background())
	if !processed || runErr == nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" {
		t.Fatalf("run status=%s", run.Status)
	}
	if !strings.Contains(run.ErrorMessage, "test case expected result is not observable: TC-1") {
		t.Fatalf("error_message=%q", run.ErrorMessage)
	}
}

func TestWorkerStopsUpstreamRequestWhenRunIsCancelled(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	requestStarted := make(chan struct{})
	requestStopped := make(chan struct{})
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_cancel_test?mode=memory&cache=shared", AI: config.AI{Enabled: true, RuntimeBaseURL: "http://ai-runtime.test", InternalSecret: secret, Model: "qwen3.6-max-preview", RequestTimeout: 5 * time.Second}}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	run := aigorm.AIRun{WorkspaceID: 1, Capability: "workflow.generate", SubjectType: "workflow_template", SubjectID: 2, SubjectVersion: 1, Status: "queued", Model: cfg.AI.Model, InputJSON: `{}`, RequestedByUser: 1}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	worker := New(db, cfg, nil)
	worker.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(requestStarted)
		<-request.Context().Done()
		close(requestStopped)
		return nil, request.Context().Err()
	})
	result := make(chan error, 1)
	go func() {
		_, runErr := worker.RunOnce(context.Background())
		result <- runErr
	}()
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("worker did not start the upstream request")
	}
	if err := db.Model(&aigorm.AIRun{}).Where("id = ?", run.ID).Update("status", "cancelled").Error; err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestStopped:
	case <-time.After(time.Second):
		t.Fatal("upstream request remained active after cancellation")
	}
	select {
	case runErr := <-result:
		if runErr != nil {
			t.Fatalf("cancelled run returned an error: %v", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not release the cancelled run")
	}
}
