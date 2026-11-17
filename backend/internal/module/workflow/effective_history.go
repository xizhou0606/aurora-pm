package workflow

import (
	"encoding/json"
	"strings"
)

// GuardHistoryRun is the minimal run record the effective-history
// interpretation needs. Callers map their persistence layer onto it.
type GuardHistoryRun struct {
	NodeID     string
	NodeType   string
	Status     string
	OutputJSON string
}

type guardRunActor struct {
	RollbackTargetNodeID   string `json:"rollback_target_node_id,omitempty"`
	TransitionTargetNodeID string `json:"transition_target_node_id,omitempty"`
}

// EffectiveNodeStatuses interprets the valid execution path of a workflow
// instance from its run history. It ports the legacy HTTP timeline semantics:
// the latest relevant run decides a node's status, rollback runs reset every
// node reachable from their target (failure/reject recovery edges are not
// followed), and supplemental materials never change completion state. All
// guard/delivery evidence must come from this single interpreter so rollback
// invalidates downstream evidence everywhere the same way.
func EffectiveNodeStatuses(definition Definition, runs []GuardHistoryRun) map[string]string {
	statuses := make(map[string]string, len(definition.Nodes))
	for _, run := range runs {
		// Supplemental material is immutable node history, not another
		// execution attempt.
		if run.Status == "supplemented" {
			continue
		}
		var actor guardRunActor
		_ = json.Unmarshal([]byte(run.OutputJSON), &actor)
		if actor.RollbackTargetNodeID != "" {
			for nodeID := range guardReachableFrom(definition, actor.RollbackTargetNodeID) {
				statuses[nodeID] = "pending"
			}
			if run.NodeID != actor.RollbackTargetNodeID {
				statuses[run.NodeID] = "rolled_back"
			}
			continue
		}
		if targetNodeID := guardAIRecoveryTarget(definition, run, actor); targetNodeID != "" {
			for nodeID := range guardReachableFrom(definition, targetNodeID) {
				statuses[nodeID] = "pending"
			}
			statuses[run.NodeID] = "rolled_back"
			continue
		}
		statuses[run.NodeID] = run.Status
	}
	return statuses
}

func guardReachableFrom(definition Definition, sourceID string) map[string]bool {
	reachable := map[string]bool{}
	queue := []string{sourceID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if reachable[current] {
			continue
		}
		reachable[current] = true
		for _, edge := range definition.Edges {
			// Failure and rejection edges model recovery paths back into an
			// earlier stage; following them would incorrectly reset completed
			// sibling branches.
			if edge.Source == current && edge.Kind != "failure" && edge.Kind != "reject" && !reachable[edge.Target] {
				queue = append(queue, edge.Target)
			}
		}
	}
	return reachable
}

func guardAIRecoveryTarget(definition Definition, run GuardHistoryRun, actor guardRunActor) string {
	if run.Status != "completed" || !strings.HasPrefix(run.NodeType, "ai_") || actor.TransitionTargetNodeID == "" {
		return ""
	}
	for _, edge := range definition.Edges {
		if edge.Source == run.NodeID && edge.Target == actor.TransitionTargetNodeID &&
			(edge.Kind == "failure" || edge.Kind == "reject") &&
			guardRecoveryTargetIsUnambiguous(definition, edge) {
			return edge.Target
		}
	}
	return ""
}

func guardRecoveryTargetIsUnambiguous(definition Definition, recovery DefinitionEdge) bool {
	for _, edge := range definition.Edges {
		if edge.ID != recovery.ID && edge.Source == recovery.Source && edge.Target == recovery.Target && edge.Kind != "failure" && edge.Kind != "reject" {
			return false
		}
	}
	return true
}
