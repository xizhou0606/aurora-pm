package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const taskAttachmentMaxBytes int64 = 32 * 1024 * 1024

type taskObjectStore interface {
	materialObjectStore
	GetObject(context.Context, string) (io.ReadCloser, error)
	RemoveObject(context.Context, string) error
}

func (s *Server) taskStore() (taskObjectStore, bool) {
	store, ok := s.materialStore.(taskObjectStore)
	return store, ok && store != nil && s.materialStoreErr == nil && s.cfg.ObjectStorage.Enabled
}

func (s *Server) listTaskAttachments(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, _, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	var rows []taskgorm.WorkTaskAttachment
	if s.db.Where("workspace_id = ? AND work_task_id = ? AND mutation_state = ?", task.WorkspaceID, task.ID, "active").
		Order("id DESC").Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	ok(c, rows)
}

func (s *Server) uploadTaskAttachment(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, project, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, taskAttachmentMaxBytes+2*1024*1024)
	if err := c.Request.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		bad(c, "invalid multipart form")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil || fileHeader.Size <= 0 {
		bad(c, "file required")
		return
	}
	if fileHeader.Size > taskAttachmentMaxBytes {
		bad(c, "file too large (max 32MB)")
		return
	}
	store, ready := s.taskStore()
	if !ready {
		fail(c, http.StatusServiceUnavailable, "object storage is required for task attachments")
		return
	}
	source, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to open attachment")
		return
	}
	defer source.Close()
	unique, err := randomMaterialHex(16)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to prepare attachment upload")
		return
	}
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	objectKey := buildTaskAttachmentKey(
		task.WorkspaceID, task.TeamID, project.ID, task.ID, s.cfg.ObjectStorage,
		time.Now().UTC(), unique, safeMaterialExtension(fileHeader.Filename),
	)
	if err := store.PutObject(c.Request.Context(), objectKey, source, fileHeader.Size, contentType); err != nil {
		fail(c, http.StatusInternalServerError, "attachment upload failed")
		return
	}
	row := taskgorm.WorkTaskAttachment{
		WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID, UploaderUserID: currentFactUserID(c),
		Filename: safeMaterialFilename(fileHeader.Filename), ContentType: contentType,
		Size: fileHeader.Size, ObjectKey: objectKey, StorageBackend: "s3",
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		_, err := lockProjectSetTx(tx, task.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		if _, err := lockProjectWorkTaskTx(tx, task.WorkspaceID, task.ID, project.ID); err != nil {
			return err
		}
		return tx.Create(&row).Error
	}); err != nil {
		_ = store.RemoveObject(context.Background(), objectKey)
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	created(c, row)
}

func buildTaskAttachmentKey(
	workspaceID, teamID, projectID, taskID uint,
	cfg config.ObjectStorage,
	now time.Time,
	unique, extension string,
) string {
	segments := make([]string, 0, 10)
	if prefix := strings.Trim(strings.TrimSpace(cfg.GlobalPrefix), "/"); prefix != "" {
		segments = append(segments, prefix)
	}
	segments = append(segments, fmt.Sprintf("workspace/%d", workspaceID))
	if cfg.TeamPrefix && teamID > 0 {
		segments = append(segments, fmt.Sprintf("team/%d", teamID))
	}
	if projectID > 0 {
		segments = append(segments, fmt.Sprintf("project/%d", projectID))
	}
	segments = append(segments, fmt.Sprintf("task/%d", taskID), "attachments", now.Format("20060102"), unique+extension)
	return strings.Join(segments, "/")
}

func (s *Server) downloadTaskAttachment(c *gin.Context) {
	attachment, task, allowed := s.taskAttachmentByID(c)
	if !allowed {
		return
	}
	_ = task
	store, ready := s.taskStore()
	if !ready {
		fail(c, http.StatusServiceUnavailable, "object storage is unavailable")
		return
	}
	reader, err := store.GetObject(c.Request.Context(), attachment.ObjectKey)
	if err != nil {
		fail(c, http.StatusNotFound, "attachment object not found")
		return
	}
	defer reader.Close()
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprint(attachment.Size))
	c.Header("Content-Disposition", fmt.Sprintf(
		`attachment; filename="%s"`, strings.ReplaceAll(filepath.Base(attachment.Filename), `"`, ""),
	))
	_, _ = io.Copy(c.Writer, reader)
}

func (s *Server) deleteTaskAttachment(c *gin.Context) {
	attachment, task, allowed := s.taskAttachmentByID(c)
	if !allowed {
		return
	}
	store, ready := s.taskStore()
	if !ready {
		fail(c, http.StatusServiceUnavailable, "object storage is unavailable")
		return
	}
	if err := store.RemoveObject(c.Request.Context(), attachment.ObjectKey); err != nil {
		fail(c, http.StatusInternalServerError, "attachment delete failed")
		return
	}
	result := s.db.Where(
		"id = ? AND workspace_id = ? AND work_task_id = ?", attachment.ID, task.WorkspaceID, task.ID,
	).Delete(&taskgorm.WorkTaskAttachment{})
	if result.Error != nil || result.RowsAffected != 1 {
		dbFail(c)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) taskAttachmentByID(
	c *gin.Context,
) (taskgorm.WorkTaskAttachment, taskgorm.WorkTask, bool) {
	id, valid := paramID(c, "attachmentId")
	if !valid {
		return taskgorm.WorkTaskAttachment{}, taskgorm.WorkTask{}, false
	}
	workspace, _ := currentWorkspace(c)
	var attachment taskgorm.WorkTaskAttachment
	if s.db.Where("id = ? AND workspace_id = ? AND mutation_state = ?", id, workspace.ID, "active").First(&attachment).Error != nil {
		fail(c, http.StatusNotFound, "attachment not found")
		return taskgorm.WorkTaskAttachment{}, taskgorm.WorkTask{}, false
	}
	task, _, allowed := s.taskByID(c, attachment.WorkTaskID)
	if !allowed || task.WorkspaceID != attachment.WorkspaceID {
		return taskgorm.WorkTaskAttachment{}, taskgorm.WorkTask{}, false
	}
	return attachment, task, true
}
