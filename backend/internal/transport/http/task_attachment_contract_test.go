package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type fakeTaskObjectStore struct {
	objects map[string][]byte
	removed []string
}

func (f *fakeTaskObjectStore) PutObject(
	_ context.Context, key string, body io.Reader, _ int64, _ string,
) error {
	if f.objects == nil {
		f.objects = map[string][]byte{}
	}
	f.objects[key], _ = io.ReadAll(body)
	return nil
}

func (f *fakeTaskObjectStore) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	value, exists := f.objects[key]
	if !exists {
		return nil, fmt.Errorf("missing")
	}
	return io.NopCloser(bytes.NewReader(value)), nil
}

func (f *fakeTaskObjectStore) RemoveObject(_ context.Context, key string) error {
	if _, exists := f.objects[key]; !exists {
		return fmt.Errorf("missing")
	}
	delete(f.objects, key)
	f.removed = append(f.removed, key)
	return nil
}

func TestTaskAttachmentUploadListDownloadDeleteAndIsolation(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "attachment-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	local := taskgorm.WorkTask{
		ID: 301, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-301", Title: "Local",
		Status: "draft", Priority: "normal",
	}
	foreign := taskgorm.WorkTask{
		ID: 302, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
		RequirementID: 23, Kind: "task", TaskKey: "B-302", Title: "Foreign",
		Status: "draft", Priority: "normal",
	}
	if err := app.db.Create(&local).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	token := app.token(t, admin)
	disabled := taskAttachmentUploadRequest(t, app, token, local.ID, "proof.txt", []byte("proof"))
	requireStatus(t, disabled, http.StatusServiceUnavailable)

	store := &fakeTaskObjectStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "minio.example.test", Bucket: "pm",
		AccessKey: "access", SecretKey: "secret", GlobalPrefix: "pm", TeamPrefix: true,
	}
	app.rebuildEngine(t, store, nil)
	uploaded := taskAttachmentUploadRequest(t, app, token, local.ID, "proof.txt", []byte("proof"))
	requireStatus(t, uploaded, http.StatusCreated)
	data := decodeData(t, uploaded).(map[string]any)
	attachmentID := uint(data["id"].(float64))
	if data["uploader_user_id"] != float64(admin.ID) || data["storage_backend"] != "s3" {
		t.Fatalf("attachment response=%s", uploaded.Body.String())
	}
	var stored taskgorm.WorkTaskAttachment
	if err := app.db.First(&stored, attachmentID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.WorkspaceID != 1 || !strings.Contains(stored.ObjectKey, "/task/301/attachments/") {
		t.Fatalf("attachment scope/key=%+v", stored)
	}

	listed := app.request(t, http.MethodGet, "/api/v1/platform/tasks/301/attachments", token, 1, nil)
	requireStatus(t, listed, http.StatusOK)
	if len(decodeData(t, listed).([]any)) != 1 {
		t.Fatalf("attachment list=%s", listed.Body.String())
	}
	downloaded := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/attachments/"+uintString(attachmentID)+"/download", token, 1, nil)
	requireStatus(t, downloaded, http.StatusOK)
	if downloaded.Body.String() != "proof" || downloaded.Header().Get("Content-Disposition") == "" {
		t.Fatalf("download body/headers=%q %v", downloaded.Body.String(), downloaded.Header())
	}

	foreignAttachment := taskgorm.WorkTaskAttachment{
		ID: 999, WorkspaceID: 2, WorkTaskID: foreign.ID, UploaderUserID: admin.ID,
		Filename: "foreign.txt", Size: 6, ObjectKey: "foreign", StorageBackend: "s3",
	}
	if err := app.db.Create(&foreignAttachment).Error; err != nil {
		t.Fatal(err)
	}
	isolated := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/attachments/999/download", token, 1, nil)
	requireStatus(t, isolated, http.StatusNotFound)

	deleted := app.request(t, http.MethodDelete,
		"/api/v1/platform/tasks/attachments/"+uintString(attachmentID), token, 1, nil)
	requireStatus(t, deleted, http.StatusNoContent)
	if len(store.removed) != 1 {
		t.Fatalf("removed keys=%v", store.removed)
	}
	var count int64
	app.db.Model(&taskgorm.WorkTaskAttachment{}).Where("id = ?", attachmentID).Count(&count)
	if count != 0 {
		t.Fatalf("attachment metadata remained=%d", count)
	}
}

func taskAttachmentUploadRequest(
	t *testing.T, app *testApp, token string, taskID uint, filename string, content []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/platform/tasks/"+uintString(taskID)+"/attachments", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", "1")
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}
