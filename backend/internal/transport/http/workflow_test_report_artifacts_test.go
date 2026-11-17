package httpapi

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestWorkflowTestReportUploadLibraryAndCompleteGate(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "acceptance", "type": "human_task", "name": "测试验收", "config": map[string]any{"test_report_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "acceptance"},
			{"id": "e2", "source": "acceptance", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "验收流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "结算优化", "description": "支持异常重试",
	})
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	path := fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/acceptance/test-report-artifacts", instanceID)

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "advance", "node_id": "acceptance",
	}), http.StatusOK)

	requireStatus(t, workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "report.pdf", []byte("%PDF-1.4\nreport"), 0), http.StatusGone)

	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "cos.ap-shanghai.myqcloud.com", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true, Region: "ap-shanghai",
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://pm-materials.cos.ap-shanghai.myqcloud.com",
	}
	app.rebuildEngine(t, store, nil)
	requireStatus(t, workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "report.xlsx", []byte("not-a-zip"), 0), http.StatusGone)

	pdf := []byte("%PDF-1.4\n极光 v2.3 测试报告")
	requireStatus(t, workflowTestCaseUploadRequest(t, app, token, workspace.ID, path, "极光测试报告.PDF", pdf, 0), http.StatusGone)

	listed := app.request(t, http.MethodGet, path, token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	if rows := decodeData(t, listed).([]any); len(rows) != 0 {
		t.Fatalf("workflow report list should stay empty, got %v", rows)
	}

	library := app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_report", token, workspace.ID, nil)
	requireStatus(t, library, http.StatusOK)
	if items := decodeData(t, library).([]any); len(items) != 0 {
		t.Fatalf("library should be empty before testing-management upload, got %v", items)
	}

	createdReport := testingLibraryUploadRequest(t, app, token, workspace.ID, map[string]string{
		"kind": "test_report", "plan_unit_id": uintString(unit.ID),
	}, "极光测试报告.PDF", pdf)
	requireStatus(t, createdReport, http.StatusCreated)

	library = app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_report", token, workspace.ID, nil)
	requireStatus(t, library, http.StatusOK)
	items := decodeData(t, library).([]any)
	if len(items) != 1 {
		t.Fatalf("library=%v", items)
	}
	item := items[0].(map[string]any)
	if item["kind"] != "test_report" || item["project_id"] != float64(project.ID) || item["project_name"] != project.Name || item["plan_unit_id"] != float64(unit.ID) {
		t.Fatalf("library item=%v project=%s", item, project.Name)
	}
	if _, ok := item["uploader_name"]; !ok {
		t.Fatalf("library item missing uploader_name: %v", item)
	}
	if title, _ := item["subject_title"].(string); title == "结算优化" {
		t.Fatalf("report cards must not bind a requirement title: %v", item)
	}

	cases := app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_case", token, workspace.ID, nil)
	requireStatus(t, cases, http.StatusOK)
	if rows := decodeData(t, cases).([]any); len(rows) != 0 {
		t.Fatalf("test case library should be empty, got %v", rows)
	}

	foreign := workflowgorm.WorkflowInstance{
		WorkspaceID: 2, TemplateID: templateID, TemplateVersionID: 1, SubjectType: "requirement",
		SubjectID: 1, Status: "running", CurrentNodeIDs: `["acceptance"]`, NodePlansJSON: "{}", StartedByUserID: 1,
	}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodGet, fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/acceptance/test-report-artifacts", foreign.ID), token, workspace.ID, nil), http.StatusNotFound)
}

