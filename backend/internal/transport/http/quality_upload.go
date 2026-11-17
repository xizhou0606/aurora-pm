package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const qualityMaterialMaxBytes int64 = 200 * 1024 * 1024

var safeMaterialExtensionPattern = regexp.MustCompile(`^\.[a-zA-Z0-9]{1,16}$`)

type materialObjectStore interface {
	PutObject(context.Context, string, io.Reader, int64, string) error
}

func (s *Server) uploadQualityBugMaterial(c *gin.Context) {
	s.uploadRequirementMaterial(c, "bug_materials")
}

func (s *Server) uploadTaskRichTextMaterial(c *gin.Context) {
	s.uploadRequirementMaterial(c, "task_materials")
}

func (s *Server) uploadRequirementMaterial(c *gin.Context, namespace string) {
	// Leave enough room for the multipart envelope while enforcing the exact
	// 200 MB limit against the uploaded file itself below.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, qualityMaterialMaxBytes+4*1024*1024)
	if err := c.Request.ParseMultipartForm(32 * 1024 * 1024); err != nil {
		bad(c, "invalid multipart form")
		return
	}
	workspace, _ := currentWorkspace(c)
	var teamID, projectID uint
	if rawRequirementID := strings.TrimSpace(c.PostForm("requirement_id")); rawRequirementID != "" {
		requirementID, err := strconv.ParseUint(rawRequirementID, 10, 64)
		if err != nil || requirementID == 0 {
			bad(c, "invalid requirement_id")
			return
		}
		var requirement requirementgorm.Requirement
		if s.db.Where("id = ? AND workspace_id = ?", uint(requirementID), workspace.ID).
			First(&requirement).Error != nil {
			fail(c, http.StatusNotFound, "requirement not found")
			return
		}
		var project projectgorm.ProjectSet
		if s.db.Where("id = ? AND workspace_id = ?", requirement.ProjectSetID, workspace.ID).
			First(&project).Error != nil {
			fail(c, http.StatusNotFound, "project set not found")
			return
		}
		if err := s.validateWorkspaceHierarchy(workspace.ID, project, requirement); err != nil {
			bad(c, err.Error())
			return
		}
		if !s.requireProjectAccess(c, workspace.ID, project.ID) {
			return
		}
		teamID, projectID = project.TeamID, project.ID
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil || fileHeader.Size <= 0 {
		bad(c, "file required")
		return
	}
	if fileHeader.Size > qualityMaterialMaxBytes {
		bad(c, "file too large (max 200MB)")
		return
	}
	if s.materialStore == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "object storage is required for material upload")
		return
	}
	source, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to open material")
		return
	}
	defer source.Close()
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	unique, err := randomMaterialHex(16)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to prepare material upload")
		return
	}
	extension := safeMaterialExtension(fileHeader.Filename)
	objectKey := buildRequirementMaterialKey(
		workspace.ID, teamID, projectID, s.cfg.ObjectStorage,
		time.Now().UTC(), namespace, unique, extension,
	)
	if err := s.materialStore.PutObject(
		c.Request.Context(), objectKey, source, fileHeader.Size, contentType,
	); err != nil {
		fail(c, http.StatusServiceUnavailable, "material storage is temporarily unavailable")
		return
	}
	publicURL, valid := objectstore.PublicURL(s.cfg.ObjectStorage, objectKey)
	if !valid {
		fail(c, http.StatusInternalServerError, "failed to build material url")
		return
	}
	created(c, gin.H{
		"url": publicURL, "object_key": objectKey, "object_storage_profile_id": 0,
		"name": safeMaterialFilename(fileHeader.Filename), "content_type": contentType,
		"size": fileHeader.Size,
	})
}

func buildQualityBugMaterialKey(
	workspaceID, teamID, projectID uint,
	cfg config.ObjectStorage,
	now time.Time,
	unique, extension string,
) string {
	return buildRequirementMaterialKey(
		workspaceID, teamID, projectID, cfg, now, "bug_materials", unique, extension,
	)
}

func buildRequirementMaterialKey(
	workspaceID, teamID, projectID uint,
	cfg config.ObjectStorage,
	now time.Time,
	namespace, unique, extension string,
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
	segments = append(segments, namespace, now.UTC().Format("20060102"), unique+extension)
	return strings.Join(segments, "/")
}

func safeMaterialExtension(filename string) string {
	extension := strings.ToLower(filepath.Ext(safeMaterialFilename(filename)))
	if !safeMaterialExtensionPattern.MatchString(extension) {
		return ""
	}
	return extension
}

func safeMaterialFilename(filename string) string {
	return filepath.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
}

func randomMaterialHex(bytesCount int) (string, error) {
	raw := make([]byte, bytesCount)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
