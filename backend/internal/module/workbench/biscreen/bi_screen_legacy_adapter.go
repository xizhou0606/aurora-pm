package biscreen

// This file is the temporary shape adapter between protocol-neutral BI read
// contracts and the established pure aggregation functions. Database access is
// forbidden here; all facts arrive through owning-domain read services.

import (
	"fmt"
	"sort"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

func resolveBIScope(query workbench.BIScreenQuery, projects project.BIContext, requirements []requirement.Snapshot) (workspaceScope, error) {
	scope := workspaceScope{ProjectSetIDs: query.ProjectSetIDs, ProjectIDs: query.PlanUnitIDs, RequirementIDs: query.RequirementIDs}
	sets := make(map[uint]project.ProjectSetSnapshot, len(projects.ProjectSets))
	units := make(map[uint]project.PlanUnitSnapshot, len(projects.PlanUnits))
	requirementByID := make(map[uint]requirement.Snapshot, len(requirements))
	// The caller's accessible project sets bound the whole aggregation: a
	// workspace member without access to a project must not see it counted,
	// searched, or offered as a scope option. nil keeps the legacy behavior
	// (no restriction resolved — the admin fast path).
	accessible := map[uint]struct{}{}
	accessLimited := query.AccessibleProjectSetIDs != nil
	for _, id := range query.AccessibleProjectSetIDs {
		accessible[id] = struct{}{}
	}
	for _, row := range projects.ProjectSets {
		sets[row.ID] = row
		if accessLimited {
			if _, ok := accessible[row.ID]; ok {
				scope.VisibleSets = append(scope.VisibleSets, row.ID)
			}
		} else {
			scope.VisibleSets = append(scope.VisibleSets, row.ID)
		}
	}
	sort.Slice(scope.VisibleSets, func(i, j int) bool { return scope.VisibleSets[i] < scope.VisibleSets[j] })
	for _, id := range scope.ProjectSetIDs {
		if _, ok := sets[id]; !ok {
			return scope, fmt.Errorf("project_set_ids contains a project set outside the workspace")
		}
		if accessLimited {
			if _, ok := accessible[id]; !ok {
				return scope, fmt.Errorf("project_set_ids contains a project set outside the workspace")
			}
		}
	}
	for _, row := range projects.PlanUnits {
		units[row.ID] = row
	}
	for _, id := range scope.ProjectIDs {
		row, ok := units[id]
		if !ok {
			return scope, fmt.Errorf("project_ids contains a missing project")
		}
		if _, ok := sets[row.ProjectSetID]; !ok {
			return scope, fmt.Errorf("project_ids contains a project outside the workspace")
		}
		if accessLimited {
			if _, ok := accessible[row.ProjectSetID]; !ok {
				return scope, fmt.Errorf("project_ids contains a project outside the workspace")
			}
		}
	}
	for _, row := range requirements {
		requirementByID[row.ID] = row
	}
	for _, id := range scope.RequirementIDs {
		row, ok := requirementByID[id]
		if !ok {
			return scope, fmt.Errorf("requirement_ids contains a missing requirement")
		}
		if _, ok := sets[row.ProjectSetID]; !ok {
			return scope, fmt.Errorf("requirement_ids contains a requirement outside the workspace")
		}
		if accessLimited {
			if _, ok := accessible[row.ProjectSetID]; !ok {
				return scope, fmt.Errorf("requirement_ids contains a requirement outside the workspace")
			}
		}
	}
	return scope, nil
}

func legacyBIScreenContext(projects project.BIContext, requirements []requirement.Snapshot) biWorkspaceContext {
	result := biWorkspaceContext{ProjectSets: map[uint]project.ProjectSetSnapshot{}, Projects: map[uint]project.PlanUnitSnapshot{}, Requirements: map[uint]requirement.Snapshot{}, Users: map[uint]identity.BIUser{}}
	for _, row := range projects.ProjectSets {
		result.ProjectSets[row.ID] = row
	}
	for _, row := range projects.PlanUnits {
		result.Projects[row.ID] = row
	}
	for _, row := range requirements {
		result.Requirements[row.ID] = row
	}
	return result
}

func filterBIRequirements(rows []requirement.Snapshot, scope workspaceScope) []requirement.Snapshot {
	result := make([]requirement.Snapshot, 0, len(rows))
	for _, row := range rows {
		planUnitID := uint(0)
		if row.PlanUnitID != nil {
			planUnitID = *row.PlanUnitID
		}
		if biScopeMatches(row.ProjectSetID, planUnitID, row.ID, scope) {
			result = append(result, row)
		}
	}
	return result
}

func buildBITeamData(scope workspaceScope, base biAggregation, dimension, key string, profile biTeamScoreProfileDTO,
	requirements []requirement.Snapshot, taskActivity []task.ActivitySnapshot, bugActivity []quality.BugActivitySnapshot,
	decisions []biTeamReviewDecision) biTeamData {
	rawRequirements, rawTasks, rawBugs := requirements, base.Tasks, base.Bugs
	if dimension == biTeamDimensionVersion {
		rawRequirements = biFilterTeamRequirements(requirements, dimension, key, base.Location)
		rawTasks = biFilterTeamTasks(base.Tasks, dimension, key, base.Location)
		rawBugs = biFilterTeamBugs(base.Bugs, dimension, key, base.Location)
	}
	filteredRequirements := biFilterTeamRequirements(requirements, dimension, key, base.Location)
	filteredTasks := biFilterTeamTasks(base.Tasks, dimension, key, base.Location)
	filteredBugs := biFilterTeamBugs(base.Bugs, dimension, key, base.Location)
	options := biTeamDimensionOptions(dimension, scope, base.Ctx, requirements, base.Tasks, base.Bugs, base.Location)
	if dimension == biTeamDimensionMonth || dimension == biTeamDimensionWeek {
		options = biTeamEventPeriodOptions(options, dimension, rawTasks, rawBugs, bugActivity, decisions, base.Location)
	}
	return biTeamAggregation{requirements: filteredRequirements, tasks: filteredTasks, bugs: filteredBugs,
		taskActivity: taskActivity, bugActivity: bugActivity, ctx: base.Ctx, dimension: dimension, dimensionKey: key,
		options: options, scoreProfile: profile, rawRequirements: rawRequirements, rawTasks: rawTasks, rawBugs: rawBugs,
		reviewDecisions: decisions, location: base.Location, now: base.Now}.data()
}

func biContextUserIDs(contextFacts biWorkspaceContext, tasks []task.Snapshot, bugs []quality.BugSnapshot) []uint {
	ids := biUserFactIDs(tasks, bugs)
	for _, row := range contextFacts.ProjectSets {
		if row.OwnerUserID != nil {
			ids = append(ids, *row.OwnerUserID)
		}
	}
	for _, row := range contextFacts.Projects {
		if row.OwnerUserID != nil {
			ids = append(ids, *row.OwnerUserID)
		}
	}
	for _, row := range contextFacts.Requirements {
		ids = append(ids, row.ProductOwnerUserIDs...)
		ids = append(ids, row.DesignOwnerUserIDs...)
		ids = append(ids, row.FrontendOwnerUserIDs...)
		ids = append(ids, row.BackendOwnerUserIDs...)
		ids = append(ids, row.TesterUserIDs...)
		ids = append(ids, row.DeveloperUserIDs...)
	}
	return mergeSortedUserIDs(ids)
}

func filterBITasks(rows []task.Snapshot, scope workspaceScope) []task.Snapshot {
	result := []task.Snapshot{}
	for _, row := range rows {
		if biScopeMatches(row.ProjectSetID, row.PlanUnitID, row.RequirementID, scope) {
			result = append(result, row)
		}
	}
	return result
}

func filterBIBugs(rows []quality.BugSnapshot, scope workspaceScope) []quality.BugSnapshot {
	result := []quality.BugSnapshot{}
	for _, row := range rows {
		if biScopeMatches(row.ProjectSetID, row.PlanUnitID, row.RequirementID, scope) {
			result = append(result, row)
		}
	}
	return result
}

func legacyBIScoreProfile(snapshot workbench.BITeamScoreProfileSnapshot) biTeamScoreProfileDTO {
	if snapshot.ID == 0 {
		return defaultBITeamScoreProfile()
	}
	return mergeBITeamScoreProfile(snapshot)
}
