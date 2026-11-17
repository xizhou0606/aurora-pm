package workflowgorm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type todoDefinition struct {
	Nodes []todoNode `json:"nodes"`
	Edges []todoEdge `json:"edges"`
}

type todoNode struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Capability string         `json:"capability,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

type todoEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind,omitempty"`
}

type todoNodePlan struct {
	PlannedEndAt   *time.Time `json:"planned_end_at,omitempty"`
	AssigneeUserID *uint      `json:"assignee_user_id,omitempty"`
}

type todoAIJobPayload struct {
	AIRunID uint   `json:"ai_run_id"`
	NodeID  string `json:"node_id"`
}

func (repository *ReadRepository) ListPrincipalTodos(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
) ([]workflow.TodoSnapshot, error) {
	var tokens []WorkflowToken
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND status = ?", workspaceID, "active",
	).Order("updated_at DESC, id DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return []workflow.TodoSnapshot{}, nil
	}
	instanceIDs := make([]uint, 0, len(tokens))
	for _, token := range tokens {
		instanceIDs = append(instanceIDs, token.InstanceID)
	}
	var instances []WorkflowInstance
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND id IN ? AND LOWER(status) = ?", workspaceID, instanceIDs, "running",
	).Find(&instances).Error; err != nil {
		return nil, err
	}
	instancesByID := make(map[uint]WorkflowInstance, len(instances))
	versionIDs, templateIDs := make([]uint, 0, len(instances)), make([]uint, 0, len(instances))
	for _, instance := range instances {
		instancesByID[instance.ID] = instance
		versionIDs = append(versionIDs, instance.TemplateVersionID)
		templateIDs = append(templateIDs, instance.TemplateID)
	}
	var versions []WorkflowTemplateVersion
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id IN ?", workspaceID, versionIDs).Find(&versions).Error; err != nil {
		return nil, err
	}
	definitions := make(map[uint]todoDefinition, len(versions))
	for _, version := range versions {
		var definition todoDefinition
		if json.Unmarshal([]byte(version.Definition), &definition) == nil {
			definitions[version.ID] = definition
		}
	}
	var templates []WorkflowTemplate
	if err := repository.db.WithContext(ctx).Select("id", "name").Where(
		"workspace_id = ? AND id IN ?", workspaceID, templateIDs,
	).Find(&templates).Error; err != nil {
		return nil, err
	}
	templateNames := make(map[uint]string, len(templates))
	for _, template := range templates {
		templateNames[template.ID] = template.Name
	}
	aiRuns, err := repository.todoAIRunIDs(ctx, workspaceID, instanceIDs)
	if err != nil {
		return nil, err
	}
	items := make([]workflow.TodoSnapshot, 0, len(tokens))
	for _, token := range tokens {
		instance, exists := instancesByID[token.InstanceID]
		if !exists {
			continue
		}
		definition, exists := definitions[instance.TemplateVersionID]
		if !exists {
			continue
		}
		node, exists := todoNodeByID(definition, token.NodeID)
		if !exists || !todoNodeNeedsAssignee(node) {
			continue
		}
		plans, err := todoNodePlans(instance.NodePlansJSON)
		if err != nil {
			continue
		}
		assigneeID, err := repository.todoEffectiveAssignee(ctx, instance, token, definition, plans)
		if err != nil || assigneeID != principalID {
			continue
		}
		actionKind, allowed, visible := todoNodeActions(definition, node, token, aiRuns[todoAIKey(instance.ID, node.ID)], plans[node.ID])
		if !visible {
			continue
		}
		item := workflow.TodoSnapshot{
			TokenID: token.ID, InstanceID: instance.ID, WorkspaceID: workspaceID,
			NodeID: node.ID, NodeName: node.Name, NodeType: node.Type, Capability: node.Capability,
			TemplateName: templateNames[instance.TemplateID], SubjectType: instance.SubjectType, SubjectID: instance.SubjectID,
			AssigneeUserID: assigneeID, ActionKind: actionKind, AllowedActions: allowed,
			CreatedAt: token.CreatedAt, UpdatedAt: token.UpdatedAt,
		}
		if plan, ok := plans[node.ID]; ok {
			item.DueAt = plan.PlannedEndAt
		}
		if runID := aiRuns[todoAIKey(instance.ID, node.ID)]; runID > 0 {
			item.AIRunID = runID
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		return items[i].TokenID > items[j].TokenID
	})
	return items, nil
}

func (repository *ReadRepository) todoAIRunIDs(ctx context.Context, workspaceID uint, instanceIDs []uint) (map[string]uint, error) {
	var jobs []WorkflowJob
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND instance_id IN ? AND kind = ?", workspaceID, instanceIDs, "ai_run",
	).Order("id").Find(&jobs).Error; err != nil {
		return nil, err
	}
	result := make(map[string]uint)
	for _, job := range jobs {
		var payload todoAIJobPayload
		if json.Unmarshal([]byte(job.PayloadJSON), &payload) == nil && payload.AIRunID > 0 && strings.TrimSpace(payload.NodeID) != "" {
			result[todoAIKey(job.InstanceID, payload.NodeID)] = payload.AIRunID
		}
	}
	return result, nil
}

