package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func workflowLatestTokensByNode(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
) map[string]workflowgorm.WorkflowToken {
	var tokens []workflowgorm.WorkflowToken
	if tx.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).
		Order("id").Find(&tokens).Error != nil {
		return map[string]workflowgorm.WorkflowToken{}
	}
	result := make(map[string]workflowgorm.WorkflowToken, len(tokens))
	for _, token := range tokens {
		result[token.NodeID] = token
	}
	return result
}

func workflowNodeHasActionEdge(definition workflowDefinition, nodeID, action string) bool {
	for _, edge := range definition.Edges {
		if edge.Source == nodeID && workflowEdgeMatchesAction(edge.Kind, action) {
			return true
		}
	}
	return false
}

func workflowNodeAIStatus(execution gin.H) string {
	status, _ := execution["status"].(string)
	return strings.ToLower(strings.TrimSpace(status))
}

func workflowTaskPlanDraftCanRecheck(execution gin.H) bool {
	output, ok := execution["output"].(map[string]any)
	if !ok {
		return true
	}
	created, exists := output["createdTasks"]
	if !exists || created == nil {
		return true
	}
	refs, ok := created.([]any)
	return ok && len(refs) == 0
}

func (s *Server) workflowNodeAllowedActions(
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	node workflowNode,
	status string,
	execution gin.H,
	selected bool,
	isRollbackSource bool,
	hasRollbackTargets bool,
) []string {
	actions := make([]string, 0, 4)
	if status == "active" && instance.Status == "running" && workflowNodeNeedsAssignee(node) {
		switch {
		case node.Type == "approval":
			if workflowNodeHasActionEdge(definition, node.ID, "approve") {
				actions = append(actions, "approve")
			}
			if workflowNodeHasActionEdge(definition, node.ID, "reject") {
				actions = append(actions, "reject")
			}
		case node.Type == "condition":
			if workflowNodeHasActionEdge(definition, node.ID, "condition_yes") {
				actions = append(actions, "condition_yes")
			}
			if workflowNodeHasActionEdge(definition, node.ID, "condition_no") {
				actions = append(actions, "condition_no")
			}
		case strings.HasPrefix(node.Type, "ai_"):
			switch workflowNodeAIStatus(execution) {
			case "queued", "running":
			case "failed", "cancelled":
				actions = append(actions, "retry_ai")
			case "completed", "applied":
				actions = append(actions, "complete")
				if node.Capability != "requirement.task_plan" ||
					(workflowNodeAIStatus(execution) == "completed" && workflowTaskPlanDraftCanRecheck(execution)) {
					actions = append(actions, "recheck_ai")
				}
				if node.Capability == "requirement.completeness" {
					actions = append(actions, "override_ai")
				}
			default:
				if node.Capability == taskplan.Capability {
					ready := false
					if s.db != nil {
						ready, _ = requirementgorm.TaskPlanReadyTx(s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
					}
					if ready {
						actions = append(actions, "retry_ai")
					}
				} else {
					actions = append(actions, "complete")
				}
			}
		case node.Type == "timer":
			if workflowTimerReady(s.db, instance, node) {
				actions = append(actions, "complete")
			}
		default:
			actions = append(actions, "complete")
		}
	}
	if strings.HasPrefix(node.Type, "ai_") && node.Capability != "requirement.task_plan" && status != "active" && instance.Status == "running" && execution != nil &&
		workflowNodeAIStatus(execution) != "queued" && workflowNodeAIStatus(execution) != "running" &&
		s.workflowAINodeCanRecheck(instance, definition, node) {
		actions = append(actions, "recheck_ai")
	}
	if instance.SubjectType == "requirement" && selected && status != "skipped" &&
		workflowNodeRequiresDesignAsset(definition, node) {
		actions = append(actions, "supplement_design")
	}
	if isRollbackSource && hasRollbackTargets && (instance.Status == "running" || instance.Status == "completed") {
		actions = append(actions, "rollback")
	}
	return uniqueStrings(actions)
}

func (s *Server) workflowNodeActionFields(
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	node workflowNode,
	token workflowgorm.WorkflowToken,
	status string,
	execution gin.H,
	selected bool,
	isRollbackSource bool,
	hasRollbackTargets bool,
) gin.H {
	var assignee *uint
	var err error
	if token.ID > 0 {
		assignee, err = workflowTokenEffectiveAssigneeTx(s.db, instance, definition, token)
	} else {
		assignee, err = workflowNodeDisplayedAssignee(instance, definition, node)
	}
	if err != nil {
		assignee = nil
	}
	actions := s.workflowNodeAllowedActions(
		instance, definition, node, status, execution, selected,
		isRollbackSource, hasRollbackTargets,
	)
	return gin.H{
		"assignee_user_id": assignee,
		"can_act":          len(actions) > 0,
		"allowed_actions":  actions,
	}
}

func workflowNodeDisplayedAssignee(instance workflowgorm.WorkflowInstance, definition workflowDefinition, node workflowNode) (*uint, error) {
	if !workflowNodeTakesAssignee(node) {
		return nil, nil
	}
	if overridden, exists, err := workflowNodePlanAssignee(instance, node.ID); err != nil {
		return nil, err
	} else if exists {
		return overridden, nil
	}
	if instance.SubjectType == "requirement" && instance.StartedByUserID > 0 &&
		firstAssignableWorkflowNodeID(definition) == node.ID {
		return workflowUintPtr(instance.StartedByUserID), nil
	}
	return nil, nil
}
