package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

var errWorkflowNodeAssigneeUnavailable = errors.New("workflow node assignee is unavailable")

func workflowUintPtr(value uint) *uint { return &value }

func workflowNodeNeedsAssignee(node workflowNode) bool {
	switch node.Type {
	case "start", "end", "parallel_split", "parallel_join":
		return false
	default:
		return true
	}
}

// workflowNodeTakesAssignee reports whether a human owner may be assigned to
// the node. AI nodes execute automatically, so requirement and plan-unit
// workflows must not set a node assignee on them.
func workflowNodeTakesAssignee(node workflowNode) bool {
	return workflowNodeNeedsAssignee(node) && !strings.HasPrefix(node.Type, "ai_")
}

func workflowAssigneeUserID(value any) (uint, bool) {
	switch typed := value.(type) {
	case uint:
		return typed, typed > 0
	case uint8:
		return uint(typed), typed > 0
	case uint16:
		return uint(typed), typed > 0
	case uint32:
		return uint(typed), typed > 0
	case uint64:
		value := uint(typed)
		return value, typed > 0 && uint64(value) == typed
	case int:
		return uint(typed), typed > 0
	case int8:
		return uint(typed), typed > 0
	case int16:
		return uint(typed), typed > 0
	case int32:
		return uint(typed), typed > 0
	case int64:
		value := uint(typed)
		return value, typed > 0 && int64(value) == typed
	case float32:
		value := float64(typed)
		if value <= 0 || math.Trunc(value) != value {
			return 0, false
		}
		result := uint(value)
		return result, result > 0 && float64(result) == value
	case float64:
		if typed <= 0 || math.Trunc(typed) != typed {
			return 0, false
		}
		result := uint(typed)
		return result, result > 0 && float64(result) == typed
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed <= 0 {
			return 0, false
		}
		result := uint(parsed)
		return result, int64(result) == parsed
	default:
		return 0, false
	}
}

var workflowTemplateIdentityKeys = []string{
	"assignee_user_id", "assignee_name", "project_id", "project_set_id", "requirement_id", "subject_id",
}

// stripWorkflowTemplateAssignees keeps people and subject records out of reusable
// definitions. Published versions created before this rule may still store
// assignee_user_id; runtime resolution must ignore that field and use node_plans
// plus the first-node creator.
func stripWorkflowTemplateAssignees(definition *workflowDefinition) {
	if definition == nil {
		return
	}
	for index := range definition.Nodes {
		node := &definition.Nodes[index]
		if node.Config == nil {
			continue
		}
		for _, key := range workflowTemplateIdentityKeys {
			delete(node.Config, key)
		}
		if len(node.Config) == 0 {
			node.Config = nil
		}
	}
}

func workflowNodeEffectiveAssigneeTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	node workflowNode,
	allowHistorical bool,
) (*uint, error) {
	if !workflowNodeTakesAssignee(node) {
		return nil, nil
	}
	if overridden, exists, err := workflowNodePlanAssignee(instance, node.ID); err != nil {
		return nil, err
	} else if exists {
		return overridden, nil
	}
	if instance.StartedByUserID > 0 && workflowNodeIsEntryAssignee(definition, node.ID) {
		return workflowUintPtr(instance.StartedByUserID), nil
	}
	if allowHistorical {
		if last, err := workflowNodeLastAssigneeTx(tx, instance, node.ID); err != nil {
			return nil, err
		} else if last != nil {
			return last, nil
		}
	}
	return nil, errWorkflowNodeAssigneeUnavailable
}

func workflowNodeLastAssigneeTx(tx *gorm.DB, instance workflowgorm.WorkflowInstance, nodeID string) (*uint, error) {
	var token workflowgorm.WorkflowToken
	err := tx.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND assignee_user_id IS NOT NULL AND assignee_user_id > 0",
		instance.WorkspaceID, instance.ID, nodeID,
	).Order("id DESC").First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if token.AssigneeUserID == nil || *token.AssigneeUserID == 0 {
		return nil, nil
	}
	return workflowUintPtr(*token.AssigneeUserID), nil
}

func workflowTokenEffectiveAssigneeTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	token workflowgorm.WorkflowToken,
) (*uint, error) {
	if _, err := decodeWorkflowNodePlans(instance); err != nil {
		return nil, err
	}
	node, found := workflowNodeByID(definition, token.NodeID)
	if !found {
		return nil, fmt.Errorf("workflow node %q is unavailable", token.NodeID)
	}
	if !workflowNodeTakesAssignee(node) {
		return nil, nil
	}
	entryLive := instance.StartedByUserID > 0 &&
		workflowNodeIsEntryAssignee(definition, token.NodeID) &&
		(token.ID == 0 || token.Status == "active" || token.Status == "waiting")
	if entryLive {
		if overridden, exists, err := workflowNodePlanAssignee(instance, node.ID); err != nil {
			return nil, err
		} else if exists {
			return overridden, nil
		}
		return workflowUintPtr(instance.StartedByUserID), nil
	}
	if token.AssigneeUserID != nil && *token.AssigneeUserID > 0 {
		return workflowUintPtr(*token.AssigneeUserID), nil
	}
	if overridden, exists, err := workflowNodePlanAssignee(instance, node.ID); err != nil {
		return nil, err
	} else if exists {
		return overridden, nil
	}
	if instance.StartedByUserID > 0 && workflowNodeIsEntryAssignee(definition, node.ID) {
		return workflowUintPtr(instance.StartedByUserID), nil
	}
	return workflowNodeEffectiveAssigneeTx(tx, instance, definition, node, true)
}

// effectiveWorkflowTokenAssignee is shared with projections such as My Todo.
// It keeps NULL assignees created before schema 0016 readable without requiring
// an eager data migration of immutable workflow history.
func (s *Server) effectiveWorkflowTokenAssignee(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	token workflowgorm.WorkflowToken,
) *uint {
	var version workflowgorm.WorkflowTemplateVersion
	if err := tx.Select("definition").
		Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).
		First(&version).Error; err != nil {
		return nil
	}
	var definition workflowDefinition
	if json.Unmarshal([]byte(version.Definition), &definition) != nil {
		return nil
	}
	stripWorkflowTemplateAssignees(&definition)
	assignee, err := workflowTokenEffectiveAssigneeTx(tx, instance, definition, token)
	if err != nil {
		return nil
	}
	return assignee
}

func createWorkflowTokenTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	nodeID string,
	status string,
	allowHistorical bool,
) error {
	node, found := workflowNodeByID(definition, nodeID)
	if !found {
		return fmt.Errorf("workflow node %q is unavailable", nodeID)
	}
	assignee, err := workflowNodeEffectiveAssigneeTx(tx, instance, definition, node, allowHistorical)
	if err != nil {
		return err
	}
	return tx.Create(&workflowgorm.WorkflowToken{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID,
		NodeID: nodeID, Status: status, AssigneeUserID: assignee,
	}).Error
}

// requireWorkflowNodeActive is a workflow-state precondition, not an access
// decision. Entry RBAC decides which principal may call the endpoint.
func (s *Server) requireWorkflowNodeActive(
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	nodeID string,
	activeOnly bool,
) bool {
	if !activeOnly {
		return true
	}
	query := s.db.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ?",
		instance.WorkspaceID, instance.ID, nodeID,
	)
	query = query.Where("status = ?", "active")
	var token workflowgorm.WorkflowToken
	lookup := query.Order("id DESC").First(&token)
	if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		dbFail(c)
		return false
	}
	if errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		fail(c, http.StatusConflict, "workflow node is no longer active")
		return false
	}
	return true
}
