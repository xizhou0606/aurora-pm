package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

var errWorkflowNextAssigneeRequired = errors.New("next node assignee is required")

type workflowNextAssigneeInput struct {
	NodeID         string `json:"node_id"`
	AssigneeUserID uint   `json:"assignee_user_id"`
}

func parseWorkflowNextNodeAssignees(items []workflowNextAssigneeInput, definition workflowDefinition) (map[string]uint, error) {
	result := map[string]uint{}
	for _, item := range items {
		nodeID := strings.TrimSpace(item.NodeID)
		if nodeID == "" || item.AssigneeUserID == 0 {
			return nil, errors.New("next node assignee is invalid")
		}
		node, found := workflowNodeByID(definition, nodeID)
		if !found || !workflowNodeTakesAssignee(node) {
			return nil, fmt.Errorf("next node %s cannot take an assignee", nodeID)
		}
		if _, exists := result[nodeID]; exists {
			return nil, errors.New("next node assignee is duplicated")
		}
		result[nodeID] = item.AssigneeUserID
	}
	return result, nil
}

func workflowAssigneeRequiredNodeIDs(definition workflowDefinition, next []string) []string {
	required := make([]string, 0, len(next))
	seen := map[string]bool{}
	queue := append([]string{}, next...)
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if seen[nodeID] {
			continue
		}
		seen[nodeID] = true
		node, found := workflowNodeByID(definition, nodeID)
		if !found {
			continue
		}
		switch node.Type {
		case "start", "end":
			continue
		case "parallel_split", "parallel_join":
			queue = append(queue, workflowNextNodes(definition, nodeID, "complete")...)
		default:
			if workflowNodeTakesAssignee(node) {
				required = append(required, nodeID)
			}
		}
	}
	return required
}

func mergeWorkflowNextAssignees(instance *workflowgorm.WorkflowInstance, assignees map[string]uint) error {
	if instance == nil || len(assignees) == 0 {
		return nil
	}
	plans, err := decodeWorkflowNodePlans(*instance)
	if err != nil {
		return err
	}
	if plans == nil {
		plans = map[string]workflowNodePlan{}
	}
	for nodeID, userID := range assignees {
		plan := plans[nodeID]
		value := userID
		plan.AssigneeUserID = &value
		plans[nodeID] = plan
	}
	encoded, err := json.Marshal(plans)
	if err != nil {
		return err
	}
	instance.NodePlansJSON = string(encoded)
	return nil
}

func (s *Server) assignWorkflowNextNodesTx(
	tx *gorm.DB,
	c *gin.Context,
	instance *workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	next []string,
	provided map[string]uint,
	defaultAssigneeID uint,
) error {
	required := workflowAssigneeRequiredNodeIDs(definition, next)
	missing := make([]string, 0, len(required))
	assigned := map[string]uint{}
	for _, nodeID := range required {
		if userID, exists := provided[nodeID]; exists && userID > 0 {
			assigned[nodeID] = userID
			continue
		}
		if _, exists, err := workflowNodePlanAssignee(*instance, nodeID); err != nil {
			return err
		} else if exists {
			continue
		}
		node, _ := workflowNodeByID(definition, nodeID)
		if instance.StartedByUserID > 0 && workflowNodeIsEntryAssignee(definition, node.ID) {
			continue
		}
		var tokenCount int64
		if err := tx.Model(&workflowgorm.WorkflowToken{}).Where(
			"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
			instance.WorkspaceID, instance.ID, nodeID, []string{"active", "waiting"},
		).Count(&tokenCount).Error; err != nil {
			return err
		}
		if tokenCount > 0 {
			continue
		}
		// 锁版语义（用户确认）：完成需求录入只结束当前节点，缺失的下一节点
		// 负责人默认就是操作者本人，不因未选人阻塞复合提交。
		if defaultAssigneeID > 0 {
			assigned[nodeID] = defaultAssigneeID
			continue
		}
		missing = append(missing, nodeID)
	}
	if len(missing) > 0 {
		return errWorkflowNextAssigneeRequired
	}
	return mergeWorkflowNextAssignees(instance, assigned)
}

func (s *Server) applyWorkflowAssigneeTokensTx(
	tx *gorm.DB,
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	assignees map[string]uint,
) error {
	if len(assignees) == 0 {
		return nil
	}
	for nodeID, userID := range assignees {
		if err := tx.Model(&workflowgorm.WorkflowToken{}).
			Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
				instance.WorkspaceID, instance.ID, nodeID, []string{"active", "waiting"}).
			Update("assignee_user_id", userID).Error; err != nil {
			return err
		}
		if err := s.recordWorkflowNodeAssignedNotification(tx, c, instance, nodeID, userID); err != nil {
			return err
		}
	}
	return nil
}
