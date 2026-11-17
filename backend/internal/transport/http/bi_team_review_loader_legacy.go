package httpapi

import requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"

// loadBITeamReviewDecisions remains only for the legacy BI settings test and
// non-screen transport compatibility. The BI screen uses workflow.BIReadService.
func (s *Server) loadBITeamReviewDecisions(
	workspaceID uint,
	requirements []requirementgorm.Requirement,
	settings biTeamMetricSettingsDTO,
) ([]biTeamReviewDecision, error) {
	if !settings.Configured || settings.RequirementWorkflowTemplateID == 0 ||
		settings.ApprovalNodeID == "" || len(requirements) == 0 {
		return []biTeamReviewDecision{}, nil
	}
	ids := make([]uint, 0, len(requirements))
	for _, requirement := range requirements {
		ids = append(ids, requirement.ID)
	}
	var rows []biTeamReviewDecision
	err := s.db.Table("workflow_approvals decisions").
		Select(`decisions.id, instances.subject_id AS requirement_id,
			decisions.status, decisions.decided_at`).
		Joins(`JOIN workflow_node_runs runs
			ON runs.id = decisions.node_run_id
			AND runs.workspace_id = decisions.workspace_id`).
		Joins(`JOIN workflow_instances instances
			ON instances.id = decisions.instance_id
			AND instances.workspace_id = decisions.workspace_id`).
		Where(`decisions.workspace_id = ?
			AND instances.subject_type = ?
			AND instances.template_id = ?
			AND instances.subject_id IN ?
			AND runs.node_id = ?
			AND runs.node_type = ?
			AND decisions.status IN ?
			AND decisions.decided_at IS NOT NULL`,
			workspaceID, "requirement", settings.RequirementWorkflowTemplateID, ids,
			settings.ApprovalNodeID, "approval", []string{"completed", "rejected"}).
		Order("decisions.decided_at ASC, decisions.id ASC").Scan(&rows).Error
	return rows, err
}
