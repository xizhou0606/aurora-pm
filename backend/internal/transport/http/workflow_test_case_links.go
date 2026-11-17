package httpapi

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

// saveWorkflowTestCaseMaterialLink stores an external HTTP(S) link as a test
// case material of the workflow's requirement, mirroring the design-link flow.
func (s *Server) saveWorkflowTestCaseMaterialLink(
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	node workflowNode,
	requirementRow requirementgorm.Requirement,
	link string,
) {
	if !validHTTPURL(link) {
		bad(c, "test case url must be http or https")
		return
	}
	row, err := s.saveRequirementMaterial(requirement.MaterialInput{
		WorkspaceID: requirementRow.WorkspaceID, RequirementID: requirementRow.ID,
		Kind: requirement.MaterialKindTestCase, Source: requirement.MaterialSourceWorkflow,
		Name:        workflowTestCaseLinkName(link, c.PostForm("name")),
		ContentType: workflowTestReportLinkContentType,
		ObjectKey:   fmt.Sprintf("link/%d/%d/%d", instance.WorkspaceID, instance.ID, time.Now().UTC().UnixNano()),
		URL:         link, UploaderUserID: currentFactUserID(c),
	})
	if err != nil {
		dbFail(c)
		return
	}
	created(c, materialAsWorkflowArtifactDTO(row, instance.ID, node.ID))
}

func workflowTestCaseLinkName(link, posted string) string {
	name := strings.TrimSpace(posted)
	if name == "" || strings.Contains(name, "://") {
		name = workflowMaterialLinkDisplayName(link)
	} else {
		name = safeMaterialFilename(name)
	}
	runes := []rune(name)
	if len(runes) > 220 {
		return string(runes[:200]) + "…"
	}
	return name
}

func workflowMaterialLinkDisplayName(link string) string {
	parsed, err := url.Parse(link)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return "测试用例链接"
	}
	name := parsed.Host
	if path := strings.Trim(parsed.Path, "/"); path != "" {
		name += "/" + path
	}
	return name
}
