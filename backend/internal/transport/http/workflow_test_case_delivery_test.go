package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type deliveryHTTPStore struct {
	mu       sync.Mutex
	readyErr error
	putErr   error
	keys     []string
	objects  map[string][]byte
}

func (store *deliveryHTTPStore) CheckReady(context.Context) error { return store.readyErr }

func (store *deliveryHTTPStore) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	raw, _ := io.ReadAll(body)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.keys = append(store.keys, key)
	if store.putErr != nil {
		return store.putErr
	}
	if store.objects == nil {
		store.objects = map[string][]byte{}
	}
	store.objects[key] = raw
	return nil
}

func TestCreateTestCaseRunRejectsUnavailableStorageBeforePersistence(t *testing.T) {
	app, token, workspace, requirementID, instanceID := newTestCaseDeliveryFixture(t)
	store := &deliveryHTTPStore{readyErr: errors.New("bucket denied")}
	enableDeliveryStore(t, app, store)
	response := createDeliveryRun(t, app, token, workspace.ID, requirementID, instanceID, "cases-a")
	requireStatus(t, response, http.StatusServiceUnavailable)
	var count int64
	if err := app.db.Model(&aigorm.AIRun{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("AI run count=%d err=%v", count, err)
	}

	// Readiness is only a prerequisite for the supported QA delivery flow;
	// invalid capability requests still receive deterministic contract errors.
	invalidCapability := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "unknown", "subject_type": "requirement", "subject_id": requirementID,
		"test_case_delivery": map[string]any{"instance_id": instanceID, "node_id": "cases-a"},
	})
	requireStatus(t, invalidCapability, http.StatusBadRequest)
}

