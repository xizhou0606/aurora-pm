package httpapi

import (
	"errors"
	"strings"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type workflowTopologySelectionError struct{ cause error }

func (err *workflowTopologySelectionError) Error() string { return err.cause.Error() }
func (err *workflowTopologySelectionError) Unwrap() error { return err.cause }

func workflowTopologySelectionFailure(err error) error {
	return &workflowTopologySelectionError{cause: err}
}

func workflowAllNodeIDs(definition workflowDefinition) map[string]bool {
	result := make(map[string]bool, len(definition.Nodes))
	for _, node := range definition.Nodes {
		result[node.ID] = true
	}
	return result
}

func workflowAllEdgeIDs(definition workflowDefinition) []string {
	result := make([]string, 0, len(definition.Edges))
	for _, edge := range definition.Edges {
		result = append(result, edge.ID)
	}
	return result
}

type workflowConditionSelection struct {
	Decision     *bool
	TargetNodeID string
}

func workflowLatestConditionSelections(
	definition workflowDefinition,
	runs []workflowgorm.WorkflowNodeRun,
) map[string]workflowConditionSelection {
	statuses, _ := workflowTimelineState(definition, runs)
	result := map[string]workflowConditionSelection{}
	seen := map[string]bool{}
	for index := len(runs) - 1; index >= 0; index-- {
		run := runs[index]
		if seen[run.NodeID] || run.NodeType != "condition" || run.Status != "completed" || statuses[run.NodeID] != "completed" {
			continue
		}
		// The newest completed run is authoritative. If it has no decision (for
		// example after a rollback/retry), do not revive an older branch choice.
		seen[run.NodeID] = true
		actor := decodeWorkflowRunActor(run.OutputJSON)
		if actor.TransitionTargetNodeID != "" {
			result[run.NodeID] = workflowConditionSelection{
				Decision: actor.Decision, TargetNodeID: actor.TransitionTargetNodeID,
			}
		} else if actor.Decision != nil {
			decision := *actor.Decision
			result[run.NodeID] = workflowConditionSelection{Decision: &decision}
		}
	}
	return result
}

func workflowSelectedNodeIDsForInstance(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, runs []workflowgorm.WorkflowNodeRun) (map[string]bool, error) {
	selected, _, err := workflowSelectedTopologyForInstance(tx, definition, instance, runs)
	return selected, err
}

func workflowSelectedTopologyForInstance(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, runs []workflowgorm.WorkflowNodeRun) (map[string]bool, []string, error) {
	conditionSelections := workflowLatestConditionSelections(definition, runs)
	nodes := make(map[string]workflowNode, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodes[node.ID] = node
	}
	selectedEdges := make(map[string]bool, len(definition.Edges))
	for _, edge := range definition.Edges {
		selectedEdges[edge.ID] = true
	}
	selected := map[string]bool{}
	queue := make([]string, 0, 1)
	for _, node := range definition.Nodes {
		if node.Type == "start" {
			queue = append(queue, node.ID)
		}
	}
	if len(queue) == 0 {
		return nil, nil, errors.New("workflow start node is unavailable")
	}
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if selected[nodeID] {
			continue
		}
		selected[nodeID] = true
		node := nodes[nodeID]
		if selection, decided := conditionSelections[nodeID]; node.Type == "condition" && decided {
			matched := make([]workflowEdge, 0)
			if selection.TargetNodeID != "" {
				action := ""
				if selection.Decision != nil {
					action = "condition_no"
					if *selection.Decision {
						action = "condition_yes"
					}
				}
				for _, edge := range definition.Edges {
					if edge.Source == nodeID && edge.Target == selection.TargetNodeID &&
						(action == "" || workflowEdgeMatchesAction(edge.Kind, action)) {
						matched = append(matched, edge)
					}
				}
			} else if selection.Decision != nil {
				action := "condition_no"
				if *selection.Decision {
					action = "condition_yes"
				}
				var err error
				matched, err = workflowNextEdgesForInstanceTx(tx, definition, instance, nodeID, action)
				if err != nil {
					return nil, nil, err
				}
			}
			if len(matched) == 0 {
				return nil, nil, workflowTopologySelectionFailure(
					errors.New("recorded workflow condition target is unavailable"),
				)
			}
			matchedIDs := make(map[string]bool, len(matched))
			for _, edge := range matched {
				matchedIDs[edge.ID] = true
			}
			for _, edge := range definition.Edges {
				if edge.Source == nodeID && !matchedIDs[edge.ID] {
					delete(selectedEdges, edge.ID)
				}
			}
		}
		for _, edge := range definition.Edges {
			if edge.Source == nodeID && selectedEdges[edge.ID] {
				queue = append(queue, edge.Target)
			}
		}
	}
	selectedEdgeIDs := make([]string, 0, len(selectedEdges))
	for _, edge := range definition.Edges {
		if selectedEdges[edge.ID] {
			selectedEdgeIDs = append(selectedEdgeIDs, edge.ID)
		}
	}
	return selected, selectedEdgeIDs, nil
}

