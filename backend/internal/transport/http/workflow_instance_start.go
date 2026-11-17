package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

var (
	errWorkflowAlreadyFixed      = errors.New("workflow template is already fixed for subject")
	errPublishedWorkflowNotFound = errors.New("published workflow not found")
	errPublishedWorkflowInvalid  = errors.New("published workflow is invalid")
)

func (s *Server) requirePublishedWorkflow(
	c *gin.Context,
	workspaceID uint,
	templateID *uint,
	subjectType string,
) (workflowgorm.WorkflowTemplate, workflowgorm.WorkflowTemplateVersion, workflowDefinition, bool) {
	if templateID == nil || *templateID == 0 {
		bad(c, "workflow_template_id required")
		return workflowgorm.WorkflowTemplate{}, workflowgorm.WorkflowTemplateVersion{}, workflowDefinition{}, false
	}
	template, version, definition, err := s.loadPublishedWorkflow(workspaceID, *templateID, subjectType)
	if err != nil {
		fail(c, http.StatusConflict, "published workflow template not found")
		return workflowgorm.WorkflowTemplate{}, workflowgorm.WorkflowTemplateVersion{}, workflowDefinition{}, false
	}
	return template, version, definition, true
}

func (s *Server) loadPublishedWorkflow(workspaceID, templateID uint, subjectType string) (workflowgorm.WorkflowTemplate, workflowgorm.WorkflowTemplateVersion, workflowDefinition, error) {
	var template workflowgorm.WorkflowTemplate
	if err := s.db.Where("id = ? AND workspace_id = ? AND subject_type = ?", templateID, workspaceID, subjectType).First(&template).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return template, workflowgorm.WorkflowTemplateVersion{}, workflowDefinition{}, fmt.Errorf("%w: template", errPublishedWorkflowNotFound)
		}
		return template, workflowgorm.WorkflowTemplateVersion{}, workflowDefinition{}, fmt.Errorf("load published workflow template: %w", err)
	}
	if template.PublishedVersion == 0 {
		return template, workflowgorm.WorkflowTemplateVersion{}, workflowDefinition{}, fmt.Errorf("%w: template has no published version", errPublishedWorkflowNotFound)
	}
	var version workflowgorm.WorkflowTemplateVersion
	if err := s.db.Where("workspace_id = ? AND template_id = ? AND version = ?", workspaceID, template.ID, template.PublishedVersion).First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return template, version, workflowDefinition{}, fmt.Errorf("%w: version", errPublishedWorkflowNotFound)
		}
		return template, version, workflowDefinition{}, fmt.Errorf("load published workflow version: %w", err)
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	if len(issues) > 0 {
		return template, version, definition, errPublishedWorkflowInvalid
	}
	return template, version, definition, nil
}

func (s *Server) createWorkflowInstanceTx(tx *gorm.DB, c *gin.Context, workspaceID uint, template workflowgorm.WorkflowTemplate, version workflowgorm.WorkflowTemplateVersion, definition workflowDefinition, subjectType string, subjectID uint) (workflowgorm.WorkflowInstance, error) {
	var existing int64
	if err := tx.Model(&workflowgorm.WorkflowInstance{}).Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, subjectType, subjectID).Count(&existing).Error; err != nil {
		return workflowgorm.WorkflowInstance{}, err
	}
	if existing > 0 {
		return workflowgorm.WorkflowInstance{}, errWorkflowAlreadyFixed
	}
	startNode := ""
	for _, node := range definition.Nodes {
		if node.Type == "start" {
			startNode = node.ID
		}
	}
	now := time.Now().UTC()
	instance := workflowgorm.WorkflowInstance{
		WorkspaceID: workspaceID, TemplateID: template.ID, TemplateVersionID: version.ID,
		SubjectType: subjectType, SubjectID: subjectID, Status: "running",
		CurrentNodeIDs: "[]", NodePlansJSON: "{}", StartedByUserID: currentFactUserID(c), StartedAt: now,
	}
	if err := tx.Create(&instance).Error; err != nil {
		return instance, err
	}
	startRun := workflowgorm.WorkflowNodeRun{WorkspaceID: workspaceID, InstanceID: instance.ID, NodeID: startNode, NodeType: "start", Status: "completed", StartedAt: now, CompletedAt: &now}
	if err := tx.Create(&startRun).Error; err != nil {
		return instance, err
	}
	current := workflowNextNodes(definition, startNode, "advance")
	var err error
	current, err = s.advanceAutomaticWorkflowNodesTx(tx, &instance, definition, current, now)
	if err != nil {
		return instance, err
	}
	encoded, _ := json.Marshal(current)
	if err := tx.Model(&instance).Updates(map[string]any{"current_node_ids": string(encoded), "status": instance.Status, "completed_at": instance.CompletedAt}).Error; err != nil {
		return instance, err
	}
	for _, nodeID := range current {
		if err := createWorkflowTokenTx(tx, instance, definition, nodeID, "active", false); err != nil {
			return instance, err
		}
		node, found := workflowNodeByID(definition, nodeID)
		if !found {
			return instance, fmt.Errorf("workflow node %q is unavailable", nodeID)
		}
		assignee, err := workflowNodeEffectiveAssigneeTx(tx, instance, definition, node, false)
		if err != nil {
			return instance, err
		}
		if assignee != nil {
			if err := s.recordWorkflowNodeAssignedNotification(tx, c, instance, nodeID, *assignee); err != nil {
				return instance, err
			}
		}
	}
	if err := syncWorkflowSubjectStatus(tx, instance, definition, current); err != nil {
		return instance, err
	}
	if err := s.queueEnteredWorkflowAINodesTx(tx, c, instance, definition, current, now); err != nil {
		return instance, err
	}
	if subjectType == "requirement" {
		if err := tx.Model(&requirementgorm.Requirement{}).Where("id = ? AND workspace_id = ?", subjectID, workspaceID).Update("workflow_instance_id", instance.ID).Error; err != nil {
			return instance, err
		}
	}
	projectID := s.workflowSubjectProjectID(tx, workspaceID, subjectType, subjectID)
	err = s.recordMutation(tx, c, projectID, "workflow_instance", instance.ID, "started", template.Name, gin.H{"template_id": template.ID, "template_version": template.PublishedVersion, "subject_type": subjectType, "subject_id": subjectID})
	return instance, err
}
