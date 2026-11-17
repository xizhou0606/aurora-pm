package httpapi

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"regexp"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestWorkflowPreReviewOpinionImageUploadContract(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	instanceID := seedPreReviewOpinionInstance(t, app, token, workspace.ID, unit.ID)

	disabled := workflowOpinionImageRequest(t, app, token, workspace.ID, instanceID, "pre-review", "shot.png", "image/png", []byte("png"))
	requireStatus(t, disabled, http.StatusServiceUnavailable)

	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "minio.example.test", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true,
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://cdn.example.test/materials",
	}
	app.rebuildEngine(t, store, nil)

	foreignNode := workflowOpinionImageRequest(t, app, token, workspace.ID, instanceID, "review", "shot.png", "image/png", pngOpinionImage())
	requireStatus(t, foreignNode, http.StatusNotFound)

	plain := workflowOpinionImageRequest(t, app, token, workspace.ID, instanceID, "pre-review", "notes.txt", "text/plain", []byte("text"))
	requireStatus(t, plain, http.StatusBadRequest)

	uploaded := workflowOpinionImageRequest(t, app, token, workspace.ID, instanceID, "pre-review", "shot.PNG", "image/png", pngOpinionImage())
	requireStatus(t, uploaded, http.StatusCreated)
	data := decodeData(t, uploaded).(map[string]any)
	keyPattern := regexp.MustCompile(`^pm/workspace/1/team/10/project/11/opinion_images/[0-9]{8}/[a-f0-9]{32}\.png$`)
	if !keyPattern.MatchString(store.key) || !bytes.Equal(store.body, pngOpinionImage()) || store.contentType != "image/png" {
		t.Fatalf("stored opinion image key=%q size=%d contentType=%q", store.key, len(store.body), store.contentType)
	}
	if data["url"] != "https://cdn.example.test/materials/"+store.key || data["name"] != "shot.PNG" {
		t.Fatalf("upload response=%v", data)
	}

	html := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/pre-review/opinions", token, workspace.ID, map[string]any{
		"body": `<p>建议对照截图</p><p><img src="` + data["url"].(string) + `" alt="shot.PNG"></p>`,
	})
	requireStatus(t, html, http.StatusCreated)
}

func seedPreReviewOpinionInstance(t *testing.T, app *testApp, token string, workspaceID, unitID uint) uint {
	t.Helper()
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "pre-review", "type": "approval", "name": "需求预审", "config": map[string]any{"critical_gate": true}},
			{"id": "review", "type": "approval", "name": "产品评审"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "pre-review"},
			{"id": "e2", "source": "pre-review", "target": "review", "kind": "approve"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspaceID, map[string]any{
		"name": "预审贴图流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspaceID, nil), http.StatusOK)
	requirement := createRequirementWithWorkflow(t, app, token, workspaceID, unitID, templateID, map[string]any{"title": "预审贴图需求"})
	return uint(requirement["workflow_instance_id"].(float64))
}

func pngOpinionImage() []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...)
}

func workflowOpinionImageRequest(
	t *testing.T,
	app *testApp,
	token string,
	workspaceID, instanceID uint,
	nodeID, filename, contentType string,
	content []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/"+nodeID+"/opinion-images",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}
