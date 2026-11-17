package workflowgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyWorkflowNodePlans(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	plans, err := decodeMutationNodePlans(state.instance.NodePlansJSON)
	if err != nil {
		return workflow.MutationResult{}, err
	}
	assigneeIDs := make([]uint, 0, len(command.NodePlans))
	assignments := make([]workflow.SubjectAssignment, 0, len(command.NodePlans))
	dateChanged := false
	for _, update := range command.NodePlans {
		if update.AssigneeUserIDSet {
			assigneeIDs = append(assigneeIDs, update.AssigneeUserID)
		}
	}
	if len(assigneeIDs) > 0 {
		if err := dependencies.Users.VerifyActiveWorkspaceUsers(ctx, command.WorkspaceID, uniqueWorkflowUserIDs(assigneeIDs)); err != nil {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
	}
	for _, update := range command.NodePlans {
		nodeID := strings.TrimSpace(update.NodeID)
		node, found := state.definition.Node(nodeID)
		if !found || !workflow.NeedsAssignee(node) {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
		plan := plans[nodeID]
		assigneeChanged := update.AssigneeUserIDSet &&
			(plan.AssigneeUserID == nil || *plan.AssigneeUserID != update.AssigneeUserID)
		if update.PlannedStartAt.Set && !sameMutationTime(plan.PlannedStartAt, update.ExpectedStartAt.Value) ||
			update.PlannedEndAt.Set && !sameMutationTime(plan.PlannedEndAt, update.ExpectedEndAt.Value) {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
		if update.PlannedStartAt.Set {
			dateChanged = dateChanged || !sameMutationTime(plan.PlannedStartAt, update.PlannedStartAt.Value)
			plan.PlannedStartAt = update.PlannedStartAt.Value
		}
		if update.PlannedEndAt.Set {
			dateChanged = dateChanged || !sameMutationTime(plan.PlannedEndAt, update.PlannedEndAt.Value)
			plan.PlannedEndAt = update.PlannedEndAt.Value
		}
		if update.AssigneeUserIDSet {
			value := update.AssigneeUserID
			plan.AssigneeUserID = &value
			if assigneeChanged {
				assignments = append(assignments, workflow.SubjectAssignment{
					WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
					ProjectSetID: state.subject.ProjectSetID, TeamID: state.subject.TeamID,
					InstanceID: state.instance.ID, SubjectID: state.subject.SubjectID,
					SubjectType: state.subject.SubjectType, SubjectTitle: state.subject.SubjectTitle,
					NodeID: node.ID, NodeName: node.Name, AssigneeUserID: update.AssigneeUserID,
				})
			}
		}
		if plan.PlannedStartAt != nil && plan.PlannedEndAt != nil && plan.PlannedStartAt.After(*plan.PlannedEndAt) {
			return workflow.MutationResult{}, workflow.ErrMutationInvalid
		}
		plans[nodeID] = plan
		if update.AssigneeUserIDSet {
			if err := updateActiveWorkflowTokenAssignee(tx, state.instance, nodeID, update.AssigneeUserID); err != nil {
				return workflow.MutationResult{}, err
			}
		}
	}
	if dateChanged && !command.ConfirmSchedule || command.ConfirmSchedule && !dateChanged {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if command.ConfirmSchedule {
		start, end := mutationScheduleRange(plans)
		if state.subject.PlanUnitID == 0 || start == nil && end == nil ||
			dependencies.Schedule.ConfirmWorkflowSchedule(ctx, command.WorkspaceID, state.subject, start, end) != nil {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
	}
	encoded, err := json.Marshal(plans)
	if err != nil {
		return workflow.MutationResult{}, err
	}
	updated := tx.Model(&WorkflowInstance{}).Where(
		"id = ? AND workspace_id = ? AND node_plans_json = ?",
		state.instance.ID, state.instance.WorkspaceID, state.instance.NodePlansJSON,
	).Update("node_plans_json", string(encoded))
	if updated.Error != nil {
		return workflow.MutationResult{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	for _, assignment := range assignments {
		if err := dependencies.Assignments.RecordSubjectAssignment(ctx, assignment); err != nil {
			return workflow.MutationResult{}, err
		}
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, Action: string(command.Operation), Status: state.instance.Status, ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return workflow.MutationResult{
		ID: state.instance.ID, WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID,
		Status: state.instance.Status, NodePlans: plans,
	}, nil
}

func mutationScheduleRange(plans map[string]workflow.NodePlan) (*time.Time, *time.Time) {
	var start, end *time.Time
	for _, plan := range plans {
		if plan.PlannedStartAt != nil && (start == nil || plan.PlannedStartAt.Before(*start)) {
			value := *plan.PlannedStartAt
			start = &value
		}
		if plan.PlannedEndAt != nil && (end == nil || plan.PlannedEndAt.After(*end)) {
			value := *plan.PlannedEndAt
			end = &value
		}
	}
	return start, end
}

func decodeMutationNodePlans(raw string) (map[string]workflow.NodePlan, error) {
	result := map[string]workflow.NodePlan{}
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &result) != nil || result == nil {
		return nil, workflow.ErrMutationConflict
	}
	for nodeID, plan := range result {
		if strings.TrimSpace(nodeID) == "" || plan.AssigneeUserID != nil && *plan.AssigneeUserID == 0 ||
			plan.PlannedStartAt != nil && plan.PlannedEndAt != nil && plan.PlannedStartAt.After(*plan.PlannedEndAt) {
			return nil, workflow.ErrMutationConflict
		}
	}
	return result, nil
}

func updateActiveWorkflowTokenAssignee(tx *gorm.DB, instance WorkflowInstance, nodeID string, assigneeID uint) error {
	var token WorkflowToken
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		instance.WorkspaceID, instance.ID, nodeID, "active",
	).Order("id DESC").First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	updated := tx.Model(&WorkflowToken{}).Where(
		"id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		token.ID, token.WorkspaceID, token.InstanceID, token.NodeID, "active",
	).Update("assignee_user_id", assigneeID)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return workflow.ErrMutationConflict
	}
	return nil
}

func sameMutationTime(left, right *time.Time) bool {
	return left == nil && right == nil || left != nil && right != nil && left.Equal(*right)
}