func (repository *ReadRepository) todoEffectiveAssignee(
	ctx context.Context,
	instance WorkflowInstance,
	token WorkflowToken,
	definition todoDefinition,
	plans map[string]todoNodePlan,
) (uint, error) {
	if todoEntryNode(definition, token.NodeID) && instance.StartedByUserID > 0 && (token.Status == "active" || token.Status == "waiting") {
		if plan, ok := plans[token.NodeID]; ok && plan.AssigneeUserID != nil && *plan.AssigneeUserID > 0 {
			return *plan.AssigneeUserID, nil
		}
		return instance.StartedByUserID, nil
	}
	if token.AssigneeUserID != nil && *token.AssigneeUserID > 0 {
		return *token.AssigneeUserID, nil
	}
	if plan, ok := plans[token.NodeID]; ok && plan.AssigneeUserID != nil && *plan.AssigneeUserID > 0 {
		return *plan.AssigneeUserID, nil
	}
	if todoEntryNode(definition, token.NodeID) && instance.StartedByUserID > 0 {
		return instance.StartedByUserID, nil
	}
	var historical WorkflowToken
	err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND assignee_user_id IS NOT NULL AND assignee_user_id > 0",
		instance.WorkspaceID, instance.ID, token.NodeID,
	).Order("id DESC").First(&historical).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return *historical.AssigneeUserID, nil
}

func todoNodePlans(raw string) (map[string]todoNodePlan, error) {
	result := map[string]todoNodePlan{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func todoNodeByID(definition todoDefinition, nodeID string) (todoNode, bool) {
	for _, node := range definition.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return todoNode{}, false
}

func todoNodeNeedsAssignee(node todoNode) bool {
	switch node.Type {
	case "start", "end", "parallel_split", "parallel_join":
		return false
	default:
		return true
	}
}

func todoEntryNode(definition todoDefinition, nodeID string) bool {
	startID := ""
	for _, node := range definition.Nodes {
		if node.Type == "start" {
			startID = node.ID
			break
		}
	}
	queue, seen := []string{startID}, map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == "" || seen[current] {
			continue
		}
		seen[current] = true
		node, ok := todoNodeByID(definition, current)
		if ok && todoNodeNeedsAssignee(node) {
			if current == nodeID {
				return true
			}
			continue
		}
		for _, edge := range definition.Edges {
			if edge.Source == current && (edge.Kind == "" || edge.Kind == "advance") {
				queue = append(queue, edge.Target)
			}
		}
	}
	return false
}

func todoNodeActions(definition todoDefinition, node todoNode, token WorkflowToken, aiRunID uint, plan todoNodePlan) (string, []string, bool) {
	switch {
	case strings.HasPrefix(node.Type, "ai_"):
		if aiRunID == 0 {
			return "", nil, false
		}
		return "ai", []string{}, true
	case node.Type == "approval":
		actions := todoEdgeActions(definition, node.ID, map[string]string{
			"": "approve", "success": "approve", "approve": "approve", "reject": "reject",
		})
		return "approval", actions, len(actions) > 0
	case node.Type == "condition":
		actions := todoEdgeActions(definition, node.ID, map[string]string{
			"": "condition_yes", "success": "condition_yes", "yes": "condition_yes", "condition_yes": "condition_yes",
			"no": "condition_no", "condition_no": "condition_no",
		})
		return "condition", actions, len(actions) > 0
	case node.Type == "timer":
		readyAt := plan.PlannedEndAt
		if readyAt == nil {
			if delay, ok := node.Config["delay_seconds"].(float64); ok && delay > 0 {
				value := token.CreatedAt.Add(time.Duration(delay * float64(time.Second)))
				readyAt = &value
			}
		}
		if readyAt != nil && readyAt.After(time.Now().UTC()) {
			return "", nil, false
		}
		return "complete", []string{"complete"}, true
	default:
		_ = token
		return "complete", []string{"complete"}, true
	}
}

func todoEdgeActions(definition todoDefinition, nodeID string, supported map[string]string) []string {
	actions := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, edge := range definition.Edges {
		if edge.Source != nodeID {
			continue
		}
		action, ok := supported[strings.ToLower(strings.TrimSpace(edge.Kind))]
		if ok && !seen[action] {
			actions = append(actions, action)
			seen[action] = true
		}
	}
	return actions
}

func todoAIKey(instanceID uint, nodeID string) string {
	return strings.Join([]string{strconv.FormatUint(uint64(instanceID), 10), nodeID}, ":")
}

var _ workflow.TodoReadStore = (*ReadRepository)(nil)
