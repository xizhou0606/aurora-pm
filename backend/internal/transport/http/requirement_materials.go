package httpapi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	requirementhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/http"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func (s *Server) listRequirementMaterials(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, _, allowed := s.requirementByID(c, id); !allowed {
		return
	}
	s.requirementMaterials.List(c)
}

func (s *Server) deleteRequirementMaterial(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, _, allowed := s.requirementByID(c, id); !allowed {
		return
	}
	s.requirementMaterials.Delete(c)
}

func (s *Server) uploadRequirementOwnedMaterial(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	requirementRow, project, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	kind, err := requirement.ParseMaterialKind(c.PostForm("kind"))
	if err != nil {
		kind, err = requirement.ParseMaterialKind(c.Query("kind"))
	}
	if err != nil {
		bad(c, "kind must be design or test_case")
		return
	}
	source := strings.TrimSpace(c.PostForm("source"))
	if source == "" {
		source = requirement.MaterialSourceRequirement
	}
	if _, err := requirement.ParseMaterialSource(source); err != nil {
		bad(c, "source is invalid")
		return
	}
	if link := strings.TrimSpace(c.PostForm("url")); link != "" && (kind == requirement.MaterialKindDesign || kind == requirement.MaterialKindTestCase) {
		if !validHTTPURL(link) {
			bad(c, "material url must be http or https")
			return
		}
		name := designLinkMaterialName(link, c.PostForm("name"))
		row, createErr := s.saveRequirementMaterial(requirement.MaterialInput{
			WorkspaceID: requirementRow.WorkspaceID, RequirementID: requirementRow.ID,
			Kind: kind, Source: source, Name: name,
			ContentType: "text/uri-list", ObjectKey: fmt.Sprintf("link/%d/%d/%d", requirementRow.WorkspaceID, requirementRow.ID, time.Now().UTC().UnixNano()),
			URL: link, UploaderUserID: currentFactUserID(c),
		})
		if createErr != nil {
			dbFail(c)
			return
		}
		if kind == requirement.MaterialKindDesign {
			if err := s.backfillRequirementDesignURL(requirementRow.WorkspaceID, requirementRow.ID, link); err != nil {
				dbFail(c)
				return
			}
		}
		created(c, requirementhttp.MaterialDTO(row))
		return
	}
	row, stored := s.storeRequirementMaterialFile(c, requirementRow, project.ID, kind, source, currentFactUserID(c), nil)
	if !stored {
		return
	}
	created(c, requirementhttp.MaterialDTO(row))
}

func (s *Server) downloadRequirementMaterial(c *gin.Context) {
	requirementID, ok := paramID(c, "id")
	if !ok {
		return
	}
	materialID, ok := paramID(c, "materialId")
	if !ok {
		return
	}
	if _, _, allowed := s.requirementByID(c, requirementID); !allowed {
		return
	}
	workspace, ok := currentWorkspace(c)
	if !ok {
		return
	}
	row, err := s.materials.Get(workspace.ID, requirementID, materialID)
	if err == requirement.ErrMaterialNotFound {
		fail(c, http.StatusNotFound, "requirement material not found")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	s.writeRequirementMaterialDownload(c, row)
}

func (s *Server) downloadWorkflowTestCaseMaterial(c *gin.Context) {
	requirementID, ok := paramID(c, "requirementId")
	if !ok {
		return
	}
	materialID, ok := paramID(c, "materialId")
	if !ok {
		return
	}
	if _, _, allowed := s.requirementByID(c, requirementID); !allowed {
		return
	}
	workspace, ok := currentWorkspace(c)
	if !ok {
		return
	}
	row, err := s.materials.Get(workspace.ID, requirementID, materialID)
	if err == requirement.ErrMaterialNotFound || row.Kind != requirement.MaterialKindTestCase {
		fail(c, http.StatusNotFound, "workflow test case material not found")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	s.writeRequirementMaterialDownload(c, row)
}

func (s *Server) writeRequirementMaterialDownload(c *gin.Context, row requirement.Material) {
	if strings.HasPrefix(row.ObjectKey, "link/") || (strings.TrimSpace(row.ObjectKey) == "" && validHTTPURL(row.URL)) {
		if !validHTTPURL(row.URL) {
			fail(c, http.StatusNotFound, "requirement material object not found")
			return
		}
		c.Redirect(http.StatusFound, row.URL)
		return
	}
	store, ready := s.materialStore.(workflowArtifactObjectReader)
	if !ready || store == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "object storage is unavailable")
		return
	}
	reader, err := store.GetObject(c.Request.Context(), row.ObjectKey)
	if err != nil {
		fail(c, http.StatusNotFound, "requirement material object not found")
		return
	}
	defer reader.Close()
	contentType := strings.TrimSpace(row.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprint(row.Size))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", row.Name))
	_, _ = io.Copy(c.Writer, reader)
}

