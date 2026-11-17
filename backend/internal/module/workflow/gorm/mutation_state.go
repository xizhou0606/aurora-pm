package workflowgorm

import (
	"encoding/json"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

func currentMutationNodeIDs(raw string) []string {
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil {
		return nil
	}
	result, seen := make([]string, 0, len(values)), map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func containsMutationNode(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeMutationNode(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func uniqueMutationNodes(values []string) []string {
	result, seen := make([]string, 0, len(values)), map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func mutationNodeActionAllowed(node workflow.DefinitionNode, action string) bool {
	switch {
	case node.Type == "approval":
		return action == "approve" || action == "reject"
	case node.Type == "condition":
		return action == "condition_yes" || action == "condition_no"
	case strings.HasPrefix(node.Type, "ai_"):
		return action == "complete" || action == "override_ai"
	default:
		return action == "complete"
	}
}
