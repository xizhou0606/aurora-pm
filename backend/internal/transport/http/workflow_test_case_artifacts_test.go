package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"regexp"
	"testing"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type fakeWorkflowArtifactStore struct {
	objects    map[string][]byte
	readErrors map[string]error
	writeError error
}

type failingWorkflowArtifactReader struct{ err error }

func (r failingWorkflowArtifactReader) Read(_ []byte) (int, error) { return 0, r.err }

func (f *fakeWorkflowArtifactStore) PutObject(
	_ context.Context,
	key string,
	body io.Reader,
	_ int64,
	_ string,
) error {
	if f.writeError != nil {
		return f.writeError
	}
	if f.objects == nil {
		f.objects = map[string][]byte{}
	}
	f.objects[key], _ = io.ReadAll(body)
	return nil
}

func (f *fakeWorkflowArtifactStore) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	if err := f.readErrors[key]; err != nil {
		return io.NopCloser(failingWorkflowArtifactReader{err: err}), nil
	}
	body, found := f.objects[key]
	if !found {
		return nil, fmt.Errorf("object not found")
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

func TestSafeWorkflowArtifactUploadErrorRedactsStorageCredentials(t *testing.T) {
	got := safeWorkflowArtifactUploadError(
		fmt.Errorf("request failed\naccess=example-access secret=example-secret"),
		"example-access",
		"example-secret",
	)
	if got != "request failed access=[redacted] secret=[redacted]" {
		t.Fatalf("sanitized error=%q", got)
	}
}

func TestWorkflowTestCaseCSVAllowsUTF8RuneAcrossPeekBoundary(t *testing.T) {
	raw := append(bytes.Repeat([]byte("a"), 511), []byte("中文")...)
	if utf8Prefix := raw[:512]; !validTestCaseFileSignature(".csv", utf8Prefix) {
		t.Fatalf("valid UTF-8 rune split at peek boundary was rejected: %x", utf8Prefix[508:])
	}
	if !validTestCaseCSV(bufio.NewReader(bytes.NewReader(raw))) {
		t.Fatal("valid streamed UTF-8 CSV was rejected")
	}
	invalid := append(bytes.Repeat([]byte("a"), 510), 0xff, 'b')
	if validTestCaseFileSignature(".csv", invalid) || validTestCaseCSV(bufio.NewReader(bytes.NewReader(invalid))) {
		t.Fatal("invalid UTF-8 CSV was accepted")
	}
}

func TestWorkflowTestCasePDFContentTypeAndSignature(t *testing.T) {
	if contentType, ok := workflowTestCaseContentType(".pdf"); !ok || contentType != "application/pdf" {
		t.Fatalf("pdf content type=%q ok=%v", contentType, ok)
	}
	if !validTestCaseFileSignature(".pdf", []byte("%PDF-1.7\ncases")) {
		t.Fatal("valid PDF header was rejected")
	}
	if validTestCaseFileSignature(".pdf", []byte("not a pdf")) || validTestCaseFileSignature(".pdf", nil) {
		t.Fatal("invalid PDF payload was accepted")
	}
}

func TestWorkflowTestCaseArtifactUploadAndTenantContract(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "test-cases", "type": "human_task", "name": "测试用例编写", "config": map[string]any{"test_case_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{{"id": "e1", "source": "start", "target": "test-cases"}, {"id": "e2", "source": "test-cases", "target": "end"}},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "QA 流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "结算优化", "description": "支持异常重试", "acceptance_criteria": "重复提交不产生重复订单",
	})
	requirementID := uint(createdRequirement["id"].(float64))
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	path := fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/test-cases/test-case-artifacts", instanceID)

	disabled := workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "cases.csv", []byte("title,expected\nA,B"), 0)
	requireStatus(t, disabled, http.StatusServiceUnavailable)

	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "cos.ap-shanghai.myqcloud.com", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true, Region: "ap-shanghai",
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://pm-materials.cos.ap-shanghai.myqcloud.com",
	}
	app.rebuildEngine(t, store, nil)
	invalid := workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "cases.xlsx", []byte("not-a-zip"), 0)
	requireStatus(t, invalid, http.StatusBadRequest)

	aiCreated := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "qa.test_cases", "subject_type": "requirement", "subject_id": requirementID,
	})
	requireStatus(t, aiCreated, http.StatusCreated)
	aiRunID := uint(decodeData(t, aiCreated).(map[string]any)["id"].(float64))
	if err := app.db.Model(&aigorm.AIRun{}).Where("id = ?", aiRunID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	csv := []byte("\xef\xbb\xbftitle,expected\n重复提交,只创建一单")
	uploaded := workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "结算测试用例.CSV", csv, aiRunID)
	requireStatus(t, uploaded, http.StatusCreated)
	data := decodeData(t, uploaded).(map[string]any)
	keyPattern := regexp.MustCompile(`^pm/workspace/1/team/10/project/11/design_assets/[0-9]{8}/[a-f0-9]{32}-cases\.csv$`)
	if !keyPattern.MatchString(store.key) || !bytes.Equal(store.body, csv) || store.contentType != "text/csv; charset=utf-8" {
		t.Fatalf("stored artifact key=%q contentType=%q body=%q", store.key, store.contentType, store.body)
	}
	if data["kind"] != "uploaded" || data["ai_run_id"] != nil || data["name"] != "结算测试用例.CSV" {
		t.Fatalf("artifact response=%v", data)
	}
	wantDownloadURL := fmt.Sprintf("/api/v1/platform/pm/workflows/test-case-materials/%d/%d/download", requirementID, uint(data["id"].(float64)))
	if data["url"] != wantDownloadURL {
		t.Fatalf("artifact url=%v, want authenticated route %q", data["url"], wantDownloadURL)
	}
	downloadedMaterial := app.request(t, http.MethodGet, wantDownloadURL, token, workspace.ID, nil)
	requireStatus(t, downloadedMaterial, http.StatusOK)
	if !bytes.Equal(downloadedMaterial.Body.Bytes(), csv) {
		t.Fatalf("workflow material download body=%q, want %q", downloadedMaterial.Body.Bytes(), csv)
	}
	projected := app.request(t, http.MethodGet,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/material-projections?kind=test_case",
		token, workspace.ID, nil,
	)
	requireStatus(t, projected, http.StatusOK)
	projectedRows := decodeData(t, projected).([]any)
	if len(projectedRows) != 1 || projectedRows[0].(map[string]any)["id"] != data["id"] {
		t.Fatalf("workflow material projections=%v", projectedRows)
	}
	firstObjectKey := store.key
	duplicate := workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "重复上传.csv", []byte("title,expected\nduplicate,no"), aiRunID)
	requireStatus(t, duplicate, http.StatusCreated)
	duplicateData := decodeData(t, duplicate).(map[string]any)
	if duplicateData["id"] == data["id"] || duplicateData["kind"] != "uploaded" || duplicateData["ai_run_id"] != nil || store.key == firstObjectKey {
		t.Fatalf("legacy ai_run_id must not bind manual upload: first=%v duplicate=%v first key=%q second key=%q", data, duplicateData, firstObjectKey, store.key)
	}
	listed := app.request(t, http.MethodGet, path, token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	if rows := decodeData(t, listed).([]any); len(rows) != 2 || rows[0].(map[string]any)["url"] != duplicateData["url"] {
		t.Fatalf("artifact list=%v", rows)
	}
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "advance", "node_id": "test-cases",
	}), http.StatusOK)
	// Missing material can be supplemented after the workflow has already left
	// this node; uploading does not reopen or advance the workflow.
	requireStatus(t, workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "late.csv", []byte("title,expected\nlate,no"), 0), http.StatusCreated)
	requireStatus(t, app.request(t, http.MethodGet, path, token, workspace.ID, nil), http.StatusOK)

	pdfUpload := workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "结算用例.pdf", []byte("%PDF-1.7\n结算"), 0)
	requireStatus(t, pdfUpload, http.StatusCreated)
	if store.contentType != "application/pdf" || !bytes.HasPrefix(store.body, []byte("%PDF")) {
		t.Fatalf("pdf artifact contentType=%q body=%q", store.contentType, store.body)
	}
	requireStatus(t, workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "fake.pdf", []byte("not a pdf"), 0), http.StatusBadRequest)

	caseLink := "https://wiki.example.com/qa/cases"
	linked := workflowTestCaseLinkRequest(t, app, token, workspace.ID, path, caseLink)
	requireStatus(t, linked, http.StatusCreated)
	linkData := decodeData(t, linked).(map[string]any)
	if linkData["source"] != "link" || linkData["url"] != caseLink || linkData["content_type"] != "text/uri-list" {
		t.Fatalf("link artifact=%v", linkData)
	}
	requireStatus(t, workflowTestCaseLinkRequest(t, app, token, workspace.ID, path, "ftp://wiki.example.com/cases"), http.StatusBadRequest)
	listed = app.request(t, http.MethodGet, path, token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	listRows := decodeData(t, listed).([]any)
	if len(listRows) != 5 {
		t.Fatalf("artifact list after pdf and link=%v", listRows)
	}
	// Materials list is ordered id DESC, so the freshly saved link comes first.
	newestRow := listRows[0].(map[string]any)
	if newestRow["url"] != caseLink || newestRow["source"] != "link" {
		t.Fatalf("listed link artifact=%v", newestRow)
	}

	app.rebuildEngine(t, &fakeWorkflowArtifactStore{writeError: fmt.Errorf("upstream connection reset")}, nil)
	storageFailure := workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "retry.csv", []byte("title,expected\nretry,later"), 0)
	requireStatus(t, storageFailure, http.StatusBadGateway)
	if !bytes.Contains(storageFailure.Body.Bytes(), []byte(`"code":"OBJECT_STORAGE_UPLOAD_FAILED"`)) {
		t.Fatalf("storage failure response=%s", storageFailure.Body.String())
	}

	foreign := workflowgorm.WorkflowInstance{WorkspaceID: 2, TemplateID: templateID, TemplateVersionID: 1, SubjectType: "requirement", SubjectID: requirementID, Status: "running", CurrentNodeIDs: `["test-cases"]`, NodePlansJSON: "{}", StartedByUserID: 1}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodGet, fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/test-cases/test-case-artifacts", foreign.ID), token, workspace.ID, nil), http.StatusNotFound)
}