func (s *Server) saveRequirementMaterial(input requirement.MaterialInput) (requirement.Material, error) {
	return s.materials.Create(input)
}

func (s *Server) backfillRequirementDesignURL(workspaceID, requirementID uint, url string) error {
	url = strings.TrimSpace(url)
	if url == "" || !validHTTPURL(url) {
		return nil
	}
	return s.db.Model(&requirementgorm.Requirement{}).
		Where("id = ? AND workspace_id = ?", requirementID, workspaceID).
		Update("design_url", url).Error
}

func designLinkMaterialName(link, posted string) string {
	name := strings.TrimSpace(posted)
	if name == "" || strings.Contains(name, "://") {
		name = strings.TrimSpace(link)
	} else {
		name = safeMaterialFilename(name)
	}
	runes := []rune(name)
	if len(runes) > 220 {
		return string(runes[:220])
	}
	return name
}

func (s *Server) resolveRequirementDesignURL(workspaceID, requirementID uint, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if s.materials != nil {
		rows, err := s.materials.List(workspaceID, requirementID, requirement.MaterialKindDesign)
		if err == nil {
			if resolved, ok := requirementhttp.CanonicalDesignURL(rows, raw); ok {
				return resolved, true
			}
		}
	}
	if validHTTPURL(raw) {
		return raw, true
	}
	return "", false
}

