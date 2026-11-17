package httpapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const workflowTestCaseArtifactMaxBytes int64 = 20 * 1024 * 1024

type workflowArtifactObjectReader interface {
	GetObject(context.Context, string) (io.ReadCloser, error)
}

func (s *Server) listWorkflowTestCaseArtifacts(c *gin.Context) {
	instance, node, _, ok := s.workflowTestCaseArtifactNode(c, false)
	if !ok {
		return
	}
	requirementRow, _, allowed := s.requirementFromWorkflowInstance(c, instance)
	if !allowed {
		return
	}
	rows, err := s.materials.List(instance.WorkspaceID, requirementRow.ID, requirement.MaterialKindTestCase)
	if err != nil {
		dbFail(c)
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, materialAsWorkflowArtifactDTO(row, instance.ID, node.ID))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func (s *Server) downloadWorkflowNodeArtifact(c *gin.Context) {
	s.downloadWorkflowNodeArtifactKind(c, true)
}

func (s *Server) downloadWorkflowTestCaseArtifact(c *gin.Context) {
	s.downloadWorkflowNodeArtifactKind(c, false)
}

func (s *Server) downloadWorkflowNodeArtifactKind(c *gin.Context, testReport bool) {
	artifactID, valid := paramID(c, "artifactId")
	if !valid {
		return
	}
	workspace, _ := currentWorkspace(c)
	var artifact workflowgorm.WorkflowNodeArtifact
	if err := s.db.Where("id = ? AND workspace_id = ?", artifactID, workspace.ID).First(&artifact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, "workflow node artifact not found")
			return
		}
		dbFail(c)
		return
	}
	if !s.workflowNodeArtifactParentExists(artifact) {
		fail(c, http.StatusNotFound, "workflow node artifact not found")
		return
	}
	if (artifact.Kind == workflowTestReportArtifactKind) != testReport {
		fail(c, http.StatusNotFound, "workflow node artifact not found")
		return
	}
	if isWorkflowTestReportLink(artifact) {
		if !validHTTPURL(artifact.URL) {
			fail(c, http.StatusNotFound, "workflow node artifact object not found")
			return
		}
		c.Redirect(http.StatusFound, artifact.URL)
		return
	}
	store, ready := s.materialStore.(workflowArtifactObjectReader)
	if !ready || store == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "object storage is unavailable")
		return
	}
	reader, err := store.GetObject(c.Request.Context(), artifact.ObjectKey)
	if err != nil {
		fail(c, http.StatusNotFound, "workflow node artifact object not found")
		return
	}
	defer reader.Close()
	bufferedReader := bufio.NewReader(reader)
	if _, err := bufferedReader.Peek(1); err != nil {
		if !errors.Is(err, io.EOF) || artifact.Size > 0 {
			fail(c, http.StatusNotFound, "workflow node artifact object not found")
			return
		}
	}
	contentType := strings.TrimSpace(artifact.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": safeWorkflowArtifactName(artifact.Name),
	})
	if disposition == "" {
		disposition = "attachment"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprint(artifact.Size))
	c.Header("Content-Disposition", disposition)
	if _, err := io.Copy(c.Writer, bufferedReader); err != nil {
		_ = c.Error(err)
	}
}

func (s *Server) workflowNodeArtifactParentExists(artifact workflowgorm.WorkflowNodeArtifact) bool {
	var instance workflowgorm.WorkflowInstance
	if s.db.Where("id = ? AND workspace_id = ?", artifact.InstanceID, artifact.WorkspaceID).First(&instance).Error != nil {
		return false
	}
	if !validWorkflowSubject(instance.SubjectType) {
		return false
	}
	_, err := s.workflowArtifactProject(instance)
	return err == nil
}

func (s *Server) uploadWorkflowTestCaseArtifact(c *gin.Context) {
	instance, node, _, ok := s.workflowTestCaseArtifactNode(c, false)
	if !ok {
		return
	}
	selected, selectedErr := s.workflowTestCaseNodeIsSelected(instance, node.ID)
	if selectedErr != nil {
		fail(c, http.StatusConflict, "workflow selected path is unavailable")
		return
	}
	if !selected {
		fail(c, http.StatusConflict, "test case workflow node is outside the selected path")
		return
	}
	requirementRow, project, allowed := s.requirementFromWorkflowInstance(c, instance)
	if !allowed {
		return
	}
	if link := strings.TrimSpace(c.PostForm("url")); link != "" {
		s.saveWorkflowTestCaseMaterialLink(c, instance, node, requirementRow, link)
		return
	}
	row, stored := s.storeRequirementMaterialFile(
		c, requirementRow, project.ID, requirement.MaterialKindTestCase,
		requirement.MaterialSourceWorkflow, currentFactUserID(c), nil,
	)
	if !stored {
		return
	}
	created(c, materialAsWorkflowArtifactDTO(row, instance.ID, node.ID))
}