func TestWorkflowNodeArtifactDownloadAndWorkspaceIsolation(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	objectKey := "pm/workspace/1/project/11/workflow_test_cases/checkout.csv"
	body := []byte("title,expected\ncheckout,success")
	store := &fakeWorkflowArtifactStore{
		objects:    map[string][]byte{objectKey: body},
		readErrors: map[string]error{},
	}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "minio.example.test", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret",
	}
	app.rebuildEngine(t, store, nil)
	instance := workflowgorm.WorkflowInstance{
		WorkspaceID: workspace.ID, SubjectType: "plan_unit", SubjectID: unit.ID,
		Status: "running", CurrentNodeIDs: `[]`, NodePlansJSON: "{}", StartedByUserID: 1,
	}
	if err := app.db.Create(&instance).Error; err != nil {
		t.Fatal(err)
	}
	local := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: workspace.ID, InstanceID: instance.ID, NodeID: "test-cases", Kind: "uploaded",
		Name: "checkout-cases.csv", ContentType: "text/csv; charset=utf-8", Size: int64(len(body)),
		ObjectKey: objectKey, URL: "https://materials.example.test/checkout.csv", UploaderUserID: 1,
	}
	foreign := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: 2, InstanceID: 82, NodeID: "test-cases", Kind: "uploaded",
		Name: "foreign-cases.csv", ContentType: "text/csv; charset=utf-8", Size: 6,
		ObjectKey: "pm/workspace/2/foreign.csv", URL: "https://materials.example.test/foreign.csv", UploaderUserID: 1,
	}
	lazyMissing := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: workspace.ID, InstanceID: instance.ID, NodeID: "test-cases", Kind: "uploaded",
		Name: "missing.csv", ContentType: "text/csv", Size: 8,
		ObjectKey: "pm/workspace/1/missing.csv", UploaderUserID: 1,
	}
	emptyObject := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: workspace.ID, InstanceID: instance.ID, NodeID: "test-cases", Kind: "uploaded",
		Name: "empty.csv", ContentType: "text/csv", Size: 8,
		ObjectKey: "pm/workspace/1/empty.csv", UploaderUserID: 1,
	}
	orphaned := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: workspace.ID, InstanceID: instance.ID + 1000, NodeID: "test-cases", Kind: "uploaded",
		Name: "orphaned.csv", ContentType: "text/csv", Size: int64(len(body)),
		ObjectKey: "pm/workspace/1/orphaned.csv", UploaderUserID: 1,
	}
	if err := app.db.Create(&local).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&lazyMissing).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&emptyObject).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&orphaned).Error; err != nil {
		t.Fatal(err)
	}
	store.readErrors[lazyMissing.ObjectKey] = fmt.Errorf("lazy object not found")
	store.objects[emptyObject.ObjectKey] = []byte{}

	downloaded := app.request(t, http.MethodGet,
		"/api/v1/platform/pm/workflows/test-case-artifacts/"+uintString(local.ID)+"/download",
		token, workspace.ID, nil,
	)
	requireStatus(t, downloaded, http.StatusOK)
	if !bytes.Equal(downloaded.Body.Bytes(), body) {
		t.Fatalf("download body=%q, want %q", downloaded.Body.Bytes(), body)
	}
	if downloaded.Header().Get("Content-Type") != local.ContentType {
		t.Fatalf("download content type=%q, want %q", downloaded.Header().Get("Content-Type"), local.ContentType)
	}
	if downloaded.Header().Get("Content-Length") != fmt.Sprint(len(body)) {
		t.Fatalf("download content length=%q, want %d", downloaded.Header().Get("Content-Length"), len(body))
	}
	if downloaded.Header().Get("Content-Disposition") != `attachment; filename=checkout-cases.csv` {
		t.Fatalf("download content disposition=%q", downloaded.Header().Get("Content-Disposition"))
	}

	isolation := app.request(t, http.MethodGet,
		"/api/v1/platform/pm/workflows/test-case-artifacts/"+uintString(foreign.ID)+"/download",
		token, workspace.ID, nil,
	)
	requireStatus(t, isolation, http.StatusNotFound)

	lazyFailure := app.request(t, http.MethodGet,
		"/api/v1/platform/pm/workflows/test-case-artifacts/"+uintString(lazyMissing.ID)+"/download",
		token, workspace.ID, nil,
	)
	requireStatus(t, lazyFailure, http.StatusNotFound)

	emptyFailure := app.request(t, http.MethodGet,
		"/api/v1/platform/pm/workflows/test-case-artifacts/"+uintString(emptyObject.ID)+"/download",
		token, workspace.ID, nil,
	)
	requireStatus(t, emptyFailure, http.StatusNotFound)

	orphanedFailure := app.request(t, http.MethodGet,
		"/api/v1/platform/pm/workflows/test-case-artifacts/"+uintString(orphaned.ID)+"/download",
		token, workspace.ID, nil,
	)
	requireStatus(t, orphanedFailure, http.StatusNotFound)
}