func TestWorkflowTestReportLinkLibraryAndCompleteGate(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "acceptance", "type": "human_task", "name": "测试验收", "config": map[string]any{"test_report_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "acceptance"},
			{"id": "e2", "source": "acceptance", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "验收链接流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "链接验收", "description": "报告用外链",
	})
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	path := fmt.Sprintf("/api/v1/platform/pm/workflows/instances/%d/nodes/acceptance/test-report-artifacts", instanceID)
	linkPath := path + "/link"

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "advance", "node_id": "acceptance",
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPost, linkPath, token, workspace.ID, map[string]any{
		"url": "javascript:alert(1)",
	}), http.StatusGone)
	requireStatus(t, app.request(t, http.MethodPost, linkPath, token, workspace.ID, map[string]any{
		"url": "https://wiki.example.com/qa/aurora-v2.3-report",
	}), http.StatusGone)

	listed := app.request(t, http.MethodGet, path, token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	if rows := decodeData(t, listed).([]any); len(rows) != 0 {
		t.Fatalf("workflow report list should stay empty, got %v", rows)
	}

	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "cos.ap-shanghai.myqcloud.com", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true, Region: "ap-shanghai",
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://pm-materials.cos.ap-shanghai.myqcloud.com",
	}
	app.rebuildEngine(t, store, nil)
	createdLink := testingLibraryUploadRequest(t, app, token, workspace.ID, map[string]string{
		"kind": "test_report", "plan_unit_id": uintString(unit.ID), "url": "https://wiki.example.com/qa/aurora-v2.3-report",
	}, "", nil)
	requireStatus(t, createdLink, http.StatusCreated)
	data := decodeData(t, createdLink).(map[string]any)
	if data["kind"] != "test_report" || data["url"] != "https://wiki.example.com/qa/aurora-v2.3-report" {
		t.Fatalf("link report response=%v", data)
	}

	library := app.request(t, http.MethodGet, "/api/v1/platform/pm/testing/items?kind=test_report", token, workspace.ID, nil)
	requireStatus(t, library, http.StatusOK)
	items := decodeData(t, library).([]any)
	if len(items) != 1 {
		t.Fatalf("library=%v", items)
	}
	item := items[0].(map[string]any)
	if item["kind"] != "test_report" || item["url"] != "https://wiki.example.com/qa/aurora-v2.3-report" || item["project_id"] != float64(project.ID) {
		t.Fatalf("library item=%v", item)
	}

	reportID := uint(data["id"].(float64))
	download := app.request(t, http.MethodGet, fmt.Sprintf("/api/v1/platform/pm/testing/reports/%d/download", reportID), token, workspace.ID, nil)
	if download.Code != http.StatusFound || download.Header().Get("Location") != "https://wiki.example.com/qa/aurora-v2.3-report" {
		t.Fatalf("download redirect code=%d location=%q body=%q", download.Code, download.Header().Get("Location"), download.Body.String())
	}
}

func TestWorkflowDefinitionStripsTestReportOnApprovalNode(t *testing.T) {
	app, token, workspace, _, _ := newAIWorkflowFixture(t)
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "错误验收", "subject_type": "requirement", "definition": map[string]any{
			"nodes": []map[string]any{
				{"id": "start", "type": "start", "name": "开始"},
				{"id": "review", "type": "approval", "name": "人工评审", "config": map[string]any{"critical_gate": true, "test_report_artifact": true}},
				{"id": "end", "type": "end", "name": "结束"},
			},
			"edges": []map[string]any{
				{"id": "e1", "source": "start", "target": "review"},
				{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
			},
		},
	})
	requireStatus(t, created, http.StatusCreated)
	template := decodeData(t, created).(map[string]any)
	if hasRetiredTestReportFlag(template["definition"]) {
		t.Fatalf("created template should strip test_report_artifact: %v", template["definition"])
	}
	templateID := uint(template["id"].(float64))
	validate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/validate-definition", token, workspace.ID, map[string]any{
		"definition": map[string]any{
			"nodes": []map[string]any{
				{"id": "start", "type": "start", "name": "开始"},
				{"id": "review", "type": "approval", "name": "测试验收", "config": map[string]any{"test_report_artifact": true}},
				{"id": "end", "type": "end", "name": "结束"},
			},
			"edges": []map[string]any{
				{"id": "e1", "source": "start", "target": "review"},
				{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
			},
		},
	})
	requireStatus(t, validate, http.StatusOK)
	payload := decodeData(t, validate).(map[string]any)
	if payload["valid"] != true {
		t.Fatalf("approval node should ignore retired test_report_artifact: %v", payload)
	}
}

func hasRetiredTestReportFlag(raw any) bool {
	definition, _ := raw.(map[string]any)
	nodes, _ := definition["nodes"].([]any)
	for _, item := range nodes {
		node, _ := item.(map[string]any)
		config, _ := node["config"].(map[string]any)
		if flag, _ := config["test_report_artifact"].(bool); flag {
			return true
		}
	}
	return false
}

func testingLibraryUploadRequest(t *testing.T, app *testApp, token string, workspaceID uint, fields map[string]string, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	if filename != "" {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
		header.Set("Content-Type", "application/octet-stream")
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(content)
	}
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/platform/pm/testing/items", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}
