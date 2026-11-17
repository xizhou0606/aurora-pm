package httpapi

import (
	"bufio"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const workflowOpinionImageMaxBytes int64 = 8 * 1024 * 1024

func (s *Server) uploadWorkflowNodeOpinionImage(c *gin.Context) {
	instance, _, okNode := s.workflowPreReviewOpinionNode(c)
	if !okNode {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, workflowOpinionImageMaxBytes+1024*1024)
	if err := c.Request.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		bad(c, "invalid multipart form")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil || fileHeader.Size <= 0 {
		bad(c, "image required")
		return
	}
	if fileHeader.Size > workflowOpinionImageMaxBytes {
		bad(c, "image too large (max 8MB)")
		return
	}
	if s.materialStore == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "object storage is required for opinion image upload")
		return
	}
	source, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to open image")
		return
	}
	defer source.Close()
	buffered := bufio.NewReader(source)
	header, peekErr := buffered.Peek(512)
	if peekErr != nil && len(header) == 0 {
		bad(c, "image is empty")
		return
	}
	contentType := strings.ToLower(http.DetectContentType(header))
	allowedContentTypes := map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}
	if !allowedContentTypes[contentType] {
		bad(c, "image must be PNG, JPG, WebP, or GIF")
		return
	}
	unique, err := randomMaterialHex(16)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to prepare image upload")
		return
	}
	project, projectErr := s.workflowArtifactProject(instance)
	if projectErr != nil {
		fail(c, http.StatusNotFound, "workflow subject project not found")
		return
	}
	extension := safeMaterialExtension(fileHeader.Filename)
	if extension == "" {
		extension = opinionImageExtension(contentType)
	}
	objectKey := buildWorkflowOpinionImageKey(
		instance.WorkspaceID, project.TeamID, project.ID, s.cfg.ObjectStorage,
		time.Now().UTC(), unique, extension,
	)
	if err := s.materialStore.PutObject(c.Request.Context(), objectKey, buffered, fileHeader.Size, contentType); err != nil {
		fail(c, http.StatusServiceUnavailable, "opinion image storage is temporarily unavailable")
		return
	}
	publicURL, valid := objectstore.PublicURL(s.cfg.ObjectStorage, objectKey)
	if !valid {
		fail(c, http.StatusInternalServerError, "failed to build image url")
		return
	}
	created(c, gin.H{
		"url": publicURL, "name": safeMaterialFilename(fileHeader.Filename),
		"content_type": contentType, "size": fileHeader.Size,
	})
}

func opinionImageExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func buildWorkflowOpinionImageKey(
	workspaceID, teamID, projectID uint,
	cfg config.ObjectStorage,
	now time.Time,
	unique, extension string,
) string {
	segments := make([]string, 0, 8)
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
	segments = append(segments, "opinion_images", now.UTC().Format("20060102"), unique+extension)
	return filepath.ToSlash(strings.Join(segments, "/"))
}
