package httpapi

import (
	"errors"
	"time"

	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func workflowReachable(definition workflowDefinition, source, target string) (bool, int) {
	if source == target {
		return true, 0
	}
	type step struct {
		id       string
		distance int
	}
	queue := []step{{id: source}}
	visited := map[string]bool{source: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range definition.Edges {
			if edge.Source != current.id || edge.Kind == "failure" || edge.Kind == "reject" || visited[edge.Target] {
				continue
			}
			if edge.Target == target {
				return true, current.distance + 1
			}
			visited[edge.Target] = true
			queue = append(queue, step{id: edge.Target, distance: current.distance + 1})
		}
	}
	return false, 0
}

func workflowMatchingParallelSplit(definition workflowDefinition, joinID string) (workflowNode, []string, bool) {
	bestDistance := int(^uint(0) >> 1)
	var best workflowNode
	var bestBranches []string
	for _, node := range definition.Nodes {
		if node.Type != "parallel_split" {
			continue
		}
		branches := workflowNextNodes(definition, node.ID, "complete")
		if len(branches) < 2 {
			continue
		}
		distance, allReach := 0, true
		for _, branch := range branches {
			reachable, branchDistance := workflowReachable(definition, branch, joinID)
			if !reachable {
				allReach = false
				break
			}
			distance += branchDistance
		}
		if allReach && distance < bestDistance {
			bestDistance, best, bestBranches = distance, node, branches
		}
	}
	return best, bestBranches, best.ID != ""
}

func (s *Server) workflowTransitionReached(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, sourceID, targetID string) bool {
	var run workflowgorm.WorkflowNodeRun
	lookup := tx.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?", instance.WorkspaceID, instance.ID, sourceID, []string{"completed", "rejected", "failed"}).Order("id DESC").Limit(1).Find(&run)
	if lookup.Error != nil || lookup.RowsAffected == 0 {
		return false
	}
	action := "complete"
	switch {
	case run.NodeType == "condition":
		actor := decodeWorkflowRunActor(run.OutputJSON)
		if actor.Decision == nil {
			return false
		}
		if *actor.Decision {
			action = "condition_yes"
		} else {
			action = "condition_no"
		}
	case run.NodeType == "approval" && run.Status == "rejected":
		action = "reject"
	case run.NodeType == "approval":
		action = "approve"
	case run.Status == "failed":
		action = "failure"
	}
	targets, err := workflowNextNodesForInstanceTx(tx, definition, instance, sourceID, action)
	return err == nil && containsString(targets, targetID)
}

func (s *Server) workflowParallelJoinProgress(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, joinID string) (int, int) {
	_, branches, paired := workflowMatchingParallelSplit(definition, joinID)
	if paired {
		completed := 0
		for _, branch := range branches {
			ready := false
			for _, edge := range definition.Edges {
				if edge.Target != joinID {
					continue
				}
				reachable, _ := workflowReachable(definition, branch, edge.Source)
				if reachable && s.workflowTransitionReached(tx, definition, instance, edge.Source, joinID) {
					ready = true
					break
				}
			}
			if ready {
				completed++
			}
		}
		return completed, len(branches)
	}
	completed, total := 0, 0
	for _, edge := range definition.Edges {
		if edge.Target == joinID {
			total++
			if s.workflowTransitionReached(tx, definition, instance, edge.Source, joinID) {
				completed++
			}
		}
	}
	return completed, total
}

func (s *Server) workflowJoinReady(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, nodeID string) bool {
	completed, total := s.workflowParallelJoinProgress(tx, definition, instance, nodeID)
	return total > 0 && completed == total
}

func completeAutomaticWorkflowNode(tx *gorm.DB, instance workflowgorm.WorkflowInstance, node workflowNode, now time.Time) error {
	run := workflowgorm.WorkflowNodeRun{WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID, NodeType: node.Type, Status: "completed", StartedAt: now, CompletedAt: &now}
	if err := tx.Create(&run).Error; err != nil {
		return err
	}
	return tx.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?", instance.WorkspaceID, instance.ID, node.ID, []string{"active", "waiting"}).
		Update("status", "completed").Error
}

func (s *Server) advanceAutomaticWorkflowNodesTx(tx *gorm.DB, instance *workflowgorm.WorkflowInstance, definition workflowDefinition, current []string, now time.Time) ([]string, error) {
	for attempts := 0; attempts <= len(definition.Nodes); attempts++ {
		advanced := false
		nextCurrent := make([]string, 0, len(current))
		for _, nodeID := range current {
			node, found := workflowNodeByID(definition, nodeID)
			if !found {
				return nil, errors.New("workflow automatic node is unavailable")
			}
			if node.Type == "parallel_join" && !s.workflowJoinReady(tx, definition, *instance, node.ID) {
				if err := tx.Model(&workflowgorm.WorkflowToken{}).
					Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "active").
					Update("status", "waiting").Error; err != nil {
					return nil, err
				}
				nextCurrent = append(nextCurrent, node.ID)
				continue
			}
			if autoGuard, isAuto, autoErr := s.workflowAutoAdvanceGuard(*instance, node); autoErr != nil {
				return nil, autoErr
			} else if isAuto {
				ready, err := s.workflowAutoNodeReadyTx(tx, *instance, autoGuard)
				if err != nil {
					return nil, err
				}
				if !ready {
					claimed := tx.Model(&workflowgorm.WorkflowToken{}).
						Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "active").
						Update("status", "waiting")
					if claimed.Error != nil {
						return nil, claimed.Error
					}
					if claimed.RowsAffected == 0 {
						if err := tx.Create(&workflowgorm.WorkflowToken{
							WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID,
							NodeID: node.ID, Status: "waiting", CreatedAt: now, UpdatedAt: now,
						}).Error; err != nil {
							return nil, err
						}
					}
					nextCurrent = append(nextCurrent, node.ID)
					continue
				}
				if err := completeRequirementGateWorkflowNode(tx, *instance, node, now); err != nil {
					return nil, err
				}
				nextCurrent = append(nextCurrent, workflowNextNodes(definition, node.ID, "complete")...)
				advanced = true
				continue
			}
			if node.Type != "parallel_split" && node.Type != "parallel_join" {
				nextCurrent = append(nextCurrent, node.ID)
				continue
			}
			if err := completeAutomaticWorkflowNode(tx, *instance, node, now); err != nil {
				return nil, err
			}
			nextCurrent = append(nextCurrent, workflowNextNodes(definition, node.ID, "complete")...)
			advanced = true
		}
		current = uniqueStrings(nextCurrent)
		if !advanced {
			return completeTerminalNodes(tx, instance, definition, current, now)
		}
	}
	return nil, errors.New("workflow automatic connector limit exceeded")
}

