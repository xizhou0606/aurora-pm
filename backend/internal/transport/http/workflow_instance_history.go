package httpapi

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

type workflowRunActor struct {
	UserID                 uint   `json:"user_id"`
	Comment                string `json:"comment,omitempty"`
	ManualOverride         bool   `json:"manual_override,omitempty"`
	Decision               *bool  `json:"decision,omitempty"`
	DesignURL              string `json:"design_url,omitempty"`
	RollbackTargetNodeID   string `json:"rollback_target_node_id,omitempty"`
	TransitionTargetNodeID string `json:"transition_target_node_id,omitempty"`
}

func encodeWorkflowRunActor(userID uint) string {
	raw, _ := json.Marshal(workflowRunActor{UserID: userID})
	return string(raw)
}

func encodeWorkflowRunComment(userID uint, comment, targetNodeID string) string {
	raw, _ := json.Marshal(workflowRunActor{
		UserID: userID, Comment: strings.TrimSpace(comment),
		TransitionTargetNodeID: strings.TrimSpace(targetNodeID),
	})
	return string(raw)
}

func encodeWorkflowRunTransition(userID uint, comment, targetNodeID string) string {
	return encodeWorkflowRunComment(userID, comment, targetNodeID)
}

func encodeWorkflowRunOverride(userID uint, comment, targetNodeID string) string {
	raw, _ := json.Marshal(workflowRunActor{
		UserID: userID, Comment: strings.TrimSpace(comment), ManualOverride: true,
		TransitionTargetNodeID: strings.TrimSpace(targetNodeID),
	})
	return string(raw)
}

func encodeWorkflowRunDecision(userID uint, comment string, decision bool, designURL, targetNodeID string) string {
	raw, _ := json.Marshal(workflowRunActor{
		UserID: userID, Comment: strings.TrimSpace(comment), Decision: &decision, DesignURL: strings.TrimSpace(designURL),
		TransitionTargetNodeID: strings.TrimSpace(targetNodeID),
	})
	return string(raw)
}

func encodeWorkflowRunMaterial(userID uint, comment, designURL string) string {
	encoded, _ := json.Marshal(workflowRunActor{UserID: userID, Comment: strings.TrimSpace(comment), DesignURL: designURL})
	return string(encoded)
}

func encodeWorkflowRunMaterialTransition(userID uint, comment, designURL, targetNodeID string) string {
	encoded, _ := json.Marshal(workflowRunActor{
		UserID: userID, Comment: strings.TrimSpace(comment), DesignURL: strings.TrimSpace(designURL),
		TransitionTargetNodeID: strings.TrimSpace(targetNodeID),
	})
	return string(encoded)
}

func encodeWorkflowRunRollback(userID uint, comment, targetNodeID string) string {
	encoded, _ := json.Marshal(workflowRunActor{
		UserID: userID, Comment: strings.TrimSpace(comment),
		RollbackTargetNodeID: strings.TrimSpace(targetNodeID),
	})
	return string(encoded)
}

func decodeWorkflowRunActor(raw string) workflowRunActor {
	var actor workflowRunActor
	if json.Unmarshal([]byte(raw), &actor) != nil {
		return workflowRunActor{}
	}
	return actor
}

