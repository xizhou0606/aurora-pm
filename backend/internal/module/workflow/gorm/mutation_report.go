package workflowgorm

import (
	"context"
	"net/url"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyWorkflowTestReportLink(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	node, found := state.definition.Node(command.NodeID)
	if !found || !workflow.SupportsTestReports(node) || state.instance.Status != "running" ||
		!containsMutationNode(currentMutationNodeIDs(state.instance.CurrentNodeIDs), node.ID) || state.subject.PlanUnitID == 0 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	// The instance row is already locked by the caller. The active token check
	// is repeated in the same transaction so a stale workflow page cannot add a
	// report after the node has advanced.
	if err := requireActiveMutationToken(tx, state.instance, node.ID, command.PrincipalID); err != nil {
		return workflow.MutationResult{}, err
	}
	report, err := dependencies.Reports.CreateWorkflowTestReportLink(
		ctx, command.WorkspaceID, state.subject, command.PrincipalID, command.ApplyKey,
		workflowTestReportLinkName(command.URL), command.URL,
	)
	if err != nil {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, NodeID: node.ID, Action: string(command.Operation), Status: state.instance.Status, ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return workflow.MutationResult{
		ID: report.ID, WorkspaceID: report.WorkspaceID, InstanceID: state.instance.ID,
		NodeID: node.ID, TestReport: &report,
	}, nil
}

func requireActiveMutationToken(tx *gorm.DB, instance WorkflowInstance, nodeID string, principalID uint) error {
	if tx == nil {
		return workflow.ErrMutationUnavailable
	}
	var token WorkflowToken
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		instance.WorkspaceID, instance.ID, nodeID, "active",
	).Order("id DESC").First(&token).Error; err != nil {
		return normalizeWorkflowMutationError(err)
	}
	if token.AssigneeUserID != nil && *token.AssigneeUserID != principalID {
		return workflow.ErrMutationConflict
	}
	return nil
}

func workflowTestReportLinkName(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return "测试报告链接"
	}
	name := parsed.Host
	if path := strings.Trim(parsed.Path, "/"); path != "" {
		name += "/" + path
	}
	runes := []rune(name)
	if len(runes) > 220 {
		return string(runes[:200]) + "…"
	}
	return name
}