// workflowAutoAdvanceGuard returns the guard that drives a node's automatic
// completion: requirement_gate nodes use their configured guard; plan-unit
// human task/form nodes with a requirement stage (explicit requirement_stage
// config or node-name inference) use the stage-driven guard 需求阶段驱动.
func (s *Server) workflowAutoAdvanceGuard(instance workflowgorm.WorkflowInstance, node workflowNode) (*workflow.RequirementsGuard, bool, error) {
	if node.Type == "requirement_gate" {
		guard, present := workflowNodeGuard(node)
		if !present {
			return nil, false, errors.New("requirement gate requires a requirements_guard configuration")
		}
		return guard, true, nil
	}
	if instance.SubjectType == "plan_unit" {
		if guard, ok, _ := workflow.NodeStageGuard(definitionNodeFromWorkflow(node)); ok {
			return guard, true, nil
		}
	}
	return nil, false, nil
}

// workflowAutoNodeReadyTx evaluates the auto-advance guard inside the
// caller's transaction so the completion only commits on current facts.
func (s *Server) workflowAutoNodeReadyTx(tx *gorm.DB, instance workflowgorm.WorkflowInstance, guard *workflow.RequirementsGuard) (bool, error) {
	inputs, err := workflowGuardInputsForInstance(tx, instance)
	if err != nil {
		return false, err
	}
	return guard.Evaluate(inputs).Satisfied(), nil
}

// completeRequirementGateWorkflowNode records the system-actor gate pass so
// the transition history shows the automatic decision and its evidence.
func completeRequirementGateWorkflowNode(tx *gorm.DB, instance workflowgorm.WorkflowInstance, node workflowNode, now time.Time) error {
	run := workflowgorm.WorkflowNodeRun{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID, NodeType: node.Type,
		Status: "completed", OutputJSON: `{"system_actor":true}`, StartedAt: now, CompletedAt: &now,
	}
	if err := tx.Create(&run).Error; err != nil {
		return err
	}
	return tx.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?", instance.WorkspaceID, instance.ID, node.ID, []string{"active", "waiting"}).
		Update("status", "completed").Error
}
