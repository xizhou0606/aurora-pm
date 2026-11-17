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

	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestRequirementDesignImageUploadContract(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID, Title: "视觉改版", Priority: "normal", Status: "backlog", Version: 1}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	disabled := requirementDesignImageRequest(t, app, token, workspace.ID, requirement.ID, "effect.png", "image/png", []byte("png"))
	requireStatus(t, disabled, http.StatusServiceUnavailable)

	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "minio.example.test", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true,
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://cdn.example.test/materials",
	}
	app.rebuildEngine(t, store, nil)
	nonImage := requirementDesignImageRequest(t, app, token, workspace.ID, requirement.ID, "effect.txt", "text/plain", []byte("text"))
	requireStatus(t, nonImage, http.StatusBadRequest)
	pngBody := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...)
	uploaded := requirementDesignImageRequest(t, app, token, workspace.ID, requirement.ID, "effect.PNG", "image/png", pngBody)
	requireStatus(t, uploaded, http.StatusCreated)
	data := decodeData(t, uploaded).(map[string]any)
	keyPattern := regexp.MustCompile(`^pm/workspace/1/team/10/project/11/design_assets/[0-9]{8}/[a-f0-9]{32}\.png$`)
	if !keyPattern.MatchString(store.key) || !bytes.Equal(store.body, pngBody) || store.contentType != "image/png" {
		t.Fatalf("stored design image key=%q size=%d contentType=%q", store.key, len(store.body), store.contentType)
	}
	if data["url"] != "https://cdn.example.test/materials/"+store.key || data["name"] != "effect.PNG" {
		t.Fatalf("upload response=%v", data)
	}
}

func requirementDesignImageRequest(t *testing.T, app *testApp, token string, workspaceID, requirementID uint, filename, contentType string, content []byte) *httptest.ResponseRecorder {
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
	request := httptest.NewRequest(http.MethodPost, "/api/v1/platform/requirements/"+uintString(requirementID)+"/design-assets/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}
