package biscreen

import (
	"context"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

type BIScreenBackend struct {
	dependencies workbench.BIScreenReadDependencies
}

func NewBIScreenBackend(dependencies workbench.BIScreenReadDependencies) (*BIScreenBackend, error) {
	if err := dependencies.Validate(); err != nil {
		return nil, err
	}
	return &BIScreenBackend{dependencies: dependencies}, nil
}

func (backend *BIScreenBackend) BuildBIScreen(ctx context.Context, query workbench.BIScreenQuery) (workbench.BIScreenResult, error) {
	workspace, err := backend.dependencies.Identities.GetBIWorkspace(ctx, query.WorkspaceID)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	if query.WorkspaceName == "" {
		query.WorkspaceName = workspace.Name
	}
	projectContext, err := backend.dependencies.Projects.LoadBIContext(ctx, query.WorkspaceID)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	requirements, err := backend.dependencies.Requirements.ListBIFacts(ctx, query.WorkspaceID)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	taskFacts, taskActivities, err := backend.dependencies.Tasks.ListBIFacts(ctx, query.WorkspaceID)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	bugFacts, bugActivities, err := backend.dependencies.Quality.ListBIFacts(ctx, query.WorkspaceID)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	scope, err := resolveBIScope(query, projectContext, requirements)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	scopedRequirements := filterBIRequirements(requirements, scope)
	contextFacts := legacyBIScreenContext(projectContext, requirements)
	tasks := filterBITasks(taskFacts, scope)
	bugs := filterBIBugs(bugFacts, scope)
	users, err := backend.dependencies.Identities.FindActiveBIUsersByFactIDs(ctx, biContextUserIDs(contextFacts, tasks, bugs))
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	for id, user := range users {
		contextFacts.Users[id] = user
	}
	agg := biAggregation{Tasks: tasks, Bugs: bugs, Ctx: contextFacts, Now: query.Now, Location: query.Location, Days: 14}
	dimension, dimensionKey, err := normalizeBITeamDimension(query.TeamDimension, query.TeamDimensionKey)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	profileSnapshot, err := backend.dependencies.Configuration.GetBITeamScoreProfile(ctx, query.WorkspaceID)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	metricSettings, err := backend.loadMetricSettings(ctx, query.WorkspaceID)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	decisions, err := backend.loadReviewDecisions(ctx, query.WorkspaceID, scopedRequirements, metricSettings)
	if err != nil {
		return workbench.BIScreenResult{}, err
	}
	teamData := buildBITeamData(scope, agg, dimension, dimensionKey, legacyBIScoreProfile(profileSnapshot),
		scopedRequirements, taskActivities, bugActivities, decisions)
	teamSet := map[uint]struct{}{}
	for _, row := range tasks {
		teamSet[row.TeamID] = struct{}{}
	}
	for _, row := range bugs {
		teamSet[row.TeamID] = struct{}{}
	}
	overview := agg.overviewMetrics()
	return workbench.BIScreenResult{
		Period: query.Period, WorkspaceID: query.WorkspaceID, WorkspaceName: query.WorkspaceName,
		TeamCount: len(teamSet), Scope: map[string]any(biScopeDTO(scope, agg.Ctx, query.Period)), OverviewMetrics: map[string]any(overview),
		UrgencyDistribution: taskDistribution(tasks, "priority"), DueDistribution: agg.dueDistribution(),
		PriorityDistribution: taskDistribution(tasks, "priority"), TypeDistribution: taskDistribution(tasks, "kind"),
		StatusProgress: taskDistribution(tasks, "status"), CompletionTrend: agg.completionTrend(),
		VersionProgress: agg.versionProgress(), RequirementProgress: agg.requirementProgress(),
		VersionRankings: agg.versionRankings(), OwnerBoard: agg.ownerBoard(), MilestoneOptions: []any{},
		QualitySeverityDistribution: agg.qualitySeverityDistribution(), QualityStatusDistribution: bugDistribution(bugs, "status"),
		QualityTrend: agg.qualityTrend(), VersionQualityRisks: agg.versionQualityRisks(), QualityOwnerLoad: agg.qualityOwnerLoad(),
		TeamData: teamData, GeneratedAt: agg.Now, Compatibility: map[string]any(biCompatibilityFields(overview, tasks, len(scope.VisibleSets))),
	}, nil
}

func (backend *BIScreenBackend) loadMetricSettings(ctx context.Context, workspaceID uint) (biTeamMetricSettingsDTO, error) {
	settings, err := backend.dependencies.Configuration.GetBITeamMetricSettings(ctx, workspaceID)
	if err != nil {
		return biTeamMetricSettingsDTO{}, err
	}
	options, err := backend.dependencies.Workflows.ListBIMetricWorkflowOptions(ctx, workspaceID)
	if err != nil {
		return biTeamMetricSettingsDTO{}, err
	}
	dto := biTeamMetricSettingsDTO{Configured: settings.Configured, RequirementWorkflowTemplateID: settings.RequirementWorkflowTemplateID,
		ApprovalNodeID: settings.ApprovalNodeID, Options: make([]biTeamMetricWorkflowOption, 0, len(options))}
	for _, option := range options {
		converted := biTeamMetricWorkflowOption{TemplateID: option.TemplateID, TemplateName: option.TemplateName, PublishedVersion: option.PublishedVersion}
		for _, node := range option.ApprovalNodes {
			converted.ApprovalNodes = append(converted.ApprovalNodes, biTeamMetricApprovalNodeOption{NodeID: node.ID, NodeName: node.Name})
			if settings.Configured && option.TemplateID == settings.RequirementWorkflowTemplateID && node.ID == settings.ApprovalNodeID {
				dto.Valid, dto.RequirementWorkflowTemplate, dto.RequirementWorkflowVersion, dto.ApprovalNodeName = true, option.TemplateName, option.PublishedVersion, node.Name
			}
		}
		dto.Options = append(dto.Options, converted)
	}
	return dto, nil
}

func (backend *BIScreenBackend) loadReviewDecisions(ctx context.Context, workspaceID uint, requirements []requirement.Snapshot, settings biTeamMetricSettingsDTO) ([]biTeamReviewDecision, error) {
	if !settings.Configured || !settings.Valid || len(requirements) == 0 {
		return []biTeamReviewDecision{}, nil
	}
	ids := make([]uint, 0, len(requirements))
	for _, row := range requirements {
		ids = append(ids, row.ID)
	}
	rows, err := backend.dependencies.Workflows.ListBIReviewDecisions(ctx, workspaceID, settings.RequirementWorkflowTemplateID, settings.ApprovalNodeID, ids)
	if err != nil {
		return nil, err
	}
	result := make([]biTeamReviewDecision, 0, len(rows))
	for _, row := range rows {
		result = append(result, biTeamReviewDecision{ID: row.ID, RequirementID: row.RequirementID, Status: row.Status, DecidedAt: row.DecidedAt})
	}
	return result, nil
}

func biScopeMatches(projectSetID, planUnitID, requirementID uint, scope workspaceScope) bool {
	contains := func(values []uint, value uint) bool {
		if len(values) == 0 {
			return true
		}
		for _, id := range values {
			if id == value {
				return true
			}
		}
		return false
	}
	return len(scope.VisibleSets) > 0 && contains(scope.VisibleSets, projectSetID) && contains(scope.ProjectSetIDs, projectSetID) && contains(scope.ProjectIDs, planUnitID) && contains(scope.RequirementIDs, requirementID)
}

var _ workbench.BIScreenBackend = (*BIScreenBackend)(nil)