func safeWorkflowArtifactUploadError(err error, secrets ...string) string {
	if err == nil {
		return ""
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	for _, secret := range secrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	runes := []rune(message)
	if len(runes) > 512 {
		message = string(runes[:512])
	}
	return message
}

func (s *Server) workflowTestCaseNodeIsSelected(instance workflowgorm.WorkflowInstance, nodeID string) (bool, error) {
	var version workflowgorm.WorkflowTemplateVersion
	if s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error != nil {
		return false, errors.New("workflow version is unavailable")
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	if len(issues) > 0 {
		return false, errors.New("workflow definition is unavailable")
	}
	return workflowNodeIsSelected(s.db, definition, instance, nodeID)
}

func (s *Server) workflowTestCaseArtifactNode(c *gin.Context, requireActive bool) (workflowgorm.WorkflowInstance, workflowNode, workflowDefinition, bool) {
	instance, ok := s.workflowInstanceByID(c)
	if !ok {
		return workflowgorm.WorkflowInstance{}, workflowNode{}, workflowDefinition{}, false
	}
	var version workflowgorm.WorkflowTemplateVersion
	if s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error != nil {
		dbFail(c)
		return workflowgorm.WorkflowInstance{}, workflowNode{}, workflowDefinition{}, false
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	nodeID := strings.TrimSpace(c.Param("nodeId"))
	node, found := workflowNodeByID(definition, nodeID)
	if len(issues) > 0 || !found || !workflowNodeSupportsTestCases(node) {
		fail(c, http.StatusNotFound, "test case workflow node not found")
		return workflowgorm.WorkflowInstance{}, workflowNode{}, workflowDefinition{}, false
	}
	if requireActive && (instance.Status != "running" || !containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)) {
		fail(c, http.StatusConflict, "test case workflow node is not active")
		return workflowgorm.WorkflowInstance{}, workflowNode{}, workflowDefinition{}, false
	}
	return instance, node, definition, true
}

func workflowNodeSupportsTestCases(node workflowNode) bool {
	configured, _ := node.Config["test_case_artifact"].(bool)
	return (node.Type == "human_task" || node.Type == "form") && (configured || strings.Contains(node.Name, "测试用例") || strings.Contains(node.Name, "测试案例"))
}

func (s *Server) workflowArtifactProject(instance workflowgorm.WorkflowInstance) (projectgorm.ProjectSet, error) {
	projectID := s.workflowSubjectProjectID(s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
	var project projectgorm.ProjectSet
	err := s.db.Where("id = ? AND workspace_id = ?", projectID, instance.WorkspaceID).First(&project).Error
	return project, err
}

func workflowTestCaseContentType(extension string) (string, bool) {
	values := map[string]string{
		".csv": "text/csv; charset=utf-8", ".xls": "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".pdf":  "application/pdf",
	}
	value, ok := values[extension]
	return value, ok
}

func validTestCaseFileSignature(extension string, header []byte) bool {
	switch extension {
	case ".xlsx":
		return len(header) >= 4 && string(header[:2]) == "PK"
	case ".xls":
		return len(header) >= 8 && string(header[:8]) == "\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"
	case ".csv":
		return len(header) > 0 && validTestCaseUTF8Chunk(header)
	case ".pdf":
		return len(header) >= 5 && string(header[:4]) == "%PDF"
	default:
		return false
	}
}

func validTestCaseUTF8Chunk(raw []byte) bool {
	for len(raw) > 0 {
		if !utf8.FullRune(raw) {
			return true
		}
		r, size := utf8.DecodeRune(raw)
		if r == utf8.RuneError && size == 1 || r == 0 {
			return false
		}
		raw = raw[size:]
	}
	return true
}

func validTestCaseCSV(reader *bufio.Reader) bool {
	for {
		r, size, err := reader.ReadRune()
		if errors.Is(err, io.EOF) {
			return true
		}
		if err != nil || r == 0 || r == utf8.RuneError && size == 1 {
			return false
		}
	}
}

func safeWorkflowArtifactName(filename string) string {
	name := safeMaterialFilename(filename)
	if name == "" {
		return "test-cases.csv"
	}
	runes := []rune(name)
	if len(runes) > 220 {
		extension := filepath.Ext(name)
		name = string(runes[:200]) + extension
	}
	return name
}

func buildWorkflowTestCaseArtifactKey(instance workflowgorm.WorkflowInstance, nodeID string, project projectgorm.ProjectSet, cfg config.ObjectStorage, now time.Time, unique, extension string) string {
	segments := make([]string, 0, 10)
	if prefix := strings.Trim(strings.TrimSpace(cfg.GlobalPrefix), "/"); prefix != "" {
		segments = append(segments, prefix)
	}
	segments = append(segments, fmt.Sprintf("workspace/%d", instance.WorkspaceID))
	if cfg.TeamPrefix && project.TeamID > 0 {
		segments = append(segments, fmt.Sprintf("team/%d", project.TeamID))
	}
	segments = append(segments, fmt.Sprintf("project/%d", project.ID), "workflow_test_cases", fmt.Sprintf("instance/%d", instance.ID), safePathSegment(nodeID), now.Format("20060102"), unique+extension)
	return filepath.ToSlash(strings.Join(segments, "/"))
}

func safePathSegment(value string) string {
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.TrimSpace(value))
	if value == "" {
		return "node"
	}
	return value
}

func workflowNodeArtifactDTO(row workflowgorm.WorkflowNodeArtifact) gin.H {
	artifactURL := fmt.Sprintf("/api/v1/platform/pm/workflows/test-case-artifacts/%d/download", row.ID)
	source := "file"
	if isWorkflowTestReportLink(row) {
		artifactURL = row.URL
		source = "link"
	} else if row.Kind == workflowTestReportArtifactKind {
		artifactURL = fmt.Sprintf("/api/v1/platform/pm/workflow-node-artifacts/%d/download", row.ID)
	}
	return gin.H{
		"id": row.ID, "instance_id": row.InstanceID, "node_id": row.NodeID, "kind": row.Kind,
		"name": row.Name, "content_type": row.ContentType, "size": row.Size,
		"url": artifactURL, "source": source, "ai_run_id": row.AIRunID,
		"uploader_user_id": row.UploaderUserID, "created_at": row.CreatedAt,
	}
}
