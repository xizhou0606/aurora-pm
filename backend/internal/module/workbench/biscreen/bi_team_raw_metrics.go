package biscreen

import (
	"sort"
	"strings"
	"time"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	taskmodule "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

type biTeamReviewDecision struct {
	ID            uint
	RequirementID uint
	Status        string
	DecidedAt     time.Time
}

type biTeamRawAccumulator struct {
	requirementCount   int
	reviewPass         int
	reviewSamples      int
	firstPass          int
	firstPassSamples   int
	bugCount           int
	bugCloseHours      float64
	bugCloseSamples    int
	invalidBugs        int
	invalidBugSamples  int
	taskCount          int
	overdue            int
	overdueSamples     int
	bugTaskBugs        int
	p0BugCount         int
	p1BugCount         int
	bugResponseHours   float64
	bugResponseSamples int
}

type biTeamRawRole struct {
	total   *biTeamRawAccumulator
	members map[uint]*biTeamRawAccumulator
}

func newBITeamRawRole() *biTeamRawRole {
	return &biTeamRawRole{total: &biTeamRawAccumulator{}, members: map[uint]*biTeamRawAccumulator{}}
}

func (r *biTeamRawRole) member(userID uint) *biTeamRawAccumulator {
	if r.members[userID] == nil {
		r.members[userID] = &biTeamRawAccumulator{}
	}
	return r.members[userID]
}

func (a biTeamAggregation) applyRawMetrics(data *biTeamData) {
	if a.rawRequirements == nil {
		a.rawRequirements = a.requirements
	}
	if a.rawTasks == nil {
		a.rawTasks = a.tasks
	}
	if a.rawBugs == nil {
		a.rawBugs = a.bugs
	}
	if a.location == nil {
		a.location = time.UTC
	}
	if a.now.IsZero() {
		a.now = time.Now().UTC()
	}
	roles := map[string]*biTeamRawRole{
		"product": newBITeamRawRole(), "testing": newBITeamRawRole(), "development": newBITeamRawRole(),
	}
	requirementByID := make(map[uint]requirement.Snapshot, len(a.rawRequirements))
	for _, requirement := range a.rawRequirements {
		requirementByID[requirement.ID] = requirement
		ownerID := biRequirementOwnerID(requirement)
		if ownerID == 0 || !a.matchesEvent(biRequirementPlanUnitID(requirement), requirement.CreatedAt) {
			continue
		}
		addRawMetric(roles["product"], ownerID, func(metric *biTeamRawAccumulator) {
			metric.requirementCount++
		})
	}
	a.applyProductReviewMetrics(roles["product"], requirementByID)
	a.applyTestingRawMetrics(roles["testing"])
	a.applyDevelopmentRawMetrics(roles["development"])
	for index := range data.SummaryRows {
		role := roles[data.SummaryRows[index].RoleKey]
		if role != nil {
			applyRawSummary(&data.SummaryRows[index], role.total, role.total.p0BugCount, role.total.p1BugCount, data.SummaryRows[index].RoleKey)
		}
	}
	for groupIndex := range data.MemberGroups {
		group := &data.MemberGroups[groupIndex]
		role := roles[group.RoleKey]
		if role == nil {
			continue
		}
		existing := make(map[uint]int, len(group.Members))
		for index := range group.Members {
			existing[group.Members[index].UserID] = index
		}
		missingUserIDs := make([]uint, 0)
		for userID := range role.members {
			if _, found := existing[userID]; found {
				continue
			}
			missingUserIDs = append(missingUserIDs, userID)
		}
		sort.Slice(missingUserIDs, func(i, j int) bool { return missingUserIDs[i] < missingUserIDs[j] })
		for _, userID := range missingUserIDs {
			name := biUserDisplayName(a.ctx, &userID)
			if name == "" {
				name = retiredUserDisplayName(userID)
			}
			group.Members = append(group.Members, biTeamMember{
				UserID: userID, UserName: name, UserAvatarURL: biUserAvatarURL(a.ctx, &userID),
			})
			existing[userID] = len(group.Members) - 1
		}
		for userID, metric := range role.members {
			applyRawMember(&group.Members[existing[userID]], metric, role.total.p0BugCount, role.total.p1BugCount, group.RoleKey)
		}
		for summaryIndex := range data.SummaryRows {
			if data.SummaryRows[summaryIndex].RoleKey == group.RoleKey {
				data.SummaryRows[summaryIndex].MemberCount = len(group.Members)
				break
			}
		}
	}
}

func (a biTeamAggregation) applyProductReviewMetrics(
	role *biTeamRawRole,
	requirements map[uint]requirement.Snapshot,
) {
	byRequirement := make(map[uint][]biTeamReviewDecision)
	for _, decision := range a.reviewDecisions {
		byRequirement[decision.RequirementID] = append(byRequirement[decision.RequirementID], decision)
		requirement, found := requirements[decision.RequirementID]
		ownerID := biRequirementOwnerID(requirement)
		if !found || ownerID == 0 || !a.matchesEvent(biRequirementPlanUnitID(requirement), decision.DecidedAt) {
			continue
		}
		approved := normalizeStatus(decision.Status) == "completed"
		addRawMetric(role, ownerID, func(metric *biTeamRawAccumulator) {
			metric.reviewSamples++
			if approved {
				metric.reviewPass++
			}
		})
	}
	for requirementID, decisions := range byRequirement {
		requirement, found := requirements[requirementID]
		ownerID := biRequirementOwnerID(requirement)
		if !found || ownerID == 0 || len(decisions) == 0 {
			continue
		}
		sort.SliceStable(decisions, func(i, j int) bool {
			if decisions[i].DecidedAt.Equal(decisions[j].DecidedAt) {
				return decisions[i].ID < decisions[j].ID
			}
			return decisions[i].DecidedAt.Before(decisions[j].DecidedAt)
		})
		latest := decisions[len(decisions)-1]
		if normalizeStatus(latest.Status) != "completed" || !a.matchesEvent(biRequirementPlanUnitID(requirement), latest.DecidedAt) {
			continue
		}
		firstPass := true
		for _, decision := range decisions {
			if normalizeStatus(decision.Status) == "rejected" {
				firstPass = false
				break
			}
		}
		addRawMetric(role, ownerID, func(metric *biTeamRawAccumulator) {
			metric.firstPassSamples++
			if firstPass {
				metric.firstPass++
			}
		})
	}
}

func (a biTeamAggregation) applyTestingRawMetrics(role *biTeamRawRole) {
	activities := qualityActivitiesByBug(a.bugActivity)
	for _, bug := range a.rawBugs {
		if bug.ReporterUserID != nil && *bug.ReporterUserID > 0 && biEffectiveQualityBug(bug) &&
			a.matchesEvent(bug.PlanUnitID, bug.CreatedAt) {
			addRawMetric(role, *bug.ReporterUserID, func(metric *biTeamRawAccumulator) { metric.bugCount++ })
		}
		if bug.ReviewerUserID != nil && *bug.ReviewerUserID > 0 {
			if resolvedAt, closedAt, found := biTeamBugCloseInterval(bug, activities[bug.ID]); found &&
				a.matchesEvent(bug.PlanUnitID, closedAt) {
				hours := closedAt.Sub(resolvedAt).Hours()
				addRawMetric(role, *bug.ReviewerUserID, func(metric *biTeamRawAccumulator) {
					metric.bugCloseHours += hours
					metric.bugCloseSamples++
				})
			}
		}
		if bug.ReporterUserID == nil || *bug.ReporterUserID == 0 || !biBugHasResolutionSample(bug) ||
			normalizeQualityBugStatus(bug.Status) == "canceled" {
			continue
		}
		decisionAt := biTeamBugResolutionDecisionAt(bug, activities[bug.ID])
		if decisionAt == nil || !a.matchesEvent(bug.PlanUnitID, *decisionAt) {
			continue
		}
		addRawMetric(role, *bug.ReporterUserID, func(metric *biTeamRawAccumulator) {
			metric.invalidBugSamples++
			if biInvalidBugResolution(bug) {
				metric.invalidBugs++
			}
		})
	}
}

func (a biTeamAggregation) applyDevelopmentRawMetrics(role *biTeamRawRole) {
	taskFacts := biTeamTaskActivityFacts(a.rawTasks, a.taskActivity)
	dueFacts := biTeamTaskDueFacts(a.rawTasks)
	for _, task := range a.rawTasks {
		if task.AssigneeID == nil || *task.AssigneeID == 0 {
			continue
		}
		ownerID := *task.AssigneeID
		if a.matchesEvent(task.PlanUnitID, task.CreatedAt) {
			addRawMetric(role, ownerID, func(metric *biTeamRawAccumulator) { metric.taskCount++ })
		}
		if taskmodule.IsCancelledStatus(task.Status) {
			continue
		}
		dueAt := dueFacts[task.ID].dueAt
		if dueAt != nil && a.matchesEvent(task.PlanUnitID, *dueAt) {
			deliveryAt := taskFacts[task.ID].deliveryAt
			overdue := biTeamTaskOverdue(dueAt, deliveryAt, a.now)
			addRawMetric(role, ownerID, func(metric *biTeamRawAccumulator) {
				metric.overdueSamples++
				if overdue {
					metric.overdue++
				}
			})
		}
	}
	responseFacts := biTeamBugResponseDurations(a.rawBugs, a.bugActivity)
	for _, bug := range a.rawBugs {
		if !biEffectiveQualityBug(bug) {
			continue
		}
		inCreationScope := a.matchesEvent(bug.PlanUnitID, bug.CreatedAt)
		isP0 := strings.EqualFold(strings.TrimSpace(bug.SeverityKey), "p0")
		isP1 := strings.EqualFold(strings.TrimSpace(bug.SeverityKey), "p1")
		if inCreationScope && isP0 {
			role.total.p0BugCount++
		}
		if inCreationScope && isP1 {
			role.total.p1BugCount++
		}
		if bug.AssigneeUserID == nil || *bug.AssigneeUserID == 0 {
			continue
		}
		ownerID := *bug.AssigneeUserID
		if inCreationScope {
			addRawMetric(role, ownerID, func(metric *biTeamRawAccumulator) {
				metric.bugTaskBugs++
			})
			if isP0 {
				role.member(ownerID).p0BugCount++
			}
			if isP1 {
				role.member(ownerID).p1BugCount++
			}
		}
		if fact, found := responseFacts[bug.ID]; found && a.matchesEvent(bug.PlanUnitID, fact.respondedAt) {
			addRawMetric(role, ownerID, func(metric *biTeamRawAccumulator) {
				metric.bugResponseHours += fact.hours
				metric.bugResponseSamples++
			})
		}
	}
}

func addRawMetric(role *biTeamRawRole, userID uint, add func(*biTeamRawAccumulator)) {
	add(role.total)
	add(role.member(userID))
}

func (a biTeamAggregation) matchesEvent(projectID uint, occurredAt time.Time) bool {
	return biTeamRecordMatches(projectID, occurredAt, a.dimension, a.dimensionKey, a.location)
}
