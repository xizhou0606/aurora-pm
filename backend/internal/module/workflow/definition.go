package workflow

import (
	"encoding/json"
	"strings"
)

type Definition struct {
	Nodes []DefinitionNode `json:"nodes"`
	Edges []DefinitionEdge `json:"edges"`
	// DeliveryProgress carries the template-level 需求交付进度 config; it is
	// parsed lazily via ParseDeliveryProgress so malformed authoring output is
	// rejected at save/publish time, not silently here.
	DeliveryProgressConfig map[string]any `json:"delivery_progress,omitempty"`
}

type DefinitionNode struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Capability string         `json:"capability,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

type DefinitionEdge struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Kind      string `json:"kind,omitempty"`
	Condition string `json:"condition,omitempty"`
}

func ParseDefinition(raw string) (Definition, error) {
	var definition Definition
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &definition) != nil || len(definition.Nodes) < 2 || len(definition.Edges) == 0 {
		return Definition{}, ErrMutationConflict
	}
	nodes, edges := map[string]bool{}, map[string]bool{}
	for index := range definition.Nodes {
		node := &definition.Nodes[index]
		node.ID, node.Type, node.Name = strings.TrimSpace(node.ID), strings.TrimSpace(node.Type), strings.TrimSpace(node.Name)
		node.Capability = strings.TrimSpace(node.Capability)
		if node.ID == "" || node.Type == "" || node.Name == "" || nodes[node.ID] || strings.HasPrefix(node.Type, "ai_") && node.Capability == "" {
			return Definition{}, ErrMutationConflict
		}
		nodes[node.ID] = true
	}
	for index := range definition.Edges {
		edge := &definition.Edges[index]
		edge.ID, edge.Source, edge.Target, edge.Kind, edge.Condition = strings.TrimSpace(edge.ID), strings.TrimSpace(edge.Source), strings.TrimSpace(edge.Target), strings.TrimSpace(edge.Kind), strings.TrimSpace(edge.Condition)
		if edge.ID == "" || edges[edge.ID] || !nodes[edge.Source] || !nodes[edge.Target] {
			return Definition{}, ErrMutationConflict
		}
		edges[edge.ID] = true
	}
	return definition, nil
}

func (definition Definition) Node(id string) (DefinitionNode, bool) {
	for _, node := range definition.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return DefinitionNode{}, false
}

func (definition Definition) Next(nodeID, action string) []string {
	result, seen := make([]string, 0, 2), map[string]bool{}
	for _, edge := range definition.Edges {
		if edge.Source == nodeID && edgeMatchesAction(edge.Kind, edge.Condition, action) && !seen[edge.Target] {
			seen[edge.Target] = true
			result = append(result, edge.Target)
		}
	}
	return result
}

func edgeMatchesAction(kind, condition, action string) bool {
	condition = strings.ToLower(strings.Join(strings.Fields(condition), " "))
	if action == "failure" || action == "reject" {
		return kind == action
	}
	if action == "condition_yes" {
		if kind == "yes" || kind == action {
			return true
		}
		return (kind == "" || kind == "success") && condition == "context.decision == true"
	}
	if action == "condition_no" {
		if kind == "no" || kind == action {
			return true
		}
		return kind == "" && condition == "context.decision == false"
	}
	return condition == "" && (kind == "" || kind == "success" || kind == action)
}

func SupportsOpinions(node DefinitionNode) bool {
	if node.Type != "approval" {
		return false
	}
	marker := strings.ToLower(node.ID + " " + node.Name)
	if strings.Contains(node.Name, "预审") {
		return true
	}
	for _, token := range []string{"pre-review", "pre_review", "prereview", "preapproval", "pre-approval", "pre_approval"} {
		if strings.Contains(marker, token) {
			return true
		}
	}
	return false
}

func SupportsTestReports(node DefinitionNode) bool {
	configured, _ := node.Config["test_report_artifact"].(bool)
	return (node.Type == "human_task" || node.Type == "form") &&
		(configured || strings.Contains(node.Name, "测试验收") || strings.Contains(node.Name, "测试报告"))
}

func NeedsAssignee(node DefinitionNode) bool {
	switch node.Type {
	case "start", "end", "parallel_split", "parallel_join", "requirement_gate":
		return false
	default:
		return true
	}
}
