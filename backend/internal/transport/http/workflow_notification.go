package httpapi

import (
	"encoding/json"
	"fmt"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func (s *Server) recordWorkflowNodeAssignedNotification(
	tx *gorm.DB,
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	nodeID string,
	assigneeID uint,
) error {
	if assigneeID == 0 {
		return nil
	}
	projectID := s.workflowSubjectProjectID(tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
	var project projectgorm.ProjectSet
	if err := tx.Select("id", "team_id").Where("id = ? AND workspace_id = ?", projectID, instance.WorkspaceID).First(&project).Error; err != nil {
		return fmt.Errorf("workflow notification project is unavailable: %w", err)
	}
	if project.TeamID == 0 || project.ID == 0 {
		return fmt.Errorf("workflow notification project is unavailable")
	}
	actorID := currentFactUserID(c)
	nodeName := nodeID
	var version workflowgorm.WorkflowTemplateVersion
	if tx.Select("definition").Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error == nil {
		var definition workflowDefinition
		if json.Unmarshal([]byte(version.Definition), &definition) == nil {
			if node, found := workflowNodeByID(definition, nodeID); found {
				nodeName = node.Name
			}
		}
	}
	subjectTitle := workflowNotificationSubjectTitle(tx, instance)
	vars := map[string]string{
		"operator": workflowNotificationUserLabel(tx, actorID),
		"node":     strings.TrimSpace(nodeName),
		"nodeId":   nodeID,
		"assignee": workflowNotificationUserLabel(tx, assigneeID),
		"title":    subjectTitle,
		"url":      s.workflowNotificationURL(instance),
	}
	if vars["node"] == "" {
		vars["node"] = nodeID
	}
	return notification.Enqueue(tx, "workflow_node_notification", instance.ID, notification.Intent{
		SkipExternal: !s.cfg.Notification.Enabled, ResourceType: instance.SubjectType, ResourceID: notification.WorkflowResourceID(instance.SubjectType, project.ID, instance.SubjectID),
		WorkspaceID: instance.WorkspaceID, Scene: notification.SceneWorkflowNodeAssigned,
		TeamID: project.TeamID, ProjectID: project.ID, ActorUserID: actorID,
		RecipientUserIDs: []uint{assigneeID}, Vars: vars,
		FallbackPlainText: workflowNodeAssignedNotificationPlainText(vars),
	})
}

func workflowNotificationUserLabel(tx *gorm.DB, userID uint) string {
	if userID == 0 {
		return "-"
	}
	name, err := identitygorm.NewAccessGuideDirectory(tx).GuideOwnerName(tx.Statement.Context, userID)
	if err == nil && strings.TrimSpace(name) != "" {
		return name
	}
	return retiredUserDisplayName(userID)
}

func workflowNotificationSubjectTitle(tx *gorm.DB, instance workflowgorm.WorkflowInstance) string {
	if instance.SubjectType == "requirement" {
		var title string
		if tx.Raw("SELECT title FROM requirements WHERE id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).Scan(&title).Error == nil && strings.TrimSpace(title) != "" {
			return strings.TrimSpace(title)
		}
	}
	if instance.SubjectType == "plan_unit" {
		var unit projectgorm.PlanUnit
		if tx.Select("title").Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&unit).Error == nil {
			return strings.TrimSpace(unit.Title)
		}
	}
	return fmt.Sprintf("流程 #%d", instance.ID)
}

func (s *Server) workflowNotificationURL(instance workflowgorm.WorkflowInstance) string {
	if instance.SubjectType == "requirement" {
		return notification.RequirementURL(s.cfg.Notification.PublicBaseURL, instance.SubjectID)
	}
	return notification.WorkflowListURL(s.cfg.Notification.PublicBaseURL)
}

func workflowNodeAssignedNotificationPlainText(vars map[string]string) string {
	return notification.PlainText(notification.SceneWorkflowNodeAssigned, vars)
}

func (s *Server) recordWorkflowPreReviewInvitedNotification(
	tx *gorm.DB,
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	node workflowNode,
	inviteeIDs []uint,
) error {
	actorID := currentFactUserID(c)
	recipients := make([]uint, 0, len(inviteeIDs))
	seen := map[uint]struct{}{}
	for _, userID := range inviteeIDs {
		if userID == 0 || userID == actorID {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		recipients = append(recipients, userID)
	}
	if len(recipients) == 0 {
		return nil
	}
	projectID := s.workflowSubjectProjectID(tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
	var project projectgorm.ProjectSet
	if err := tx.Select("id", "team_id").Where("id = ? AND workspace_id = ?", projectID, instance.WorkspaceID).First(&project).Error; err != nil {
		return fmt.Errorf("workflow notification project is unavailable: %w", err)
	}
	if project.TeamID == 0 || project.ID == 0 {
		return fmt.Errorf("workflow notification project is unavailable")
	}
	nodeName := strings.TrimSpace(node.Name)
	if nodeName == "" {
		nodeName = node.ID
	}
	vars := map[string]string{
		"operator": workflowNotificationUserLabel(tx, actorID),
		"node":     nodeName,
		"nodeId":   node.ID,
		"title":    workflowNotificationSubjectTitle(tx, instance),
		"url":      s.workflowNotificationURL(instance),
	}
	return notification.Enqueue(tx, "workflow_node_notification", instance.ID, notification.Intent{
		SkipExternal: !s.cfg.Notification.Enabled, ResourceType: instance.SubjectType, ResourceID: notification.WorkflowResourceID(instance.SubjectType, project.ID, instance.SubjectID),
		WorkspaceID: instance.WorkspaceID, Scene: notification.SceneWorkflowPreReviewInvited,
		TeamID: project.TeamID, ProjectID: project.ID, ActorUserID: actorID,
		RecipientUserIDs: recipients, Vars: vars,
		FallbackPlainText: workflowPreReviewInvitedNotificationPlainText(vars),
	})
}

func workflowPreReviewInvitedNotificationPlainText(vars map[string]string) string {
	return notification.PlainText(notification.SceneWorkflowPreReviewInvited, vars)
}
