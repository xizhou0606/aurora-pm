package httpapi

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"gorm.io/gorm"
)

func (s *Server) recordMutation(
	tx *gorm.DB,
	c *gin.Context,
	projectID uint,
	resource string,
	resourceID uint,
	action string,
	target string,
	payload any,
) error {
	return s.recordMutationWithDetail(
		tx, c, projectID, resource, resourceID, action, target, "", payload,
	)
}

func (s *Server) recordMutationWithDetail(
	tx *gorm.DB,
	c *gin.Context,
	projectID uint,
	resource string,
	resourceID uint,
	action string,
	target string,
	detail string,
	payload any,
) error {
	workspace, _ := currentWorkspace(c)
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := s.recordAuditWithDetail(
		tx, c, projectID, resource, resourceID, action, target, detail,
	); err != nil {
		return err
	}
	event := auditgorm.OutboxEvent{
		WorkspaceID:   workspace.ID,
		AggregateType: resource,
		AggregateID:   resourceID,
		EventType:     "pm." + resource + "." + action,
		Payload:       string(raw),
		OccurredAt:    time.Now().UTC(),
	}
	return tx.Create(&event).Error
}

func (s *Server) recordAuditWithDetail(
	tx *gorm.DB,
	c *gin.Context,
	projectID uint,
	resource string,
	resourceID uint,
	action string,
	target string,
	detail string,
) error {
	a, _ := currentActor(c)
	workspace, _ := currentWorkspace(c)
	auditResource, auditAction := legacyAuditKeys(resource, action)
	audit := auditgorm.AuditLog{
		WorkspaceID: workspace.ID,
		UserID:      factUserID(a.User),
		Username:    a.User.Username,
		Role:        a.User.Role,
		Method:      c.Request.Method,
		StatusCode:  200,
		Resource:    auditResource,
		ResourceID:  resourceID,
		ProjectID:   projectID,
		Action:      auditAction,
		Target:      target,
		Detail:      detail,
	}
	return tx.Create(&audit).Error
}

func legacyAuditKeys(resource, action string) (string, string) {
	resources := map[string]string{
		"project_set": "project", "plan_unit": "version", "work_task": "task",
		"project_permission": "permission", "delivery_label": "version",
	}
	actions := map[string]string{
		"created": "create", "updated": "update", "deleted": "delete",
		"upserted": "update", "moved": "move", "owner_transferred": "transfer_owner",
	}
	if value := resources[resource]; value != "" {
		resource = value
	}
	if value := actions[action]; value != "" {
		action = value
	}
	return resource, action
}