func workflowDisplayTopology(definition workflowDefinition, runs []workflowgorm.WorkflowNodeRun, conditionSelectedEdgeIDs, currentNodeIDs []string) ([]string, []string) {
	candidates := make(map[string]bool, len(conditionSelectedEdgeIDs))
	for _, edgeID := range conditionSelectedEdgeIDs {
		candidates[edgeID] = true
	}
	takenRecoveryEdges := workflowLatestTakenRecoveryEdgeIDs(definition, runs)
	for edgeID := range workflowCurrentRecoveryEdgeIDs(definition, runs, currentNodeIDs) {
		takenRecoveryEdges[edgeID] = true
	}
	takenRecoverySources := map[string]bool{}
	for _, edge := range definition.Edges {
		if takenRecoveryEdges[edge.ID] && !workflowRecoveryReturnsToSource(definition, edge, candidates) {
			takenRecoverySources[edge.Source] = true
		}
	}
	for _, edge := range definition.Edges {
		isRecovery := edge.Kind == "failure" || edge.Kind == "reject"
		if isRecovery && !takenRecoveryEdges[edge.ID] {
			delete(candidates, edge.ID)
			continue
		}
		if !isRecovery && takenRecoverySources[edge.Source] {
			delete(candidates, edge.ID)
		}
	}
	reachable := map[string]bool{}
	queue := make([]string, 0, 1)
	for _, node := range definition.Nodes {
		if node.Type == "start" {
			queue = append(queue, node.ID)
		}
	}
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if reachable[nodeID] {
			continue
		}
		reachable[nodeID] = true
		for _, edge := range definition.Edges {
			if edge.Source == nodeID && candidates[edge.ID] {
				queue = append(queue, edge.Target)
			}
		}
	}
	nodeIDs := make([]string, 0, len(reachable))
	for _, node := range definition.Nodes {
		if reachable[node.ID] {
			nodeIDs = append(nodeIDs, node.ID)
		}
	}
	edgeIDs := make([]string, 0, len(candidates))
	for _, edge := range definition.Edges {
		if candidates[edge.ID] && reachable[edge.Source] && reachable[edge.Target] {
			edgeIDs = append(edgeIDs, edge.ID)
		}
	}
	return nodeIDs, edgeIDs
}

func workflowRecoveryReturnsToSource(
	definition workflowDefinition,
	recovery workflowEdge,
	candidates map[string]bool,
) bool {
	reachable := map[string]bool{}
	queue := []string{recovery.Target}
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if nodeID == recovery.Source {
			return true
		}
		if reachable[nodeID] {
			continue
		}
		reachable[nodeID] = true
		for _, edge := range definition.Edges {
			if edge.Source != nodeID || !candidates[edge.ID] ||
				edge.Kind == "failure" || edge.Kind == "reject" {
				continue
			}
			queue = append(queue, edge.Target)
		}
	}
	return false
}

func workflowCurrentRecoveryEdgeIDs(definition workflowDefinition, runs []workflowgorm.WorkflowNodeRun, currentNodeIDs []string) map[string]bool {
	statuses, _ := workflowTimelineState(definition, runs)
	current := map[string]bool{}
	for _, nodeID := range currentNodeIDs {
		current[nodeID] = true
	}
	candidates := map[string][]workflowEdge{}
	for _, edge := range definition.Edges {
		if !current[edge.Target] {
			continue
		}
		if edge.Kind == "reject" && statuses[edge.Source] == "rejected" || edge.Kind == "failure" && statuses[edge.Source] == "failed" {
			candidates[edge.Target] = append(candidates[edge.Target], edge)
		}
	}
	taken := map[string]bool{}
	for _, edges := range candidates {
		if len(edges) == 1 {
			taken[edges[0].ID] = true
		}
	}
	return taken
}

func workflowLatestTakenRecoveryEdgeIDs(definition workflowDefinition, runs []workflowgorm.WorkflowNodeRun) map[string]bool {
	statuses, _ := workflowTimelineState(definition, runs)
	taken := map[string]bool{}
	seen := map[string]bool{}
	for index := len(runs) - 1; index >= 0; index-- {
		run := runs[index]
		actor := decodeWorkflowRunActor(run.OutputJSON)
		completedAIRecovery := workflowCompletedAIRecoveryTarget(definition, run, actor) != ""
		if seen[run.NodeID] || run.Status == "supplemented" ||
			statuses[run.NodeID] != run.Status && !completedAIRecovery {
			continue
		}
		seen[run.NodeID] = true
		if actor.RollbackTargetNodeID != "" || actor.TransitionTargetNodeID == "" {
			continue
		}
		for _, edge := range definition.Edges {
			if edge.Source != run.NodeID || edge.Target != actor.TransitionTargetNodeID {
				continue
			}
			switch edge.Kind {
			case "reject":
				if run.Status == "rejected" {
					taken[edge.ID] = true
				}
			case "failure":
				if run.Status == "failed" || run.Status == "completed" && strings.HasPrefix(run.NodeType, "ai_") && workflowRecoveryTargetIsUnambiguous(definition, edge) {
					taken[edge.ID] = true
				}
			}
		}
	}
	return taken
}

func workflowRecoveryTargetIsUnambiguous(definition workflowDefinition, recovery workflowEdge) bool {
	for _, edge := range definition.Edges {
		if edge.ID != recovery.ID && edge.Source == recovery.Source && edge.Target == recovery.Target && edge.Kind != "failure" && edge.Kind != "reject" {
			return false
		}
	}
	return true
}

func workflowNodeIsSelected(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, nodeID string) (bool, error) {
	var runs []workflowgorm.WorkflowNodeRun
	if err := tx.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error; err != nil {
		return false, err
	}
	selected, err := workflowSelectedNodeIDsForInstance(tx, definition, instance, runs)
	if err != nil {
		return false, err
	}
	return selected[nodeID], nil
}
