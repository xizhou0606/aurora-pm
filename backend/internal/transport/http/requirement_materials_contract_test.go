package httpapi

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestRequirementMaterialsEarlyUploadAndTestingLibraryContract(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "cos.ap-shanghai.myqcloud.com", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true, Region: "ap-shanghai",
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://pm-materials.cos.ap-shanghai.myqcloud.com",
	}
	app.rebuildEngine(t, store, nil)

	requirement := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
		Title: "提前上传材料", Priority: "normal", Status: "backlog", Version: 1,
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}

	materialPath := fmt.Sprintf("/api/v1/platform/requirements/%d/materials", requirement.ID)
	first := requirementMaterialUploadRequest(t, app, token, workspace.ID, materialPath, "test_case", "cases-a.csv", []byte("title,expected\nA,ok"))
	requireStatus(t, first, http.StatusCreated)
	second := requirementMaterialUploadRequest(t, app, token, workspace.ID, materialPath, "test_case", "cases-b.csv", []byte("title,expected\nB,ok"))
	requireStatus(t, second, http.StatusCreated)

	listed := app.request(t, http.MethodGet, materialPath+"?kind=test_case", token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	if rows := decodeData(t, listed).([]any); len(rows) != 2 {
		t.Fatalf("requirement materials=%v", rows)
	}

	pool := app.request(t, http.MethodGet,
		"/api/v1/platform/requirements?project_new_id="+uintString(project.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, pool, http.StatusOK)
	deliverables := requirementPoolRowDeliverables(t, pool, requirement.ID)
	if deliverables["test_cases"] != true {
		t.Fatalf("pool test_cases=%v", deliverables["test_cases"])
	}
	artifact, _ := deliverables["test_case_artifact"].(map[string]any)
	materialID, _ := artifact["id"].(float64)
	wantDownload := fmt.Sprintf("/api/v1/platform/requirements/%d/materials/%d/download", requirement.ID, uint(materialID))
	if artifact["requirement_id"] != float64(requirement.ID) || artifact["url"] != wantDownload {
		t.Fatalf("pool test case artifact=%v want requirement_id=%d url=%s", artifact, requirement.ID, wantDownload)
	}
	downloaded := app.request(t, http.MethodGet, wantDownload, token, workspace.ID, nil)
	requireStatus(t, downloaded, http.StatusOK)
	if !bytes.Contains(downloaded.Body.Bytes(), []byte("title,expected")) {
		t.Fatalf("material download body=%q", downloaded.Body.String())
	}

	library := app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_case", token, workspace.ID, nil)
	requireStatus(t, library, http.StatusOK)
	if rows := decodeData(t, library).([]any); len(rows) != 2 {
		t.Fatalf("testing library cases=%v", rows)
	}

	missingRequirement := testingLibraryUploadRequest(t, app, token, workspace.ID, map[string]string{
		"kind": "test_case",
	}, "missing.csv", []byte("title,expected\nC,ok"))
	requireStatus(t, missingRequirement, http.StatusBadRequest)

	fromLibrary := testingLibraryUploadRequest(t, app, token, workspace.ID, map[string]string{
		"kind": "test_case", "requirement_id": uintString(requirement.ID),
	}, "library.csv", []byte("title,expected\nlibrary,ok"))
	requireStatus(t, fromLibrary, http.StatusCreated)

	listed = app.request(t, http.MethodGet, materialPath+"?kind=test_case", token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	if rows := decodeData(t, listed).([]any); len(rows) != 3 {
		t.Fatalf("requirement materials after library upload=%v", rows)
	}

	missingPlanUnit := testingLibraryUploadRequest(t, app, token, workspace.ID, map[string]string{
		"kind": "test_report",
	}, "report.pdf", []byte("%PDF-1.4\nreport"))
	requireStatus(t, missingPlanUnit, http.StatusBadRequest)

	createdReport := testingLibraryUploadRequest(t, app, token, workspace.ID, map[string]string{
		"kind": "test_report", "plan_unit_id": uintString(unit.ID),
	}, "report.pdf", []byte("%PDF-1.4\nreport"))
	requireStatus(t, createdReport, http.StatusCreated)

	designs := app.request(t, http.MethodGet, materialPath+"?kind=design", token, workspace.ID, nil)
	requireStatus(t, designs, http.StatusOK)
	if rows := decodeData(t, designs).([]any); len(rows) != 0 {
		t.Fatalf("reports must not appear as requirement materials: %v", rows)
	}

	foreign := requirementgorm.Requirement{
		WorkspaceID: 2, TeamID: 99, ProjectSetID: 99, Title: "外租户", Priority: "normal", Status: "backlog", Version: 1,
	}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodGet, fmt.Sprintf("/api/v1/platform/requirements/%d/materials", foreign.ID), token, workspace.ID, nil), http.StatusNotFound)
	requireStatus(t, testingLibraryUploadRequest(t, app, token, workspace.ID, map[string]string{
		"kind": "test_case", "requirement_id": uintString(foreign.ID),
	}, "foreign.csv", []byte("title,expected\nx,y")), http.StatusNotFound)

	// Test-case materials accept PDF files and external links like CSV/XLS do.
	pdfUpload := requirementMaterialUploadRequest(t, app, token, workspace.ID, materialPath, "test_case", "cases.pdf", []byte("%PDF-1.7\ncases"))
	requireStatus(t, pdfUpload, http.StatusCreated)
	if data := decodeData(t, pdfUpload).(map[string]any); data["content_type"] != "application/pdf" {
		t.Fatalf("pdf material=%v", data)
	}
	requireStatus(t, requirementMaterialUploadRequest(t, app, token, workspace.ID, materialPath, "test_case", "fake.pdf", []byte("not a pdf")), http.StatusBadRequest)

	caseLink := "https://wiki.example.com/qa/test-cases"
	linked := requirementMaterialLinkRequest(t, app, token, workspace.ID, materialPath, "test_case", caseLink)
	requireStatus(t, linked, http.StatusCreated)
	if data := decodeData(t, linked).(map[string]any); data["content_type"] != "text/uri-list" || data["origin_url"] != caseLink {
		t.Fatalf("test case link material=%v", data)
	}
	requireStatus(t, requirementMaterialLinkRequest(t, app, token, workspace.ID, materialPath, "test_case", "ftp://wiki.example.com/cases"), http.StatusBadRequest)

	listed = app.request(t, http.MethodGet, materialPath+"?kind=test_case", token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	if rows := decodeData(t, listed).([]any); len(rows) != 5 {
		t.Fatalf("requirement materials after pdf and link=%v", rows)
	}

	library = app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_case", token, workspace.ID, nil)
	requireStatus(t, library, http.StatusOK)
	var linkItem map[string]any
	for _, row := range decodeData(t, library).([]any) {
		item := row.(map[string]any)
		if item["url"] == caseLink {
			linkItem = item
		}
	}
	if linkItem == nil || linkItem["source"] != "link" {
		t.Fatalf("testing library link item=%v want source=link for %q", linkItem, caseLink)
	}

	pool = app.request(t, http.MethodGet,
		"/api/v1/platform/requirements?project_new_id="+uintString(project.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, pool, http.StatusOK)
	deliverables = requirementPoolRowDeliverables(t, pool, requirement.ID)
	artifact, _ = deliverables["test_case_artifact"].(map[string]any)
	if artifact == nil || artifact["url"] != caseLink {
		t.Fatalf("pool link artifact=%v want url=%q", artifact, caseLink)
	}
}

func TestRequirementPoolEchoesSavedDesignMaterialsAfterReload(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "cos.ap-shanghai.myqcloud.com", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true, Region: "ap-shanghai",
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://pm-materials.cos.ap-shanghai.myqcloud.com",
	}
	app.rebuildEngine(t, store, nil)

	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "设计稿回显流程", map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "design", "type": "human_task", "name": "UI/UX设计", "config": map[string]any{"require_design_image": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "design"},
			{"id": "e2", "source": "design", "target": "end"},
		},
	})
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"project_new_id": project.ID, "title": "表格回显设计稿",
	})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	first := "https://www.figma.com/design/abc123/apps"
	second := "https://www.figma.com/design/def456/apps111"
	materialPath := fmt.Sprintf("/api/v1/platform/requirements/%d/materials", requirementID)
	requireStatus(t, requirementMaterialLinkRequest(t, app, token, workspace.ID, materialPath, "design", first), http.StatusCreated)
	requireStatus(t, requirementMaterialLinkRequest(t, app, token, workspace.ID, materialPath, "design", second), http.StatusCreated)
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "supplement_design", "node_id": "design", "design_url": first,
	}), http.StatusOK)

	listed := app.request(t, http.MethodGet,
		"/api/v1/platform/requirements?project_new_id="+uintString(project.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, listed, http.StatusOK)
	deliverables := requirementPoolRowDeliverables(t, listed, requirementID)
	if deliverables["design"] != true {
		t.Fatalf("pool design ready=%v", deliverables["design"])
	}
	urls, _ := deliverables["design_urls"].([]any)
	if !containsAllStrings(urls, first, second) {
		t.Fatalf("pool design_urls=%v want %q and %q", urls, first, second)
	}
	artifacts, _ := deliverables["design_artifacts"].([]any)
	if !designArtifactsEchoLinks(artifacts, first, second) {
		t.Fatalf("pool design_artifacts=%v want link names %q and %q", artifacts, first, second)
	}
	var updated requirementgorm.Requirement
	if err := app.db.Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).First(&updated).Error; err != nil || updated.DesignURL == "" {
		t.Fatalf("requirement design_url=%q err=%v", updated.DesignURL, err)
	}
}

