package aiworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

type workerDeliveryStore struct {
	fail bool
	keys []string
}

func (store *workerDeliveryStore) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	_, _ = io.Copy(io.Discard, body)
	store.keys = append(store.keys, key)
	if store.fail {
		return errors.New("temporary COS failure")
	}
	return nil
}

func TestWorkerRemovesDeliveryMetadataFromRuntimePayload(t *testing.T) {
	secret := "worker-test-secret-at-least-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload struct {
			Input map[string]any `json:"input"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, exposed := payload.Input["test_case_delivery"]; exposed {
			t.Fatalf("runtime payload exposed delivery metadata: %v", payload.Input)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":` + workerQAOutput() + `,"usage":{}}}`))
	}))
	defer server.Close()
	cfg := config.Config{
		DatabaseDriver: "sqlite", DatabaseDSN: "file:worker_delivery_payload?mode=memory&cache=shared",
		AI: config.AI{Enabled: true, RuntimeBaseURL: server.URL, InternalSecret: secret, Model: "test", RequestTimeout: time.Second},
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
		SubjectVersion: 1, Status: "queued", Model: "test", RequestedByUser: 1,
		InputJSON: `{"subject":{"title":"结算"},"test_case_delivery":{"instance_id":40,"node_id":"test-cases"}}`,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	processed, runErr := New(db, cfg, nil).RunOnce(context.Background())
	if !processed || runErr != nil {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if err := db.First(&run, run.ID).Error; err != nil || run.Status != "completed" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}

func TestWorkerIdleCompensationRecoversCompletedRun(t *testing.T) {
	db, cfg, run := workerDeliveryFixture(t)
	runID := run.ID
	wrongKey := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: 1, InstanceID: 40, NodeID: "test-cases", Kind: "ai_generated",
		Name: "测试用例-流程40.csv", ContentType: qatestcase.CSVContentType, Size: 10,
		ObjectKey: "pm/workspace/1/team/10/project/11/workflow_test_cases/legacy.csv",
		URL:       "https://cdn.example.test/legacy.csv", AIRunID: &runID, UploaderUserID: 1,
	}
	if err := db.Create(&wrongKey).Error; err != nil {
		t.Fatal(err)
	}
	for id := uint(61); id <= 180; id++ {
		legacyRun := aigorm.AIRun{
			ID: id, WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "plan_unit", SubjectID: 20,
			SubjectVersion: 1, Status: "completed", Model: "test", RequestedByUser: 1,
			InputJSON: `{}`, OutputJSON: workerQAOutput(),
		}
		if err := db.Create(&legacyRun).Error; err != nil {
			t.Fatal(err)
		}
	}
	store := &workerDeliveryStore{fail: true}
	worker := New(db, cfg, nil)
	worker.delivery = qatestcase.NewMaterializer(db, cfg.ObjectStorage, store, nil)
	now := time.Now().UTC()
	worker.now = func() time.Time { return now }
	processed, err := worker.RunOnce(context.Background())
	if processed || err != nil || worker.deliveryScanBeforeID != 81 || len(store.keys) != 0 {
		t.Fatalf("bounded first page processed=%v err=%v cursor=%d keys=%v", processed, err, worker.deliveryScanBeforeID, store.keys)
	}
	processed, err = worker.RunOnce(context.Background())
	if !processed || err != nil || len(store.keys) != 1 {
		t.Fatalf("second page compensation processed=%v err=%v keys=%v", processed, err, store.keys)
	}
	var current aigorm.AIRun
	if err := db.First(&current, run.ID).Error; err != nil || current.Status != "completed" {
		t.Fatalf("delivery failure changed successful run: %+v err=%v", current, err)
	}
	var count int64
	canonicalKey := "pm/workspace/1/team/10/project/11/workflow_test_cases/instance/40/test-cases/run/60.csv"
	if err := db.Model(&workflowgorm.WorkflowNodeArtifact{}).Where("ai_run_id = ? AND object_key = ?", run.ID, canonicalKey).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("canonical artifact count after failure=%d err=%v", count, err)
	}
	store.fail = false
	for attempt := 0; attempt < 8 && len(store.keys) < 2; attempt++ {
		now = now.Add(31 * time.Second)
		processed, err = worker.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("recovery attempt %d processed=%v err=%v", attempt, processed, err)
		}
	}
	if err := db.Model(&workflowgorm.WorkflowNodeArtifact{}).Where("ai_run_id = ? AND object_key = ?", run.ID, canonicalKey).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("artifact count after recovery=%d err=%v keys=%v", count, err, store.keys)
	}
	if len(store.keys) != 2 || store.keys[0] != store.keys[1] {
		t.Fatalf("compensation keys=%v", store.keys)
	}
}

