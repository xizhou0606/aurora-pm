package httpapi

import "strings"

func workflowNodeCollectsRequirementInput(node workflowNode) bool {
	if node.Type != "human_task" && node.Type != "form" {
		return false
	}
	if required, _ := node.Config["requirement_input"].(bool); required {
		return true
	}
	return strings.Contains(node.Name, "需求录入") || strings.Contains(node.Name, "需求收集")
}

func firstAssignableWorkflowNodeID(definition workflowDefinition) string {
	ids := workflowEntryAssigneeNodeIDs(definition)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func workflowEntryAssigneeNodeIDs(definition workflowDefinition) []string {
	startID := ""
	for _, node := range definition.Nodes {
		if node.Type == "start" {
			startID = node.ID
			break
		}
	}
	if startID == "" {
		return nil
	}
	return workflowAssigneeRequiredNodeIDs(definition, workflowNextNodes(definition, startID, "advance"))
}

func workflowNodeIsEntryAssignee(definition workflowDefinition, nodeID string) bool {
	for _, id := range workflowEntryAssigneeNodeIDs(definition) {
		if id == nodeID {
			return true
		}
	}
	return false
}

func firstRequirementEntryNodeID(definition workflowDefinition) string {
	startID := ""
	for _, node := range definition.Nodes {
		if node.Type == "start" {
			startID = node.ID
			break
		}
	}
	if startID == "" {
		return ""
	}
	queue := []string{startID}
	seen := map[string]bool{}
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
		if workflowNodeCollectsRequirementInput(node) {
			return node.ID
		}
		queue = append(queue, workflowNextNodes(definition, nodeID, "advance")...)
	}
	return ""
}