func TestPlanUnitTestCaseAIInputIncludesRequirementsAndTasks(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID, Title: "登录", Description: "手机号登录", Version: 2}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&taskgorm.WorkTask{WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, ProjectID: unit.ID, RequirementID: requirement.ID, Kind: "task", TaskKey: "QA-1", Title: "实现登录", Status: "in_dev"}).Error; err != nil {
		t.Fatal(err)
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "qa.test_cases", "subject_type": "plan_unit", "subject_id": unit.ID,
	})
	requireStatus(t, created, http.StatusCreated)
	input := decodeData(t, created).(map[string]any)["input"].(map[string]any)
	context := input["test_case_context"].(map[string]any)
	if len(context["requirements"].([]any)) != 1 || len(context["tasks"].([]any)) != 1 {
		t.Fatalf("test case context=%v", context)
	}
}

func TestCompleteTestCaseNodeWithoutMaterialStillAdvances(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "test-cases", "type": "human_task", "name": "测试用例编写", "config": map[string]any{"test_case_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{{"id": "e1", "source": "start", "target": "test-cases"}, {"id": "e2", "source": "test-cases", "target": "end"}},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "免测试用例完成流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "无测试用例交付"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "test-cases",
	}), http.StatusOK)
	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	if data := decodeData(t, detail).(map[string]any); data["status"] != "completed" {
		t.Fatalf("instance status=%v, want completed without a test case material", data["status"])
	}
}

