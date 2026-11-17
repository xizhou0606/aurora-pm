package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type fakeMaterialStore struct {
	key         string
	contentType string
	body        []byte
	size        int64
	err         error
	objects     map[string][]byte
}

func (f *fakeMaterialStore) PutObject(
	_ context.Context,
	key string,
	body io.Reader,
	size int64,
	contentType string,
) error {
	f.key, f.size, f.contentType = key, size, contentType
	f.body, _ = io.ReadAll(body)
	if f.objects == nil {
		f.objects = map[string][]byte{}
	}
	f.objects[key] = append([]byte(nil), f.body...)
	return f.err
}

func (f *fakeMaterialStore) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	body, found := f.objects[key]
	if !found {
		return nil, fmt.Errorf("object not found")
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

func TestQualityMaterialUploadContractWorkspaceAccessAndIsolation(t *testing.T) {
	fix := setupQualityPermissionFixture(t)

	disabled := qualityMaterialRequest(t, fix, "owner", fix.requirement.ID, "evidence.png", []byte("proof"))
	requireStatus(t, disabled, http.StatusServiceUnavailable)

	store := &fakeMaterialStore{}
	fix.app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "minio.example.test", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true,
		GlobalPrefix: "pm", TeamPrefix: true,
		PublicBaseURL: "https://cdn.example.test/materials",
	}
	fix.app.rebuildEngine(t, store, nil)

	legacyReadonly := qualityMaterialRequest(t, fix, "readonly", fix.requirement.ID, "evidence.png", []byte("proof"))
	requireStatus(t, legacyReadonly, http.StatusCreated)

	foreignRequirement := seedForeignQualityUploadScope(t, fix)
	foreign := qualityMaterialRequest(t, fix, "owner", foreignRequirement.ID, "secret.png", []byte("secret"))
	requireStatus(t, foreign, http.StatusNotFound)

	missingRequirement := qualityMaterialRequest(t, fix, "owner", 0, "evidence.png", []byte("proof"))
	requireStatus(t, missingRequirement, http.StatusCreated)
	workspaceKeyPattern := regexp.MustCompile(`^pm/workspace/1/bug_materials/[0-9]{8}/[a-f0-9]{32}\.png$`)
	if !workspaceKeyPattern.MatchString(store.key) {
		t.Fatalf("workspace material object key=%q", store.key)
	}

	uploaded := qualityMaterialRequest(t, fix, "owner", fix.requirement.ID, "screen shot.PNG", []byte("proof"))
	requireStatus(t, uploaded, http.StatusCreated)
	data := decodeData(t, uploaded).(map[string]any)
	keyPattern := regexp.MustCompile(`^pm/workspace/1/team/10/project/11/bug_materials/[0-9]{8}/[a-f0-9]{32}\.png$`)
	if !keyPattern.MatchString(store.key) {
		t.Fatalf("object key=%q", store.key)
	}
	if string(store.body) != "proof" || store.size != 5 || store.contentType != "application/octet-stream" {
		t.Fatalf("uploaded body=%q size=%d contentType=%q", store.body, store.size, store.contentType)
	}
	if data["object_key"] != store.key || data["url"] != "https://cdn.example.test/materials/"+store.key ||
		data["name"] != "screen shot.PNG" || data["size"] != float64(5) {
		t.Fatalf("upload response=%v", data)
	}

	csv := qualityMaterialRequest(t, fix, "owner", fix.requirement.ID,
		"1.4 需求迭代.csv", []byte("title,status\n示例,todo"))
	requireStatus(t, csv, http.StatusCreated)
	if !strings.HasSuffix(store.key, ".csv") {
		t.Fatalf("csv object key=%q", store.key)
	}
	video := qualityMaterialRequest(t, fix, "owner", fix.requirement.ID,
		"oceans.mp4", []byte("representative mp4 payload"))
	requireStatus(t, video, http.StatusCreated)
	if !strings.HasSuffix(store.key, ".mp4") {
		t.Fatalf("video object key=%q", store.key)
	}
	taskImage := taskRichTextMaterialRequest(
		t, fix, "owner", 0, "task-proof.png", []byte("task proof"),
	)
	requireStatus(t, taskImage, http.StatusCreated)
	taskKeyPattern := regexp.MustCompile(`^pm/workspace/1/task_materials/[0-9]{8}/[a-f0-9]{32}\.png$`)
	if !taskKeyPattern.MatchString(store.key) {
		t.Fatalf("task material object key=%q", store.key)
	}
}

func qualityMaterialRequest(
	t *testing.T,
	fix qualityPermissionFixture,
	actor string,
	requirementID uint,
	filename string,
	content []byte,
) *httptest.ResponseRecorder {
	return requirementMaterialRequest(
		t, fix, actor, requirementID, filename, content,
		"/api/v1/platform/quality/bugs/materials/upload",
	)
}

func taskRichTextMaterialRequest(
	t *testing.T,
	fix qualityPermissionFixture,
	actor string,
	requirementID uint,
	filename string,
	content []byte,
) *httptest.ResponseRecorder {
	return requirementMaterialRequest(
		t, fix, actor, requirementID, filename, content,
		"/api/v1/platform/tasks/materials/upload",
	)
}

func requirementMaterialRequest(
	t *testing.T,
	fix qualityPermissionFixture,
	actor string,
	requirementID uint,
	filename string,
	content []byte,
	path string,
) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if requirementID > 0 {
		if err := writer.WriteField("requirement_id", fmt.Sprint(requirementID)); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+fix.app.token(t, fix.users[actor]))
	request.Header.Set("X-Workspace-ID", fmt.Sprint(fix.workspaceID))
	response := httptest.NewRecorder()
	fix.app.engine.ServeHTTP(response, request)
	return response
}

func seedForeignQualityUploadScope(t *testing.T, fix qualityPermissionFixture) requirementgorm.Requirement {
	t.Helper()
	workspace := identitygorm.Workspace{ID: 2, Name: "Foreign", Slug: "quality-foreign", Enabled: true}
	team := identitygorm.Team{ID: 20, WorkspaceID: workspace.ID, Name: "Foreign team"}
	project := projectgorm.ProjectSet{ID: 21, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "Foreign project"}
	plan := projectgorm.PlanUnit{ID: 22, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "Foreign plan"}
	requirement := requirementgorm.Requirement{ID: 23, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, PlanUnitID: plan.ID, Title: "Foreign requirement"}
	for _, row := range []any{&workspace, &team, &project, &plan, &requirement} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return requirement
}
