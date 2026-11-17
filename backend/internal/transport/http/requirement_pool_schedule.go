package httpapi

import (
	"time"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func buildRequirementPoolWorkflowOverview(
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	runs []workflowgorm.WorkflowNodeRun,
	selected map[string]bool,
	schedule workflowSubjectSchedule,
	businessDate time.Time,
) *requirementPoolWorkflowOverview {
	statuses, _ := workflowTimelineState(definition, runs)
	plans := workflowNodePlans(instance)
	currentIDs := map[string]bool{}
	for _, nodeID := range currentWorkflowNodes(instance.CurrentNodeIDs) {
		currentIDs[nodeID] = true
	}
	currentNames := make([]string, 0, len(currentIDs))
	currentNodes := make([]requirementPoolWorkflowNodeOverview, 0, len(currentIDs))
	completed, total := 0, 0
	for _, node := range definition.Nodes {
		if !selected[node.ID] || requirementPoolConnectorNode(node.Type) {
			continue
		}
		total++
		if statuses[node.ID] == "completed" {
			completed++
		}
		if currentIDs[node.ID] {
			currentNames = append(currentNames, node.Name)
			plan := plans[node.ID]
			currentNodes = append(currentNodes, requirementPoolWorkflowNodeOverview{
				ID: node.ID, Name: node.Name, PlannedEndAt: plan.PlannedEndAt,
				Overdue: requirementPoolNodeOverdue(plan, statuses[node.ID], schedule, businessDate),
			})
		}
	}
	if instance.Status == "completed" {
		completed = total
		currentNames = []string{"流程已完成"}
		currentNodes = nil
	}
	if len(currentNames) == 0 {
		for _, node := range definition.Nodes {
			if selected[node.ID] && currentIDs[node.ID] {
				currentNames = append(currentNames, node.Name)
			}
		}
	}
	return &requirementPoolWorkflowOverview{
		InstanceID: instance.ID, Status: instance.Status,
		CurrentNodeNames: currentNames, CurrentNodes: currentNodes,
		CompletedSteps: completed, TotalSteps: total,
	}
}

func requirementPoolNodeOverdue(
	plan workflowNodePlan,
	status string,
	schedule workflowSubjectSchedule,
	businessDate time.Time,
) bool {
	start, end := schedule.Subject.PlannedStartAt, schedule.Subject.PlannedEndAt
	return schedule.State != "conflict" &&
		start != nil && end != nil && !start.After(*end) &&
		status != "completed" && status != "skipped" &&
		plan.PlannedEndAt != nil && workflowDateLabel(*plan.PlannedEndAt).Before(businessDate)
}

// Workflow schedule dates are displayed as their UTC date prefix. Normalize
// the request's local business day to the same date-label representation.
func requirementPoolBusinessDate(c *gin.Context) time.Time {
	start, _ := businessDayBounds(c)
	localStart := start.In(biLocation(c))
	return time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, time.UTC)
}

func workflowDateLabel(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func requirementPoolConnectorNode(nodeType string) bool {
	return nodeType == "start" || nodeType == "end" || nodeType == "parallel_split" || nodeType == "parallel_join"
}

func loadRequirementPoolScheduleScopes(
	db *gorm.DB,
	workspaceID uint,
	requirements []requirementgorm.Requirement,
) (map[uint]workflowScheduleScope, error) {
	unitIDs := make([]uint, 0, len(requirements))
	projectIDs := make([]uint, 0, len(requirements))
	seenUnits, seenProjects := map[uint]bool{}, map[uint]bool{}
	for _, requirement := range requirements {
		if requirement.PlanUnitID > 0 && !seenUnits[requirement.PlanUnitID] {
			seenUnits[requirement.PlanUnitID] = true
			unitIDs = append(unitIDs, requirement.PlanUnitID)
		}
		if requirement.ProjectSetID > 0 && !seenProjects[requirement.ProjectSetID] {
			seenProjects[requirement.ProjectSetID] = true
			projectIDs = append(projectIDs, requirement.ProjectSetID)
		}
	}
	var units []projectgorm.PlanUnit
	if len(unitIDs) > 0 {
		if err := db.Where("workspace_id = ? AND id IN ?", workspaceID, unitIDs).Find(&units).Error; err != nil {
			return nil, err
		}
	}
	var projects []projectgorm.ProjectSet
	if len(projectIDs) > 0 {
		if err := db.Where("workspace_id = ? AND id IN ?", workspaceID, projectIDs).Find(&projects).Error; err != nil {
			return nil, err
		}
	}
	unitByID := make(map[uint]projectgorm.PlanUnit, len(units))
	for _, unit := range units {
		unitByID[unit.ID] = unit
	}
	projectByID := make(map[uint]projectgorm.ProjectSet, len(projects))
	for _, project := range projects {
		projectByID[project.ID] = project
	}
	scopes := make(map[uint]workflowScheduleScope, len(requirements))
	for _, requirement := range requirements {
		project, projectFound := projectByID[requirement.ProjectSetID]
		if !projectFound {
			continue
		}
		if requirement.PlanUnitID == 0 {
			scopes[requirement.ID] = workflowScheduleScopeForProject(project)
			continue
		}
		unit, unitFound := unitByID[requirement.PlanUnitID]
		if !unitFound || unit.ProjectSetID != requirement.ProjectSetID {
			continue
		}
		scopes[requirement.ID] = workflowScheduleScopeForPlanUnit(unit, project)
	}
	return scopes, nil
}