func TestSkippedWorkflowTestCaseNodeRejectsArtifactUpload(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "choice", "type": "condition", "name": "是否需要测试用例"},
			{"id": "cases", "type": "human_task", "name": "测试用例编写", "config": map[string]any{"test_case_artifact": true}},
			{"id": "manual", "type": "human_task", "name": "人工处理"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "choice"},
			{"id": "e2", "source": "choice", "target": "cases", "kind": "yes"},
			{"id": "e3", "source": "choice", "target": "manual", "kind": "no"},
			{"id": "e4", "source": "cases", "target": "end"},
			{"id": "e5", "source": "manual", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "可选测试用例流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "不需要测试用例"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "condition_no", "node_id": "choice",
	}), http.StatusOK)

	path := fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/cases/test-case-artifacts", instanceID)
	uploaded := workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "skipped.csv", []byte("title,expected\nA,B"), 0)
	requireStatus(t, uploaded, http.StatusConflict)
	var artifacts int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeArtifact{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "cases").
		Count(&artifacts).Error; err != nil || artifacts != 0 {
		t.Fatalf("skipped node artifact count=%d err=%v", artifacts, err)
	}
}

func workflowTestCaseLinkRequest(t *testing.T, app *testApp, token string, workspaceID uint, path, link string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("url", link)
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}

func workflowTestCaseUploadRequest(t *testing.T, app *testApp, token string, workspaceID uint, path, filename string, content []byte, aiRunID uint) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	if aiRunID > 0 {
		_ = writer.WriteField("ai_run_id", uintString(aiRunID))
	}
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}
