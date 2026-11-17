package httpapi

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
)

func (s *Server) validateTestCaseDeliveryTarget(
	tx *gorm.DB,
	workspaceID uint,
	subjectType string,
	subjectID uint,
	target qatestcase.Target,
) error {
	var instance workflowgorm.WorkflowInstance
	if err := tx.Where("id = ? AND workspace_id = ?", target.InstanceID, workspaceID).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("workflow instance not found")
		}
		return err
	}
	if instance.SubjectType != subjectType || instance.SubjectID != subjectID {
		return fmt.Errorf("test_case_delivery does not match the AI subject")
	}
	var version workflowgorm.WorkflowTemplateVersion
	if err := tx.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, workspaceID).First(&version).Error; err != nil {
		return fmt.Errorf("workflow version not found")
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	node, found := workflowNodeByID(definition, target.NodeID)
	if len(issues) > 0 || !found || !workflowNodeSupportsTestCases(node) {
		return fmt.Errorf("test case workflow node is invalid")
	}
	selected, err := workflowNodeIsSelected(tx, definition, instance, node.ID)
	if err != nil {
		return fmt.Errorf("workflow selected path is unavailable")
	}
	if !selected {
		return fmt.Errorf("test case workflow node is outside the selected path")
	}
	query := tx.Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspaceID, instance.ID, node.ID)
	active := instance.Status == "running" && containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)
	if active {
		query = query.Where("status = ?", "active")
	}
	var token workflowgorm.WorkflowToken
	lookup := query.Order("id DESC").First(&token)
	if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		return lookup.Error
	}
	if active && errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("workflow node is no longer active")
	}
	if token.ID == 0 {
		token = workflowgorm.WorkflowToken{WorkspaceID: workspaceID, InstanceID: instance.ID, NodeID: node.ID}
	}
	return nil
}