func requirementMaterialLinkRequest(t *testing.T, app *testApp, token string, workspaceID uint, path, kind, link string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("kind", kind)
	_ = writer.WriteField("source", "requirement")
	_ = writer.WriteField("url", link)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}

func requirementPoolRowDeliverables(t *testing.T, listed *httptest.ResponseRecorder, requirementID uint) map[string]any {
	t.Helper()
	for _, row := range decodeRequirementPoolList(t, listed).Data {
		if row["id"] != float64(requirementID) {
			continue
		}
		deliverables, ok := row["deliverable_overview"].(map[string]any)
		if !ok {
			t.Fatalf("deliverable overview missing: %v", row)
		}
		return deliverables
	}
	t.Fatalf("requirement %d missing from pool", requirementID)
	return nil
}

func containsAllStrings(values []any, want ...string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		text, _ := value.(string)
		seen[text] = true
	}
	for _, item := range want {
		if !seen[item] {
			return false
		}
	}
	return len(want) > 0
}

func designArtifactsEchoLinks(values []any, want ...string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name, _ := item["name"].(string)
		url, _ := item["url"].(string)
		if name == url && url != "" {
			seen[url] = true
		}
	}
	for _, item := range want {
		if !seen[item] {
			return false
		}
	}
	return len(want) > 0
}

func requirementMaterialUploadRequest(t *testing.T, app *testApp, token string, workspaceID uint, path, kind, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("kind", kind)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}
