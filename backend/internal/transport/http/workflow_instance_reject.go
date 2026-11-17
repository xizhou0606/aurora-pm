package httpapi

import workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"

func workflowPreviousCompletedBusinessNode(definition workflowDefinition, runs []workflowgorm.WorkflowNodeRun, sourceNodeID string) string {
	statuses, _ := workflowTimelineState(definition, runs)
	latestCompletedRunID := map[string]uint{}
	for _, run := range runs {
		if run.Status == "completed" {
			latestCompletedRunID[run.NodeID] = run.ID
		}
	}
	structural := func(node workflowNode) bool {
		switch node.Type {
		case "start", "end", "parallel_split", "parallel_join":
			return true
		default:
			return false
		}
	}
	visited := map[string]bool{sourceNodeID: true}
	frontier := []string{sourceNodeID}
	for len(frontier) > 0 {
		frontierIDs := make(map[string]bool, len(frontier))
		for _, nodeID := range frontier {
			frontierIDs[nodeID] = true
		}
		candidates := make([]string, 0)
		next := make([]string, 0)
		for _, edge := range definition.Edges {
			if !frontierIDs[edge.Target] || edge.Kind == "failure" || edge.Kind == "reject" || visited[edge.Source] {
				continue
			}
			visited[edge.Source] = true
			node, found := workflowNodeByID(definition, edge.Source)
			if !found {
				continue
			}
			if structural(node) {
				next = append(next, node.ID)
			} else if statuses[node.ID] == "completed" {
				candidates = append(candidates, node.ID)
			}
		}
		if len(candidates) > 0 {
			selected := candidates[0]
			for _, candidate := range candidates[1:] {
				if latestCompletedRunID[candidate] > latestCompletedRunID[selected] {
					selected = candidate
				}
			}
			return selected
		}
		frontier = uniqueStrings(next)
	}
	return ""
}

func (s *Server) workflowPreviousBusinessNode(instance workflowgorm.WorkflowInstance, definition workflowDefinition, sourceNodeID string) (string, error) {
	var runs []workflowgorm.WorkflowNodeRun
	if err := s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error; err != nil {
		return "", err
	}
	return workflowPreviousCompletedBusinessNode(definition, runs, sourceNodeID), nil
}
