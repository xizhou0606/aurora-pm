package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

const (
	workflowTestReportLinkContentType = "text/uri-list"
	workflowTestReportLinkMaxRunes    = 2048
)

type workflowTestReportLinkBody struct {
	URL string `json:"url"`
}

func (s *Server) createWorkflowTestReportLink(c *gin.Context) {
	_, _, _, ok := s.workflowTestReportArtifactNode(c, false)
	if !ok {
		return
	}
	fail(c, http.StatusGone, "test reports must be uploaded in testing management by plan unit")
}

func isWorkflowTestReportLink(row workflowgorm.WorkflowNodeArtifact) bool {
	return row.Kind == workflowTestReportArtifactKind &&
		(row.ContentType == workflowTestReportLinkContentType || strings.HasPrefix(row.ObjectKey, "link/"))
}

func workflowTestReportLinkObjectKey(instance workflowgorm.WorkflowInstance, nodeID, unique string) string {
	return fmt.Sprintf("link/workspace/%d/instance/%d/%s/%s", instance.WorkspaceID, instance.ID, safePathSegment(nodeID), unique)
}

func workflowTestReportLinkName(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return "测试报告链接"
	}
	name := parsed.Host
	if path := strings.Trim(parsed.Path, "/"); path != "" {
		name += "/" + path
	}
	runes := []rune(name)
	if len(runes) > 220 {
		return string(runes[:200]) + "…"
	}
	return name
}