func (s *Server) storeRequirementMaterialFile(
	c *gin.Context,
	requirementRow requirementgorm.Requirement,
	projectID uint,
	kind, source string,
	uploaderID uint,
	aiRunID *uint,
) (requirement.Material, bool) {
	maxBytes := requirementDesignImageMaxBytes
	if kind == requirement.MaterialKindTestCase {
		maxBytes = workflowTestCaseArtifactMaxBytes
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes+1024*1024)
	if err := c.Request.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		bad(c, "invalid multipart form")
		return requirement.Material{}, false
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil || fileHeader.Size <= 0 {
		bad(c, "file required")
		return requirement.Material{}, false
	}
	if fileHeader.Size > maxBytes {
		bad(c, "file too large (max 20MB)")
		return requirement.Material{}, false
	}
	if s.materialStore == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "system object storage is required")
		return requirement.Material{}, false
	}
	name := safeMaterialFilename(fileHeader.Filename)
	extension := strings.ToLower(filepath.Ext(name))
	sourceFile, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to open file")
		return requirement.Material{}, false
	}
	defer sourceFile.Close()
	buffered := bufio.NewReader(sourceFile)
	header, peekErr := buffered.Peek(512)
	if peekErr != nil && len(header) == 0 {
		bad(c, "file is empty")
		return requirement.Material{}, false
	}
	contentType := ""
	switch kind {
	case requirement.MaterialKindDesign:
		contentType = strings.ToLower(http.DetectContentType(header))
		if !map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}[contentType] {
			bad(c, "design asset must be PNG, JPG, WebP, or GIF")
			return requirement.Material{}, false
		}
	case requirement.MaterialKindTestCase:
		var validType bool
		contentType, validType = workflowTestCaseContentType(extension)
		if !validType {
			bad(c, "test case file must be CSV, XLS, XLSX, or PDF")
			return requirement.Material{}, false
		}
		if !validTestCaseFileSignature(extension, header) {
			bad(c, "test case file content does not match its extension")
			return requirement.Material{}, false
		}
	}
	unique, err := randomMaterialHex(16)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to prepare upload")
		return requirement.Material{}, false
	}
	objectKey := buildRequirementOwnedMaterialKey(requirementRow, projectID, kind, s.cfg.ObjectStorage, time.Now().UTC(), unique, extension)
	if err := s.materialStore.PutObject(c.Request.Context(), objectKey, buffered, fileHeader.Size, contentType); err != nil {
		failCode(c, http.StatusBadGateway, apiErrorObjectStorageUploadFailed, "material upload failed")
		return requirement.Material{}, false
	}
	publicURL, valid := objectstore.PublicURL(s.cfg.ObjectStorage, objectKey)
	if !valid {
		fail(c, http.StatusInternalServerError, "failed to build file url")
		return requirement.Material{}, false
	}
	row, err := s.saveRequirementMaterial(requirement.MaterialInput{
		WorkspaceID: requirementRow.WorkspaceID, RequirementID: requirementRow.ID,
		Kind: kind, Source: source, Name: name, ContentType: contentType, Size: fileHeader.Size,
		ObjectKey: objectKey, URL: publicURL, AIRunID: aiRunID, UploaderUserID: uploaderID,
	})
	if err != nil {
		dbFail(c)
		return requirement.Material{}, false
	}
	if kind == requirement.MaterialKindDesign {
		if err := s.backfillRequirementDesignURL(requirementRow.WorkspaceID, requirementRow.ID, publicURL); err != nil {
			dbFail(c)
			return requirement.Material{}, false
		}
	}
	_ = s.recordMutation(s.db, c, projectID, "requirement_material", row.ID, "uploaded", row.Name, gin.H{
		"requirement_id": requirementRow.ID, "kind": kind, "source": source,
	})
	return row, true
}

func buildRequirementOwnedMaterialKey(
	requirementRow requirementgorm.Requirement,
	projectID uint,
	kind string,
	cfg config.ObjectStorage,
	now time.Time,
	unique, extension string,
) string {
	namespace := unique + "-design"
	if kind == requirement.MaterialKindTestCase {
		namespace = unique + "-cases"
	}
	return buildRequirementDesignAssetKey(requirementRow.WorkspaceID, requirementRow.TeamID, projectID, cfg, now, namespace, extension)
}

func (s *Server) requirementFromWorkflowInstance(
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
) (requirementgorm.Requirement, projectgorm.ProjectSet, bool) {
	if instance.SubjectType != "requirement" {
		fail(c, http.StatusConflict, "workflow subject is not a requirement")
		return requirementgorm.Requirement{}, projectgorm.ProjectSet{}, false
	}
	return s.requirementByID(c, instance.SubjectID)
}

func materialAsWorkflowArtifactDTO(row requirement.Material, instanceID uint, nodeID string) gin.H {
	kind := "uploaded"
	if row.Source == requirement.MaterialSourceAIGenerated {
		kind = "ai_generated"
	}
	dto := requirementhttp.MaterialDTO(row)
	dto["instance_id"] = instanceID
	dto["node_id"] = nodeID
	dto["kind"] = kind
	if requirementhttp.IsMaterialLink(row) {
		dto["url"] = row.URL
		dto["source"] = "link"
		return dto
	}
	dto["url"] = fmt.Sprintf(
		"/api/v1/platform/pm/workflows/test-case-materials/%d/%d/download",
		row.RequirementID,
		row.ID,
	)
	return dto
}
