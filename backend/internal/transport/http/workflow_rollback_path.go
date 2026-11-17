package httpapi

import workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"

// workflowRollbackTargetIDs derives rollback choices from the effective run
// history. A completed node elsewhere in the definition is not enough: its
// recorded forward execution must lead to the requested source.
func workflowRollbackTargetIDs(
	definition workflowDefinition,
	runs []workflowgorm.WorkflowNodeRun,
	sourceNodeID string,
	sourceIsActive bool,
) map[string]bool {
	statuses, latestRuns := workflowRollbackExecutionState(definition, runs)
	startNodeID := ""
	for _, node := range definition.Nodes {
		if node.Type == "start" {
			startNodeID = node.ID
			break
		}
	}
	targets := map[string]bool{}
	for _, node := range definition.Nodes {
		if node.ID == sourceNodeID || workflowRollbackStructuralNode(node) || statuses[node.ID] != "completed" {
			continue
		}
		if startNodeID == "" || !workflowRollbackExecutedPath(definition, statuses, latestRuns, startNodeID, node.ID) {
			continue
		}
		if !workflowRollbackExecutedPath(definition, statuses, latestRuns, node.ID, sourceNodeID) {
			continue
		}
		if sourceIsActive && !workflowRollbackWithinActiveParallelBranch(definition, statuses, sourceNodeID, node.ID) {
			continue
		}
		targets[node.ID] = true
	}
	return targets
}

func workflowRollbackExecutionState(
	definition workflowDefinition,
	runs []workflowgorm.WorkflowNodeRun,
) (map[string]string, map[string]workflowgorm.WorkflowNodeRun) {
	statuses, _ := workflowTimelineState(definition, runs)
	latest := make(map[string]workflowgorm.WorkflowNodeRun, len(statuses))
	for index := len(runs) - 1; index >= 0; index-- {
		run := runs[index]
		if !workflowRollbackExecutedStatus(run.Status) || statuses[run.NodeID] != run.Status {
			continue
		}
		if _, exists := latest[run.NodeID]; !exists {
			latest[run.NodeID] = run
		}
	}
	return statuses, latest
}

func workflowRollbackExecutedPath(
	definition workflowDefinition,
	statuses map[string]string,
	latestRuns map[string]workflowgorm.WorkflowNodeRun,
	targetNodeID string,
	sourceNodeID string,
) bool {
	if targetNodeID == sourceNodeID || statuses[targetNodeID] != "completed" {
		return false
	}
	visited := map[string]bool{}
	queue := []string{targetNodeID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		for _, edge := range definition.Edges {
			if edge.Source != current || !workflowRollbackEdgeWasExecuted(definition, latestRuns, edge) {
				continue
			}
			if edge.Target == sourceNodeID {
				return true
			}
			if !visited[edge.Target] && workflowRollbackExecutedStatus(statuses[edge.Target]) {
				queue = append(queue, edge.Target)
			}
		}
	}
	return false
}

func workflowRollbackEdgeWasExecuted(
	definition workflowDefinition,
	latestRuns map[string]workflowgorm.WorkflowNodeRun,
	edge workflowEdge,
) bool {
	node, found := workflowNodeByID(definition, edge.Source)
	run, executed := latestRuns[edge.Source]
	if !found || !executed {
		return false
	}
	actor := decodeWorkflowRunActor(run.OutputJSON)
	if actor.TransitionTargetNodeID != "" {
		return actor.TransitionTargetNodeID == edge.Target
	}
	if node.Type == "parallel_split" {
		return workflowEdgeMatchesAction(edge.Kind, "complete")
	}
	action := "complete"
	if run.Status == "failed" {
		action = "failure"
	} else if run.Status == "rejected" {
		action = "reject"
	} else if node.Type == "approval" {
		action = "approve"
	} else if node.Type == "condition" {
		if actor.Decision == nil {
			return false
		}
		if *actor.Decision {
			action = "condition_yes"
		} else {
			action = "condition_no"
		}
	}
	matched := make([]workflowEdge, 0, 1)
	for _, candidate := range definition.Edges {
		if candidate.Source == edge.Source && workflowEdgeMatchesAction(candidate.Kind, action) {
			matched = append(matched, candidate)
		}
	}
	return len(matched) == 1 && matched[0].ID == edge.ID
}

func workflowRollbackExecutedStatus(status string) bool {
	return status == "completed" || status == "rejected" || status == "failed"
}

// An active node inside a parallel branch may only roll back within that same
// branch. Rewinding to the split's shared ancestry would invalidate every
// sibling, so that operation fails closed instead of partially resetting the
// parallel group.
func workflowRollbackWithinActiveParallelBranch(
	definition workflowDefinition,
	statuses map[string]string,
	sourceNodeID string,
	targetNodeID string,
) bool {
	for _, join := range definition.Nodes {
		if join.Type != "parallel_join" {
			continue
		}
		split, branches, paired := workflowMatchingParallelSplit(definition, join.ID)
		if !paired || statuses[split.ID] != "completed" || sourceNodeID == join.ID {
			continue
		}
		sourceBranches := make([]string, 0, 1)
		for _, branch := range branches {
			fromBranch, _ := workflowReachable(definition, branch, sourceNodeID)
			toJoin, _ := workflowReachable(definition, sourceNodeID, join.ID)
			if fromBranch && toJoin {
				sourceBranches = append(sourceBranches, branch)
			}
		}
		if len(sourceBranches) == 0 {
			continue
		}
		if len(sourceBranches) != 1 {
			return false
		}
		fromBranch, _ := workflowReachable(definition, sourceBranches[0], targetNodeID)
		toJoin, _ := workflowReachable(definition, targetNodeID, join.ID)
		if !fromBranch || !toJoin || targetNodeID == join.ID || targetNodeID == split.ID {
			return false
		}
	}
	return true
}

func workflowRollbackStructuralNode(node workflowNode) bool {
	switch node.Type {
	case "start", "end", "parallel_split", "parallel_join":
		return true
	default:
		return false
	}
}

func workflowRollbackCurrentPlan(
	definition workflowDefinition,
	runs []workflowgorm.WorkflowNodeRun,
	current []string,
	sourceNodeID string,
	targetNodeID string,
) (affected []string, next []string, ok bool) {
	statuses, latestRuns := workflowRollbackExecutionState(definition, runs)
	affectedSet := map[string]bool{}
	for _, nodeID := range current {
		if nodeID == sourceNodeID || workflowRollbackExecutedPath(definition, statuses, latestRuns, targetNodeID, nodeID) {
			affected = append(affected, nodeID)
			affectedSet[nodeID] = true
		}
	}
	if len(current) > 0 && !affectedSet[sourceNodeID] {
		return nil, nil, false
	}
	insertedTarget := false
	for _, nodeID := range current {
		if affectedSet[nodeID] {
			if !insertedTarget {
				next = append(next, targetNodeID)
				insertedTarget = true
			}
			continue
		}
		next = append(next, nodeID)
	}
	if !insertedTarget {
		next = append(next, targetNodeID)
	}
	return uniqueStrings(affected), uniqueStrings(next), true
}