func TestWorkerInterleavesCompensationWithQueuedGeneration(t *testing.T) {
	db, cfg, _ := workerDeliveryFixture(t)
	secret := "worker-delivery-interleave-secret-32"
	runtime := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"output":` + workerQAOutput() + `,"usage":{}}}`))
	}))
	defer runtime.Close()
	cfg.AI.RuntimeBaseURL, cfg.AI.InternalSecret, cfg.AI.RequestTimeout = runtime.URL, secret, time.Second
	queued := aigorm.AIRun{
		ID: 61, WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "plan_unit", SubjectID: 20,
		SubjectVersion: 1, Status: "queued", Model: "test", RequestedByUser: 1, InputJSON: `{"subject":{}}`,
	}
	if err := db.Create(&queued).Error; err != nil {
		t.Fatal(err)
	}
	store := &workerDeliveryStore{fail: true}
	worker := New(db, cfg, nil)
	worker.delivery = qatestcase.NewMaterializer(db, cfg.ObjectStorage, store, nil)
	processed, err := worker.RunOnce(context.Background())
	if !processed || err != nil || len(store.keys) != 1 {
		t.Fatalf("delivery turn processed=%v err=%v keys=%v", processed, err, store.keys)
	}
	processed, err = worker.RunOnce(context.Background())
	if !processed || err != nil {
		t.Fatalf("queued turn processed=%v err=%v", processed, err)
	}
	if err := db.First(&queued, queued.ID).Error; err != nil || queued.Status != "completed" {
		t.Fatalf("queued run starved: %+v err=%v", queued, err)
	}
}

func TestWorkerPermanentlyBacksOffInvalidDeliveryContract(t *testing.T) {
	db, cfg, run := workerDeliveryFixture(t)
	if err := db.Model(&aigorm.AIRun{}).Where("id = ?", run.ID).Update("output_json", `{}`).Error; err != nil {
		t.Fatal(err)
	}
	store := &workerDeliveryStore{}
	worker := New(db, cfg, nil)
	worker.delivery = qatestcase.NewMaterializer(db, cfg.ObjectStorage, store, nil)
	processed, err := worker.RunOnce(context.Background())
	if processed || err != nil {
		t.Fatalf("invalid contract processed=%v err=%v", processed, err)
	}
	retry := worker.deliveryBackoff[run.ID]
	if !retry.Permanent || len(store.keys) != 0 {
		t.Fatalf("retry=%+v keys=%v", retry, store.keys)
	}
}

func workerDeliveryFixture(t *testing.T) (*gorm.DB, config.Config, aigorm.AIRun) {
	t.Helper()
	cfg := config.Config{
		DatabaseDriver: "sqlite", DatabaseDSN: fmt.Sprintf("file:worker_delivery_%d?mode=memory&cache=shared", time.Now().UnixNano()),
		AI:            config.AI{Enabled: true, Model: "test"},
		ObjectStorage: config.ObjectStorage{Enabled: true, Endpoint: "cos.example.test", Bucket: "pm", AccessKey: "key", SecretKey: "secret", UseSSL: true, GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://cdn.example.test"},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"test-cases","type":"human_task","name":"测试用例","config":{"test_case_artifact":true}},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"test-cases"},{"id":"e2","source":"test-cases","target":"end"}]}`
	rows := []any{
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "PM"},
		&projectgorm.PlanUnit{ID: 20, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "V1"},
		&workflowgorm.WorkflowTemplateVersion{ID: 30, WorkspaceID: 1, TemplateID: 31, Version: 1, Definition: definition, DefinitionHash: strings.Repeat("a", 64), PublishedByUser: 1, PublishedAt: time.Now().UTC()},
		&workflowgorm.WorkflowInstance{ID: 40, WorkspaceID: 1, TemplateID: 31, TemplateVersionID: 30, SubjectType: "plan_unit", SubjectID: 20, Status: "running", CurrentNodeIDs: `["test-cases"]`, NodePlansJSON: `{}`, StartedByUserID: 1, StartedAt: time.Now().UTC()},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	run := aigorm.AIRun{
		ID: 60, WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "plan_unit", SubjectID: 20,
		SubjectVersion: 1, Status: "completed", Model: "test", RequestedByUser: 1,
		InputJSON: `{"subject":{},"test_case_delivery":{"instance_id":40,"node_id":"test-cases"}}`, OutputJSON: workerQAOutput(),
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	return db, cfg, run
}

func workerQAOutput() string {
	return `{"summary":"已生成","changes":[],"testCases":[{"caseId":"TC-MAIN-001","title":"主流程","category":"positive","priority":"P0","preconditions":[],"steps":["提交"],"expectedResults":["显示成功"],"evidence":["subject.title"]}],"testCaseDesign":{"smokeCaseIds":["TC-MAIN-001"],"executionOrder":["TC-MAIN-001"],"automationCandidateCaseIds":[],"releaseBlockingCaseIds":["TC-MAIN-001"]},"skillProvenance":{"name":"` + qatestcase.SkillName + `","repository":"` + qatestcase.SkillRepository + `","path":"` + qatestcase.SkillPath + `","commit":"` + qatestcase.SampleSkillCommit + `","packageSha256":"` + qatestcase.SampleSkillPackageSHA256 + `","mode":"advanced","resources":["` + qatestcase.StructuredResource + `","` + qatestcase.AdvancedResource + `","` + qatestcase.WritingResource + `"]}}`
}
