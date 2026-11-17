package workflowgorm

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyWorkflowOpinionCreate(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	node, found := state.definition.Node(command.NodeID)
	if !found || !workflow.SupportsOpinions(node) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	inviteIDs := uniqueWorkflowUserIDs(command.InviteUserIDs)
	if len(inviteIDs) > 0 {
		if err := dependencies.Users.VerifyActiveWorkspaceUsers(ctx, command.WorkspaceID, inviteIDs); err != nil {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
	}
	result := workflow.MutationResult{
		ID: state.instance.ID, WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID,
		NodeID: node.ID, InvitedUserIDs: inviteIDs,
	}
	now := time.Now().UTC()
	if command.Body != "" {
		row := WorkflowNodeOpinion{
			WorkspaceID: command.WorkspaceID, InstanceID: state.instance.ID, NodeID: node.ID,
			AuthorUserID: command.PrincipalID, Body: command.Body, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return workflow.MutationResult{}, err
		}
		opinion := workflowOpinionResult(row)
		result.ID, result.Opinion = row.ID, &opinion
	}
	for _, inviteeID := range inviteIDs {
		if inviteeID == command.PrincipalID {
			continue
		}
		row := WorkflowOpinionInvitation{
			WorkspaceID: command.WorkspaceID, InstanceID: state.instance.ID, NodeID: node.ID,
			InviterUserID: command.PrincipalID, InviteeUserID: inviteeID, ApplyKey: command.ApplyKey, CreatedAt: now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return workflow.MutationResult{}, err
		}
	}
	if len(inviteIDs) > 0 {
		if err := dependencies.Invitations.RecordPreReviewInvitation(ctx, workflow.PreReviewInvitation{
			WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
			ProjectSetID: state.subject.ProjectSetID, TeamID: state.subject.TeamID,
			InstanceID: state.instance.ID, SubjectID: state.subject.SubjectID,
			SubjectType: state.subject.SubjectType, SubjectTitle: state.subject.SubjectTitle,
			NodeID: node.ID, NodeName: node.Name, InviteUserIDs: inviteIDs,
		}); err != nil {
			return workflow.MutationResult{}, err
		}
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, NodeID: node.ID, Action: string(command.Operation), Status: state.instance.Status, ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return result, nil
}

func applyWorkflowOpinionUpdate(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	node, found := state.definition.Node(command.NodeID)
	if !found || !workflow.SupportsOpinions(node) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	var row WorkflowNodeOpinion
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ?",
		command.OpinionID, command.WorkspaceID, state.instance.ID, node.ID,
	).First(&row).Error; err != nil {
		return workflow.MutationResult{}, normalizeWorkflowMutationError(err)
	}
	if row.AuthorUserID != command.PrincipalID {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	now := time.Now().UTC()
	updated := tx.Model(&WorkflowNodeOpinion{}).Where(
		"id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ? AND author_user_id = ? AND updated_at = ?",
		row.ID, row.WorkspaceID, row.InstanceID, row.NodeID, command.PrincipalID, row.UpdatedAt,
	).Updates(map[string]any{"body": command.Body, "updated_at": now})
	if updated.Error != nil {
		return workflow.MutationResult{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	row.Body, row.UpdatedAt = command.Body, now
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, NodeID: node.ID, Action: string(command.Operation), Status: state.instance.Status, ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	opinion := workflowOpinionResult(row)
	return workflow.MutationResult{
		ID: row.ID, WorkspaceID: row.WorkspaceID, InstanceID: row.InstanceID, NodeID: row.NodeID, Opinion: &opinion,
	}, nil
}

func workflowOpinionResult(row WorkflowNodeOpinion) workflow.NodeOpinion {
	return workflow.NodeOpinion{
		ID: row.ID, WorkspaceID: row.WorkspaceID, InstanceID: row.InstanceID, NodeID: row.NodeID,
		AuthorUserID: row.AuthorUserID, Body: row.Body, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Edited: row.UpdatedAt.After(row.CreatedAt),
	}
}

func uniqueWorkflowUserIDs(values []uint) []uint {
	seen := map[uint]bool{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