func (s *Server) workflowTransitionHistory(instance workflowgorm.WorkflowInstance, runs []workflowgorm.WorkflowNodeRun, approvals []workflowgorm.WorkflowApproval, definition workflowDefinition) []gin.H {
	nodes := make(map[string]workflowNode, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodes[node.ID] = node
	}
	approvalByRun := make(map[uint]workflowgorm.WorkflowApproval, len(approvals))
	for _, approval := range approvals {
		approvalByRun[approval.NodeRunID] = approval
	}
	refs := workspaceContextReferences{}.withUserID(instance.StartedByUserID)
	for _, approval := range approvals {
		refs.UserIDs = appendOptionalID(refs.UserIDs, approval.ApproverUserID)
	}
	for _, run := range runs {
		refs = refs.withUserID(decodeWorkflowRunActor(run.OutputJSON).UserID)
	}
	workspace := s.loadWorkspaceContext(instance.WorkspaceID, refs)
	history := make([]gin.H, 0, len(runs))
	for _, run := range runs {
		node := nodes[run.NodeID]
		occurredAt := run.StartedAt
		if run.CompletedAt != nil {
			occurredAt = *run.CompletedAt
		}
		operatorType, operatorName := "system", "系统"
		operatorID := uint(0)
		comment := ""
		actor := decodeWorkflowRunActor(run.OutputJSON)
		action := workflowHistoryAction(run, actor)
		if run.NodeType == "start" {
			operatorType, operatorID = "user", instance.StartedByUserID
		} else if approval, ok := approvalByRun[run.ID]; ok {
			operatorType, comment = "user", approval.Comment
			if approval.ApproverUserID != nil {
				operatorID = *approval.ApproverUserID
			}
			if approval.DecidedAt != nil {
				occurredAt = *approval.DecidedAt
			}
		} else if (actor.ManualOverride || actor.Decision != nil || actor.RollbackTargetNodeID != "") && actor.UserID > 0 {
			operatorType, operatorID, comment = "user", actor.UserID, actor.Comment
		} else if strings.HasPrefix(run.NodeType, "ai_") {
			operatorType, operatorName = "ai", "AI 助手"
		} else if actor.UserID > 0 {
			operatorType, operatorID, comment = "user", actor.UserID, actor.Comment
		}
		if operatorID > 0 {
			operatorName = userDisplayName(workspace, &operatorID)
		}
		name := strings.TrimSpace(node.Name)
		if name == "" {
			name = run.NodeID
		}
		item := gin.H{
			"id": run.ID, "node_id": run.NodeID, "node_name": name, "node_type": run.NodeType,
			"action": action, "status": run.Status, "operator_type": operatorType,
			"operator_user_id": operatorID, "operator_name": operatorName, "comment": comment,
			"error_message": run.ErrorMessage, "occurred_at": occurredAt,
		}
		if actor.RollbackTargetNodeID != "" {
			item["target_node_id"] = actor.RollbackTargetNodeID
			if target, found := nodes[actor.RollbackTargetNodeID]; found {
				item["target_node_name"] = target.Name
			}
		} else if actor.TransitionTargetNodeID != "" {
			item["target_node_id"] = actor.TransitionTargetNodeID
			if target, found := nodes[actor.TransitionTargetNodeID]; found {
				item["target_node_name"] = target.Name
			}
		}
		if actor.Decision != nil {
			item["decision"] = *actor.Decision
		}
		if strings.TrimSpace(actor.DesignURL) != "" {
			item["materials"] = []gin.H{{"kind": "design_image", "url": strings.TrimSpace(actor.DesignURL)}}
		}
		history = append(history, item)
	}
	return history
}

func workflowHistoryAction(run workflowgorm.WorkflowNodeRun, actor workflowRunActor) string {
	switch {
	case run.NodeType == "start":
		return "started"
	case actor.RollbackTargetNodeID != "":
		return "rolled_back"
	case run.Status == "supplemented":
		return "material_supplemented"
	case run.Status == "rejected":
		return "rejected"
	case run.Status == "failed":
		return "failed"
	case run.NodeType == "approval":
		return "approved"
	case strings.HasPrefix(run.NodeType, "ai_") && actor.ManualOverride:
		return "ai_overridden"
	case strings.HasPrefix(run.NodeType, "ai_"):
		if run.Status == "waiting" {
			return "ai_started"
		}
		return "ai_completed"
	case run.NodeType == "condition" && actor.Decision != nil && *actor.Decision:
		return "condition_yes"
	case run.NodeType == "condition" && actor.Decision != nil:
		return "condition_no"
	default:
		return "completed"
	}
}
