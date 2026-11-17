package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/http"
)

// listWorkflowMaterialProjections exposes requirement-owned delivery material
// through the workflow instance that projects it. The dedicated route keeps
// this read independently grantable from the broad requirement material page.
func (s *Server) listWorkflowMaterialProjections(c *gin.Context) {
	instance, ok := s.workflowInstanceByID(c)
	if !ok {
		return
	}
	requirementRow, _, allowed := s.requirementFromWorkflowInstance(c, instance)
	if !allowed {
		return
	}
	kind := c.Query("kind")
	if kind != "" && kind != requirement.MaterialKindDesign && kind != requirement.MaterialKindTestCase {
		bad(c, "kind must be design or test_case")
		return
	}
	rows, err := s.materials.List(instance.WorkspaceID, requirementRow.ID, kind)
	if err != nil {
		dbFail(c)
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, requirementhttp.MaterialDTO(row))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}
