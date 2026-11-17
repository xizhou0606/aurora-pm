package workflowgorm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyWorkflowRollback(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	if strings.TrimSpace(command.TargetNodeID) == "" || strings.TrimSpace(command.Comment) == "" ||
		(state.instance.Status != "running" && state.instance.Status != "completed") {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	current := currentMutationNodeIDs(state.instance.CurrentNodeIDs)
	sourceID := strings.TrimSpace(command.NodeID)
	if state.instance.Status == "running" {
		if sourceID == "" && len(current) == 1 {
			sourceID = current[0]
		}
		if !containsMutationNode(current, sourceID) {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
		if _, err := lockActiveMutationToken(tx, state.instance, sourceID, command.PrincipalID); err != nil {
			return workflow.MutationResult{}, err
		}
	}
	runs, sourceNode, targetNode, err := rollbackMutationPath(tx, state, sourceID, command.TargetNodeID)
	if err != nil {
		return workflow.MutationResult{}, err
	}
	_ = runs
	if state.instance.Status == "running" {
		for _, nodeID := range current {
			updated := tx.Model(&WorkflowToken{}).Where(
				"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
				state.instance.WorkspaceID, state.instance.ID, nodeID, []string{"active", "waiting"},
			).Update("status", "rolled_back")
			if updated.Error != nil || updated.RowsAffected != 1 {
				return workflow.MutationResult{}, workflow.ErrMutationConflict
			}
		}
	}
	if err := createHistoricalMutationToken(tx, state.instance, targetNode); err != nil {
		return workflow.MutationResult{}, err
	}
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]any{
		"actor_user_id": command.PrincipalID, "comment": command.Comment, "target_node_id": targetNode.ID,
	})
	if err := tx.Create(&WorkflowNodeRun{
		WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID, NodeID: sourceNode.ID,
		NodeType: sourceNode.Type, Status: "rolled_back", OutputJSON: string(payload), StartedAt: now, CompletedAt: &now,
	}).Error; err != nil {
		return workflow.MutationResult{}, err
	}
	encoded, _ := json.Marshal([]string{targetNode.ID})
	updated := tx.Model(&WorkflowInstance{}).Where(
		"id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ? AND node_plans_json = ?",
		state.instance.ID, state.instance.WorkspaceID, state.instance.Status,
		state.instance.CurrentNodeIDs, state.instance.NodePlansJSON,
	).Updates(map[string]any{"status": "running", "current_node_ids": string(encoded), "completed_at": nil})
	if updated.Error != nil || updated.RowsAffected != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, NodeID: sourceNode.ID, Action: command.Action, Status: "running", ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return workflow.MutationResult{
		ID: state.instance.ID, WorkspaceID: command.WorkspaceID, InstanceID: state.instance.ID,
		NodeID: sourceNode.ID, Status: "running", Action: command.Action, CurrentNodeIDs: []string{targetNode.ID},
	}, nil
}

func rollbackMutationPath(
	tx *gorm.DB,
	state workflowMutationState,
	sourceID, targetID string,
) ([]WorkflowNodeRun, workflow.DefinitionNode, workflow.DefinitionNode, error) {
	var runs []WorkflowNodeRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND instance_id = ?", state.instance.WorkspaceID, state.instance.ID,
	).Order("id").Find(&runs).Error; err != nil {
		return nil, workflow.DefinitionNode{}, workflow.DefinitionNode{}, err
	}
	if sourceID == "" && state.instance.Status == "completed" {
		for index := len(runs) - 1; index >= 0; index-- {
			if runs[index].Status == "completed" {
				sourceID = runs[index].NodeID
				break
			}
		}
	}
	source, sourceFound := state.definition.Node(sourceID)
	target, targetFound := state.definition.Node(strings.TrimSpace(targetID))
	if !sourceFound || !targetFound || !workflow.NeedsAssignee(target) {
		return nil, workflow.DefinitionNode{}, workflow.DefinitionNode{}, workflow.ErrMutationConflict
	}
	targetSeen := false
	for _, run := range runs {
		if run.NodeID == target.ID && run.Status == "completed" {
			targetSeen = true
		}
		if run.NodeID == source.ID && targetSeen {
			return runs, source, target, nil
		}
	}
	return nil, workflow.DefinitionNode{}, workflow.DefinitionNode{}, workflow.ErrMutationConflict
}

func applyWorkflowResume(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	if state.instance.Status != "rejected" {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	var rejected WorkflowNodeRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND instance_id = ? AND status = ?",
		state.instance.WorkspaceID, state.instance.ID, "rejected",
	).Order("id DESC").First(&rejected).Error; err != nil {
		return workflow.MutationResult{}, normalizeWorkflowMutationError(err)
	}
	targets := state.definition.Next(rejected.NodeID, "reject")
	if len(targets) != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	target, found := state.definition.Node(targets[0])
	if !found || !workflow.NeedsAssignee(target) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if err := createHistoricalMutationToken(tx, state.instance, target); err != nil {
		return workflow.MutationResult{}, err
	}
	encoded, _ := json.Marshal([]string{target.ID})
	updated := tx.Model(&WorkflowInstance{}).Where(
		"id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ? AND node_plans_json = ?",
		state.instance.ID, state.instance.WorkspaceID, "rejected", state.instance.CurrentNodeIDs, state.instance.NodePlansJSON,
	).Updates(map[string]any{"status": "running", "current_node_ids": string(encoded), "completed_at": nil})
	if updated.Error != nil || updated.RowsAffected != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, NodeID: rejected.NodeID, Action: command.Action, Status: "running", ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return workflow.MutationResult{
		ID: state.instance.ID, WorkspaceID: command.WorkspaceID, InstanceID: state.instance.ID,
		NodeID: rejected.NodeID, Status: "running", Action: command.Action, CurrentNodeIDs: []string{target.ID},
	}, nil
}

func createHistoricalMutationToken(tx *gorm.DB, instance WorkflowInstance, node workflow.DefinitionNode) error {
	plans, err := decodeMutationNodePlans(instance.NodePlansJSON)
	if err != nil {
		return err
	}
	assignee := plans[node.ID].AssigneeUserID
	if assignee == nil {
		var historical WorkflowToken
		if err := tx.Where(
			"workspace_id = ? AND instance_id = ? AND node_id = ? AND assignee_user_id IS NOT NULL",
			instance.WorkspaceID, instance.ID, node.ID,
		).Order("id DESC").First(&historical).Error; err != nil {
			return workflow.ErrMutationConflict
		}
		assignee = historical.AssigneeUserID
	}
	if assignee == nil || *assignee == 0 {
		return workflow.ErrMutationConflict
	}
	return tx.Create(&WorkflowToken{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
		Status: "active", AssigneeUserID: assignee,
	}).Error
}
