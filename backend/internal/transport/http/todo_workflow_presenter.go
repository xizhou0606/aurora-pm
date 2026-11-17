package httpapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func workflowTodoAIKey(instanceID uint, nodeID string) string {
	return fmt.Sprintf("%d:%s", instanceID, nodeID)
}

func (s *Server) loadWorkflowTodoAIStates(
	workspaceID uint,
	instanceIDs []uint,
) (map[string]aigorm.AIRun, error) {
	result := map[string]aigorm.AIRun{}
	if len(instanceIDs) == 0 {
		return result, nil
	}
	var jobs []workflowgorm.WorkflowJob
	if err := s.db.Where(
		"workspace_id = ? AND instance_id IN ? AND kind = ?",
		workspaceID, instanceIDs, "ai_run",
	).Order("id").Find(&jobs).Error; err != nil {
		return nil, err
	}
	latestRunIDs := map[string]uint{}
	runIDs := make([]uint, 0, len(jobs))
	for _, job := range jobs {
		var payload workflowAIJobPayload
		if json.Unmarshal([]byte(job.PayloadJSON), &payload) != nil ||
			payload.AIRunID == 0 || strings.TrimSpace(payload.NodeID) == "" {
			continue
		}
		key := workflowTodoAIKey(job.InstanceID, strings.TrimSpace(payload.NodeID))
		latestRunIDs[key] = payload.AIRunID
		runIDs = append(runIDs, payload.AIRunID)
	}
	if len(runIDs) == 0 {
		return result, nil
	}
	var runs []aigorm.AIRun
	if err := s.db.Where("workspace_id = ? AND id IN ?", workspaceID, runIDs).
		Find(&runs).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]aigorm.AIRun, len(runs))
	for _, run := range runs {
		byID[run.ID] = run
	}
	for key, runID := range latestRunIDs {
		if run, exists := byID[runID]; exists {
			result[key] = run
		}
	}
	return result, nil
}

func (s *Server) workflowTodoActions(
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	node workflowNode,
	aiState aigorm.AIRun,
) (string, []string, bool) {
	var execution gin.H
	if strings.HasPrefix(node.Type, "ai_") {
		status := strings.ToLower(strings.TrimSpace(aiState.Status))
		if aiState.ID == 0 || status == "queued" || status == "running" {
			return "", nil, false
		}
		execution = gin.H{"status": status}
	}
	actions := s.workflowNodeAllowedActions(
		instance, definition, node, "active", execution, true, false, false,
	)
	if len(actions) == 0 {
		return "", nil, false
	}
	if strings.HasPrefix(node.Type, "ai_") {
		switch strings.ToLower(strings.TrimSpace(aiState.Status)) {
		case "failed", "cancelled":
			return "ai_retry", actions, true
		case "completed", "applied":
			return "ai_confirm", actions, true
		default:
			return "", nil, false
		}
	}
	switch node.Type {
	case "approval":
		return "approval", actions, true
	case "condition":
		return "condition", actions, true
	default:
		return "complete", actions, true
	}
}

func workflowTodoDTO(item workflowTodoItem) gin.H {
	revision := fmt.Sprint(item.Token.ID)
	if item.AIRunID > 0 {
		revision = fmt.Sprintf("%d:%d:%s", item.Token.ID, item.AIRunID, item.AIStatus)
	}
	row := gin.H{
		"id":          item.Token.ID,
		"todo_id":     fmt.Sprintf("workflow:%d:%s:%d", item.Instance.ID, item.Node.ID, item.Token.ID),
		"source_type": "workflow_node", "source_revision": revision,
		"title": item.Node.Name, "status": "active", "action_kind": item.ActionKind,
		"allowed_actions": item.AllowedActions, "detail_path": workflowTodoDetailPath(item),
		"workflow_instance_id": item.Instance.ID, "workflow_token_id": item.Token.ID,
		"workflow_node_id": item.Node.ID, "node_type": item.Node.Type,
		"subject_type": item.Instance.SubjectType, "subject_id": item.Instance.SubjectID,
		"subject_title": item.Subject.Title, "assignee_user_id": item.AssigneeUserID,
		"project_set_id":   item.Subject.ProjectSetID,
		"project_set_name": item.Subject.ProjectSetName,
		"template_name":    item.TemplateName, "due_at": item.DueAt,
		"created_at": item.Token.CreatedAt, "updated_at": item.Token.UpdatedAt,
	}
	if item.Subject.ProjectID > 0 {
		row["project_id"], row["project_name"] = item.Subject.ProjectID, item.Subject.ProjectName
	}
	if item.Subject.RequirementID > 0 {
		row["requirement_id"] = item.Subject.RequirementID
		row["requirement_title"] = item.Subject.RequirementName
	}
	if item.AIStatus != "" {
		row["ai_status"], row["ai_run_id"] = item.AIStatus, item.AIRunID
	}
	return row
}

func workflowTodoDetailPath(item workflowTodoItem) string {
	query := url.Values{"workflow_node": []string{item.Node.ID}}.Encode()
	if item.Instance.SubjectType == "requirement" {
		return fmt.Sprintf("/workspace/requirements/%d?%s", item.Instance.SubjectID, query)
	}
	return fmt.Sprintf(
		"/workspace/projects/%d/items/%d?%s",
		item.Subject.ProjectSetID, item.Instance.SubjectID, query,
	)
}

func sortWorkflowTodoItems(items []workflowTodoItem, now time.Time) {
	sort.SliceStable(items, func(left, right int) bool {
		a, b := items[left], items[right]
		aOverdue := a.DueAt != nil && a.DueAt.Before(now)
		bOverdue := b.DueAt != nil && b.DueAt.Before(now)
		if aOverdue != bOverdue {
			return aOverdue
		}
		priority := map[string]int{
			"ai_retry": 0, "approval": 1, "ai_confirm": 2, "condition": 3, "complete": 4,
		}
		if priority[a.ActionKind] != priority[b.ActionKind] {
			return priority[a.ActionKind] < priority[b.ActionKind]
		}
		if (a.DueAt == nil) != (b.DueAt == nil) {
			return a.DueAt != nil
		}
		if a.DueAt != nil && !a.DueAt.Equal(*b.DueAt) {
			return a.DueAt.Before(*b.DueAt)
		}
		if !a.Token.UpdatedAt.Equal(b.Token.UpdatedAt) {
			return a.Token.UpdatedAt.After(b.Token.UpdatedAt)
		}
		return a.Token.ID > b.Token.ID
	})
}
