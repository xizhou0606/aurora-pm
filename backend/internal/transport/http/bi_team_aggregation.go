package httpapi

import (
	"time"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

type biTeamAggregation struct {
	requirements    []requirement.Snapshot
	tasks           []task.Snapshot
	bugs            []quality.BugSnapshot
	taskActivity    []task.ActivitySnapshot
	bugActivity     []quality.BugActivitySnapshot
	ctx             biWorkspaceContext
	dimension       string
	dimensionKey    string
	options         []biTeamDimensionOption
	scoreProfile    biTeamScoreProfileDTO
	rawRequirements []requirement.Snapshot
	rawTasks        []task.Snapshot
	rawBugs         []quality.BugSnapshot
	reviewDecisions []biTeamReviewDecision
	location        *time.Location
	now             time.Time
}

func (a biTeamAggregation) data() biTeamData {
	if len(a.scoreProfile.Items) == 0 {
		a.scoreProfile = defaultBITeamScoreProfile()
	}
	roles := []*biTeamRoleAccumulator{
		{key: "product", label: "产品", total: newBITeamAccumulator(), members: map[uint]*biTeamAccumulator{}},
		{key: "development", label: "研发", total: newBITeamAccumulator(), members: map[uint]*biTeamAccumulator{}},
		{key: "testing", label: "测试", total: newBITeamAccumulator(), members: map[uint]*biTeamAccumulator{}},
	}
	roleByKey := map[string]*biTeamRoleAccumulator{}
	for _, role := range roles {
		roleByKey[role.key] = role
	}
	taskActivity := biTeamTaskActivityFacts(a.tasks, a.taskActivity)
	bugReturnCounts := biTeamBugReturnCounts(a.bugActivity)
	bugResponseFacts := biTeamBugResponseFacts(a.bugs, a.bugActivity, time.Now().UTC())
	tasksByRequirement := map[uint][]task.Snapshot{}
	bugsByRequirement := map[uint][]quality.BugSnapshot{}
	for _, task := range a.tasks {
		tasksByRequirement[task.RequirementID] = append(tasksByRequirement[task.RequirementID], task)
	}
	for _, bug := range a.bugs {
		bugsByRequirement[bug.RequirementID] = append(bugsByRequirement[bug.RequirementID], bug)
	}

	product := roleByKey["product"]
	for _, requirement := range a.requirements {
		if len(requirement.ProductOwnerUserIDs) == 0 || requirement.ProductOwnerUserIDs[0] == 0 {
			continue
		}
		ownerID := requirement.ProductOwnerUserIDs[0]
		member := product.member(ownerID)
		for _, metric := range []*biTeamAccumulator{product.total, member} {
			metric.workload++
			if biRequirementDone(requirement.Status) {
				metric.completedWorkload++
			}
		}
		linkedTasks := tasksByRequirement[requirement.ID]
		if firstTaskAt := biFirstTaskCreatedAt(linkedTasks); firstTaskAt != nil && !firstTaskAt.Before(requirement.CreatedAt) {
			hours := firstTaskAt.Sub(requirement.CreatedAt).Hours()
			product.total.addResponse(hours)
			member.addResponse(hours)
		}
		for _, task := range linkedTasks {
			if isTaskDone(task.Status) && task.CompletedAt != nil && task.DueAt != nil {
				product.total.addOnTime(!task.CompletedAt.After(*task.DueAt))
				member.addOnTime(!task.CompletedAt.After(*task.DueAt))
			}
		}
		product.total.riskSamples++
		member.riskSamples++
		if biRequirementHasEffectiveBug(bugsByRequirement[requirement.ID]) {
			product.total.risk++
			member.risk++
		}
		requirementSubmitted := false
		requirementReworked := false
		for _, task := range linkedTasks {
			facts := taskActivity[task.ID]
			if facts.submitted {
				requirementSubmitted = true
			}
			if facts.reworkCount > 0 {
				requirementReworked = true
				product.total.reworkCount += facts.reworkCount
				member.reworkCount += facts.reworkCount
			}
		}
		if requirementSubmitted {
			product.total.submittedItems[requirement.ID] = struct{}{}
			member.submittedItems[requirement.ID] = struct{}{}
		}
		if requirementReworked {
			product.total.reworkedItems[requirement.ID] = struct{}{}
			member.reworkedItems[requirement.ID] = struct{}{}
		}
	}

	development := roleByKey["development"]
	for _, task := range a.tasks {
		if task.AssigneeID == nil || *task.AssigneeID == 0 {
			continue
		}
		ownerID := *task.AssigneeID
		member := development.member(ownerID)
		for _, metric := range []*biTeamAccumulator{development.total, member} {
			metric.workload++
			if isTaskDone(task.Status) {
				metric.completedWorkload++
				metric.riskSamples++
			}
		}
		facts := taskActivity[task.ID]
		if facts.firstInDevAt != nil && !facts.firstInDevAt.Before(task.CreatedAt) {
			hours := facts.firstInDevAt.Sub(task.CreatedAt).Hours()
			development.total.addResponse(hours)
			member.addResponse(hours)
		}
		if isTaskDone(task.Status) && task.CompletedAt != nil && task.DueAt != nil {
			development.total.addOnTime(!task.CompletedAt.After(*task.DueAt))
			member.addOnTime(!task.CompletedAt.After(*task.DueAt))
		}
		if facts.submitted {
			development.total.submittedItems[task.ID] = struct{}{}
			member.submittedItems[task.ID] = struct{}{}
		}
		if facts.reworkCount > 0 {
			development.total.reworkCount += facts.reworkCount
			member.reworkCount += facts.reworkCount
			development.total.reworkedItems[task.ID] = struct{}{}
			member.reworkedItems[task.ID] = struct{}{}
		}
	}
	for _, bug := range a.bugs {
		if !biEffectiveQualityBug(bug) || bug.AssigneeUserID == nil || *bug.AssigneeUserID == 0 {
			continue
		}
		ownerID := *bug.AssigneeUserID
		member := development.member(ownerID)
		member.riskIDs[bug.ID] = struct{}{}
		development.total.riskIDs[bug.ID] = struct{}{}
		if fact := bugResponseFacts[bug.ID]; fact.eligible {
			member.addBugResponse(fact.timely)
			development.total.addBugResponse(fact.timely)
		}
	}

	testing := roleByKey["testing"]
	for _, bug := range a.bugs {
		ownerID := biTeamBugOwner(bug)
		if ownerID == 0 {
			continue
		}
		member := testing.member(ownerID)
		terminalAt := biBugTerminalAt(bug)
		for _, metric := range []*biTeamAccumulator{testing.total, member} {
			metric.workload++
			if terminalAt != nil {
				metric.completedWorkload++
			}
		}
		responseAt := bug.ReviewedAt
		if responseAt == nil {
			responseAt = bug.CompletedAt
		}
		if bug.ResolvedAt != nil && responseAt != nil && !responseAt.Before(*bug.ResolvedAt) {
			hours := responseAt.Sub(*bug.ResolvedAt).Hours()
			testing.total.addResponse(hours)
			member.addResponse(hours)
		}
		if terminalAt != nil && bug.DueAt != nil {
			testing.total.addOnTime(!terminalAt.After(*bug.DueAt))
			member.addOnTime(!terminalAt.After(*bug.DueAt))
		}
		if biBugHasResolutionSample(bug) {
			testing.total.riskSamples++
			member.riskSamples++
			if biInvalidBugResolution(bug) {
				testing.total.risk++
				member.risk++
			}
		}
		if bug.ResolvedAt != nil || isBIQualityReview(bug.Status) || terminalAt != nil {
			testing.total.submittedItems[bug.ID] = struct{}{}
			member.submittedItems[bug.ID] = struct{}{}
		}
		if count := bugReturnCounts[bug.ID]; count > 0 {
			testing.total.reworkCount += count
			member.reworkCount += count
			testing.total.reworkedItems[bug.ID] = struct{}{}
			member.reworkedItems[bug.ID] = struct{}{}
		}
	}

	summaryRows := make([]biTeamSummaryRow, 0, len(roles))
	memberGroups := make([]biTeamMemberGroup, 0, len(roles))
	for _, role := range roles {
		summaryRows = append(summaryRows, role.summary(a.scoreProfile))
		memberGroups = append(memberGroups, role.group(a.ctx, a.scoreProfile))
	}
	result := biTeamData{
		Dimension: dimensionOrDefault(a.dimension), DimensionKey: a.dimensionKey,
		DimensionOptions: a.options, SummaryRows: summaryRows, MemberGroups: memberGroups,
		ScoreProfile: a.scoreProfile,
		MetricNotes:  biTeamMetricNotes(),
	}
	a.applyRawMetrics(&result)
	return result
}