func TestCreateTestCaseDeliveryTargetValidationAndListProjection(t *testing.T) {
	app, ownerToken, workspace, requirementID, instanceID := newTestCaseDeliveryFixture(t)
	enableDeliveryStore(t, app, &deliveryHTTPStore{})

	missingNode := createDeliveryRun(t, app, ownerToken, workspace.ID, requirementID, instanceID, "missing")
	requireStatus(t, missingNode, http.StatusBadRequest)
	foreign := workflowgorm.WorkflowInstance{
		WorkspaceID: 2, TemplateID: 1, TemplateVersionID: 1, SubjectType: "requirement", SubjectID: requirementID,
		Status: "running", CurrentNodeIDs: `["cases-a"]`, NodePlansJSON: `{}`, StartedByUserID: 1,
	}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	crossTenant := createDeliveryRun(t, app, ownerToken, workspace.ID, requirementID, foreign.ID, "cases-a")
	requireStatus(t, crossTenant, http.StatusNotFound)

	other := app.createUser(t, 2, "other", "member", "")
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: workspace.ID, UserID: other.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	created := createDeliveryRun(t, app, app.token(t, other), workspace.ID, requirementID, instanceID, "cases-a")
	requireStatus(t, created, http.StatusCreated)
	runID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	listed := app.request(t, http.MethodGet, fmt.Sprintf("/api/v1/platform/ai/runs?subject_type=requirement&subject_id=%d", requirementID), ownerToken, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	rows := decodeData(t, listed).([]any)
	if len(rows) != 1 {
		t.Fatalf("runs=%v", rows)
	}
	row := rows[0].(map[string]any)
	target := row["test_case_delivery"].(map[string]any)
	if uint(row["id"].(float64)) != runID || target["instance_id"] != float64(instanceID) || target["node_id"] != "cases-a" {
		t.Fatalf("run projection=%v", row)
	}
	if _, exposed := row["input"]; exposed {
		t.Fatalf("list response exposed full AI input: %v", row)
	}
}

func TestRetryTestCaseDeliveryRunRevalidatesStorageWithoutAssigneeCheck(t *testing.T) {
	app, ownerToken, workspace, requirementID, instanceID := newTestCaseDeliveryFixture(t)
	enableDeliveryStore(t, app, &deliveryHTTPStore{})
	created := createDeliveryRun(t, app, ownerToken, workspace.ID, requirementID, instanceID, "cases-a")
	requireStatus(t, created, http.StatusCreated)
	runID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	claimFence := time.Now().UTC().Truncate(time.Millisecond)
	if err := app.db.Model(&aigorm.AIRun{}).Where("id = ?", runID).Updates(map[string]any{
		"status": "failed", "started_at": claimFence,
	}).Error; err != nil {
		t.Fatal(err)
	}
	retryPath := fmt.Sprintf("/api/v1/platform/ai/runs/%d/retry", runID)
	enableDeliveryStore(t, app, &deliveryHTTPStore{readyErr: errors.New("bucket denied")})
	requireStatus(t, app.request(t, http.MethodPost, retryPath, ownerToken, workspace.ID, nil), http.StatusServiceUnavailable)

	other := app.createUser(t, 2, "delivery-retry-other", "member", "")
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: workspace.ID, UserID: other.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	enableDeliveryStore(t, app, &deliveryHTTPStore{})
	requireStatus(t, app.request(t, http.MethodPost, retryPath, app.token(t, other), workspace.ID, nil), http.StatusOK)
	var run aigorm.AIRun
	if err := app.db.First(&run, runID).Error; err != nil || run.Status != "queued" || run.StartedAt == nil || !run.StartedAt.Equal(claimFence) {
		t.Fatalf("retry run=%+v err=%v", run, err)
	}
}

func TestAIGeneratedArtifactEndpointIsIdempotentAndTargetBound(t *testing.T) {
	app, token, workspace, requirementID, instanceID := newTestCaseDeliveryFixture(t)
	store := &deliveryHTTPStore{}
	enableDeliveryStore(t, app, store)
	created := createDeliveryRun(t, app, token, workspace.ID, requirementID, instanceID, "cases-a")
	requireStatus(t, created, http.StatusCreated)
	runID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	if err := app.db.Model(&aigorm.AIRun{}).Where("id = ?", runID).Updates(map[string]any{
		"status": "completed", "output_json": qaDeliveryOutput(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	wrongNodePath := fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/cases-b/test-case-artifacts/ai-generated", instanceID)
	requireStatus(t, app.request(t, http.MethodPost, wrongNodePath, token, workspace.ID, map[string]any{"ai_run_id": runID}), http.StatusConflict)
	path := fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/cases-a/test-case-artifacts/ai-generated", instanceID)
	store.readyErr = errors.New("bucket denied")
	enableDeliveryStore(t, app, store)
	requireStatus(t, app.request(t, http.MethodPost, path, token, workspace.ID, map[string]any{"ai_run_id": runID}), http.StatusServiceUnavailable)
	if len(store.keys) != 0 {
		t.Fatalf("readiness failure uploaded keys=%v", store.keys)
	}
	store.readyErr = nil
	enableDeliveryStore(t, app, store)
	first := app.request(t, http.MethodPost, path, token, workspace.ID, map[string]any{"ai_run_id": runID})
	requireStatus(t, first, http.StatusCreated)
	second := app.request(t, http.MethodPost, path, token, workspace.ID, map[string]any{"ai_run_id": runID})
	requireStatus(t, second, http.StatusOK)
	firstData, secondData := decodeData(t, first).(map[string]any), decodeData(t, second).(map[string]any)
	if firstData["id"] != secondData["id"] || len(store.keys) != 1 {
		t.Fatalf("first=%v second=%v upload keys=%v", firstData, secondData, store.keys)
	}
	var artifact workflowgorm.WorkflowNodeArtifact
	if err := app.db.Where("ai_run_id = ?", runID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	wantSuffix := fmt.Sprintf("/instance/%d/cases-a/run/%d.csv", instanceID, runID)
	if !strings.HasSuffix(artifact.ObjectKey, wantSuffix) || artifact.Kind != "ai_generated" {
		t.Fatalf("artifact=%+v want suffix=%q", artifact, wantSuffix)
	}
}

func newTestCaseDeliveryFixture(t *testing.T) (*testApp, string, identitygorm.Workspace, uint, uint) {
	t.Helper()
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "cases-a", "type": "human_task", "name": "测试用例 A", "config": map[string]any{"test_case_artifact": true}},
			{"id": "cases-b", "type": "human_task", "name": "测试用例 B", "config": map[string]any{"test_case_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "cases-a"},
			{"id": "e2", "source": "cases-a", "target": "cases-b"},
			{"id": "e3", "source": "cases-b", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "稳定交付流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "复杂结算需求"})
	return app, token, workspace, uint(requirement["id"].(float64)), uint(requirement["workflow_instance_id"].(float64))
}

func enableDeliveryStore(t *testing.T, app *testApp, store *deliveryHTTPStore) {
	t.Helper()
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "cos.example.test", Bucket: "pm", AccessKey: "key", SecretKey: "secret",
		UseSSL: true, GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://cdn.example.test",
	}
	app.rebuildEngine(t, store, nil)
}

func createDeliveryRun(t *testing.T, app *testApp, token string, workspaceID, requirementID, instanceID uint, nodeID string) *httptest.ResponseRecorder {
	t.Helper()
	return app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspaceID, map[string]any{
		"capability": "qa.test_cases", "subject_type": "requirement", "subject_id": requirementID,
		"test_case_delivery": map[string]any{"instance_id": instanceID, "node_id": nodeID},
	})
}

func qaDeliveryOutput() string {
	return `{"summary":"已生成","testCases":[{"caseId":"TC-MAIN-001","title":"主流程","category":"positive","priority":"P0","preconditions":[],"steps":["提交"],"expectedResults":["显示成功"],"evidence":["subject.title"]}],"testCaseDesign":{"smokeCaseIds":["TC-MAIN-001"],"executionOrder":["TC-MAIN-001"],"automationCandidateCaseIds":[],"releaseBlockingCaseIds":["TC-MAIN-001"]},"skillProvenance":{"name":"` + qatestcase.SkillName + `","repository":"` + qatestcase.SkillRepository + `","path":"` + qatestcase.SkillPath + `","commit":"` + qatestcase.SampleSkillCommit + `","packageSha256":"` + qatestcase.SampleSkillPackageSHA256 + `","mode":"advanced","resources":["` + qatestcase.StructuredResource + `","` + qatestcase.AdvancedResource + `","` + qatestcase.WritingResource + `"]}}`
}
