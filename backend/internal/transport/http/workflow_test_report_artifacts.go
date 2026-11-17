package httpapi

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const (
	workflowTestReportArtifactKind     = "test_report"
	workflowTestReportArtifactMaxBytes = 20 * 1024 * 1024
)

func (s *Server) listWorkflowTestReportArtifacts(c *gin.Context) {
	if _, _, _, ok := s.workflowTestReportArtifactNode(c, false); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": []gin.H{}, "total": 0})
}

func (s *Server) workflowTestReportWriteContext(c *gin.Context) (workflowgorm.WorkflowInstance, workflowNode, bool) {
	instance, node, _, ok := s.workflowTestReportArtifactNode(c, false)
	if !ok {
		return workflowgorm.WorkflowInstance{}, workflowNode{}, false
	}
	selected, selectedErr := s.workflowTestCaseNodeIsSelected(instance, node.ID)
	if selectedErr != nil {
		fail(c, http.StatusConflict, "workflow selected path is unavailable")
		return workflowgorm.WorkflowInstance{}, workflowNode{}, false
	}
	if !selected {
		fail(c, http.StatusConflict, "test report workflow node is outside the selected path")
		return workflowgorm.WorkflowInstance{}, workflowNode{}, false
	}
	if !s.requireWorkflowNodeActive(c, instance, node.ID, containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)) {
		return workflowgorm.WorkflowInstance{}, workflowNode{}, false
	}
	return instance, node, true
}

func (s *Server) uploadWorkflowTestReportArtifact(c *gin.Context) {
	_, _, _, ok := s.workflowTestReportArtifactNode(c, false)
	if !ok {
		return
	}
	fail(c, http.StatusGone, "test reports must be uploaded in testing management by plan unit")
}

func (s *Server) workflowTestReportArtifactNode(c *gin.Context, requireActive bool) (workflowgorm.WorkflowInstance, workflowNode, workflowDefinition, bool) {
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
	if len(issues) > 0 || !found || !workflowNodeSupportsTestReports(node) {
		fail(c, http.StatusNotFound, "test report workflow node not found")
		return workflowgorm.WorkflowInstance{}, workflowNode{}, workflowDefinition{}, false
	}
	if requireActive && (instance.Status != "running" || !containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)) {
		fail(c, http.StatusConflict, "test report workflow node is not active")
		return workflowgorm.WorkflowInstance{}, workflowNode{}, workflowDefinition{}, false
	}
	return instance, node, definition, true
}

func workflowNodeSupportsTestReports(node workflowNode) bool {
	configured, _ := node.Config["test_report_artifact"].(bool)
	return (node.Type == "human_task" || node.Type == "form") && (configured || strings.Contains(node.Name, "测试验收") || strings.Contains(node.Name, "测试报告"))
}

func workflowNodeHasTestReport(tx *gorm.DB, instance workflowgorm.WorkflowInstance, nodeID string) (bool, error) {
	var count int64
	err := tx.Model(&workflowgorm.WorkflowNodeArtifact{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND kind = ?", instance.WorkspaceID, instance.ID, nodeID, workflowTestReportArtifactKind).
		Count(&count).Error
	return count > 0, err
}

func workflowTestReportContentType(extension string) (string, bool) {
	values := map[string]string{
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	value, ok := values[extension]
	return value, ok
}

func validTestReportFileSignature(extension string, header []byte) bool {
	switch extension {
	case ".pdf":
		return len(header) >= 4 && string(header[:4]) == "%PDF"
	case ".docx", ".xlsx":
		return len(header) >= 4 && string(header[:2]) == "PK"
	case ".doc":
		return len(header) >= 8 && string(header[:8]) == "\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"
	default:
		return false
	}
}

func safeWorkflowTestReportName(filename string) string {
	name := safeMaterialFilename(filename)
	if name == "" {
		return "test-report.pdf"
	}
	runes := []rune(name)
	if len(runes) > 220 {
		extension := filepath.Ext(name)
		name = string(runes[:200]) + extension
	}
	return name
}

func buildWorkflowTestReportArtifactKey(instance workflowgorm.WorkflowInstance, nodeID string, project projectgorm.ProjectSet, cfg config.ObjectStorage, now time.Time, unique, extension string) string {
	segments := make([]string, 0, 10)
	if prefix := strings.Trim(strings.TrimSpace(cfg.GlobalPrefix), "/"); prefix != "" {
		segments = append(segments, prefix)
	}
	segments = append(segments, fmt.Sprintf("workspace/%d", instance.WorkspaceID))
	if cfg.TeamPrefix && project.TeamID > 0 {
		segments = append(segments, fmt.Sprintf("team/%d", project.TeamID))
	}
	segments = append(segments, fmt.Sprintf("project/%d", project.ID), "workflow_test_reports", fmt.Sprintf("instance/%d", instance.ID), safePathSegment(nodeID), now.Format("20060102"), unique+extension)
	return filepath.ToSlash(strings.Join(segments, "/"))
}
