package httpapi

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const requirementDesignImageMaxBytes int64 = 20 * 1024 * 1024

func (s *Server) uploadRequirementDesignAsset(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	requirementRow, project, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, requirementDesignImageMaxBytes+1024*1024)
	if err := c.Request.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		bad(c, "invalid multipart form")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil || fileHeader.Size <= 0 {
		bad(c, "design image required")
		return
	}
	if fileHeader.Size > requirementDesignImageMaxBytes {
		bad(c, "design image too large (max 20MB)")
		return
	}
	if s.materialStore == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "object storage is required for design image upload")
		return
	}
	source, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to open design image")
		return
	}
	defer source.Close()
	buffered := bufio.NewReader(source)
	header, peekErr := buffered.Peek(512)
	if peekErr != nil && len(header) == 0 {
		bad(c, "design image is empty")
		return
	}
	contentType := strings.ToLower(http.DetectContentType(header))
	allowedContentTypes := map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}
	if !allowedContentTypes[contentType] {
		bad(c, "design asset must be PNG, JPG, WebP, or GIF")
		return
	}
	unique, err := randomMaterialHex(16)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to prepare design image upload")
		return
	}
	objectKey := buildRequirementDesignAssetKey(
		requirementRow.WorkspaceID, requirementRow.TeamID, project.ID, s.cfg.ObjectStorage,
		time.Now().UTC(), unique, safeMaterialExtension(fileHeader.Filename),
	)
	if err := s.materialStore.PutObject(c.Request.Context(), objectKey, buffered, fileHeader.Size, contentType); err != nil {
		fail(c, http.StatusInternalServerError, "design image upload failed")
		return
	}
	publicURL, valid := objectstore.PublicURL(s.cfg.ObjectStorage, objectKey)
	if !valid {
		fail(c, http.StatusInternalServerError, "failed to build design image url")
		return
	}
	if _, err := s.saveRequirementMaterial(requirement.MaterialInput{
		WorkspaceID: requirementRow.WorkspaceID, RequirementID: requirementRow.ID,
		Kind: requirement.MaterialKindDesign, Source: requirement.MaterialSourceRequirement,
		Name: safeMaterialFilename(fileHeader.Filename), ContentType: contentType, Size: fileHeader.Size,
		ObjectKey: objectKey, URL: publicURL, UploaderUserID: currentFactUserID(c),
	}); err != nil {
		dbFail(c)
		return
	}
	created(c, gin.H{
		"url": publicURL, "name": safeMaterialFilename(fileHeader.Filename),
		"content_type": contentType, "size": fileHeader.Size,
	})
}

func buildRequirementDesignAssetKey(workspaceID, teamID, projectID uint, cfg config.ObjectStorage, now time.Time, unique, extension string) string {
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
	segments = append(segments, "design_assets", now.UTC().Format("20060102"), unique+extension)
	return filepath.ToSlash(strings.Join(segments, "/"))
}

func validHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && parsed.Host != "" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}
