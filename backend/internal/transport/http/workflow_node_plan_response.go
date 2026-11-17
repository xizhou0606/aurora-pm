package httpapi

import (
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func workflowNodePlanDatabaseConflict(err error) bool {
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"deadlock", "serialization failure", "could not serialize", "sqlstate 40001",
		"sqlstate 40p01", "database is locked", "database table is locked", "error 1213",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (s *Server) respondWorkflowNodePlans(
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	version workflowgorm.WorkflowTemplateVersion,
) {
	var runs []workflowgorm.WorkflowNodeRun
	var approvals []workflowgorm.WorkflowApproval
	var template workflowgorm.WorkflowTemplate
	if s.db.Where("id = ? AND workspace_id = ?", instance.ID, instance.WorkspaceID).First(&instance).Error != nil ||
		s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error != nil ||
		s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&approvals).Error != nil ||
		s.db.Where("id = ? AND workspace_id = ?", instance.TemplateID, instance.WorkspaceID).First(&template).Error != nil {
		dbFail(c)
		return
	}
	dto := s.workflowInstanceDTOWithDefinition(instance, runs, approvals, definition)
	ok(c, attachWorkflowTemplateMeta(dto, template, version))
}

func sortedWorkflowNodePlanKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
