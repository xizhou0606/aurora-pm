package httpapi

import (
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type biTeamAccumulator struct {
	workload           int
	completedWorkload  int
	responseHours      float64
	responseSamples    int
	onTime             int
	onTimeSamples      int
	risk               int
	riskSamples        int
	riskIDs            map[uint]struct{}
	reworkCount        int
	submittedItems     map[uint]struct{}
	reworkedItems      map[uint]struct{}
	bugResponseTimely  int
	bugResponseSamples int
}

type biTeamRoleAccumulator struct {
	key     string
	label   string
	total   *biTeamAccumulator
	members map[uint]*biTeamAccumulator
}

type biTeamTaskFacts struct {
	firstInDevAt *time.Time
	deliveryAt   *time.Time
	submitted    bool
	reworkCount  int
}

type biTeamBugResponseFact struct {
	eligible bool
	timely   bool
}

func biTeamTaskActivityFacts(
	tasks []task.Snapshot, activities []task.ActivitySnapshot,
) map[uint]biTeamTaskFacts {
	result := map[uint]biTeamTaskFacts{}
	for _, task := range tasks {
		status := normalizeStatus(task.Status)
		result[task.ID] = biTeamTaskFacts{
			submitted: status == "pending_test" || isTaskDone(status),
		}
	}
	orderedActivities := append([]task.ActivitySnapshot(nil), activities...)
	sort.SliceStable(orderedActivities, func(i, j int) bool {
		if orderedActivities[i].CreatedAt.Equal(orderedActivities[j].CreatedAt) {
			return orderedActivities[i].ID < orderedActivities[j].ID
		}
		return orderedActivities[i].CreatedAt.Before(orderedActivities[j].CreatedAt)
	})
	for _, activity := range orderedActivities {
		facts, ok := result[activity.WorkTaskID]
		if !ok || normalizeStatus(activity.FieldKey) != "status" {
			continue
		}
		oldStatus := normalizeStatus(activity.OldValue)
		newStatus := normalizeStatus(activity.NewValue)
		if newStatus == "in_dev" && facts.firstInDevAt == nil {
			createdAt := activity.CreatedAt
			facts.firstInDevAt = &createdAt
		}
		if newStatus == "pending_test" {
			facts.submitted = true
			deliveredAt := activity.CreatedAt
			facts.deliveryAt = &deliveredAt
		} else if isTaskDone(newStatus) {
			if facts.deliveryAt == nil {
				deliveredAt := activity.CreatedAt
				facts.deliveryAt = &deliveredAt
			}
		} else if newStatus != "" {
			facts.deliveryAt = nil
		}
		if oldStatus == "pending_test" && newStatus == "in_dev" {
			facts.reworkCount++
		}
		result[activity.WorkTaskID] = facts
	}
	for _, task := range tasks {
		facts := result[task.ID]
		status := normalizeStatus(task.Status)
		switch {
		case status == "pending_test":
			if facts.deliveryAt == nil && !task.CreatedAt.IsZero() {
				deliveredAt := task.CreatedAt
				facts.deliveryAt = &deliveredAt
			}
		case isTaskDone(status):
			if facts.deliveryAt == nil && task.CompletedAt != nil {
				deliveredAt := *task.CompletedAt
				facts.deliveryAt = &deliveredAt
			}
		default:
			facts.deliveryAt = nil
		}
		result[task.ID] = facts
	}
	return result
}

func biTeamBugReturnCounts(activities []quality.BugActivitySnapshot) map[uint]int {
	result := map[uint]int{}
	for _, activity := range activities {
		if normalizeStatus(activity.Action) == "returned" && normalizeStatus(activity.FieldKey) == "status" &&
			normalizeStatus(activity.OldValue) == "review" && normalizeStatus(activity.NewValue) == "in_progress" {
			result[activity.QualityBugID]++
		}
	}
	return result
}

func biTeamBugResponseFacts(
	bugs []quality.BugSnapshot,
	activities []quality.BugActivitySnapshot,
	now time.Time,
) map[uint]biTeamBugResponseFact {
	byBug := make(map[uint][]quality.BugActivitySnapshot)
	for _, activity := range activities {
		byBug[activity.QualityBugID] = append(byBug[activity.QualityBugID], activity)
	}
	result := make(map[uint]biTeamBugResponseFact)
	for _, bug := range bugs {
		if bug.AssigneeUserID == nil || *bug.AssigneeUserID == 0 || bug.CreatedAt.IsZero() {
			continue
		}
		ownerText := strconv.FormatUint(uint64(*bug.AssigneeUserID), 10)
		assignedAt := bug.CreatedAt
		var respondedAt *time.Time
		if bug.ResolvedAt != nil && !bug.ResolvedAt.Before(assignedAt) {
			at := *bug.ResolvedAt
			respondedAt = &at
		}
		rows := append([]quality.BugActivitySnapshot(nil), byBug[bug.ID]...)
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt.Before(rows[j].CreatedAt) })
		for _, activity := range rows {
			field := normalizeStatus(activity.FieldKey)
			if field == "assignee_user_id" && strings.TrimSpace(activity.NewValue) == ownerText {
				assignedAt = activity.CreatedAt
				respondedAt = nil
				continue
			}
			status := normalizeQualityBugStatus(activity.NewValue)
			if field == "status" && (status == "in_progress" || status == "review") &&
				!activity.CreatedAt.Before(assignedAt) && (respondedAt == nil || activity.CreatedAt.Before(*respondedAt)) {
				at := activity.CreatedAt
				respondedAt = &at
			}
		}
		deadline := assignedAt.Add(24 * time.Hour)
		if respondedAt != nil {
			result[bug.ID] = biTeamBugResponseFact{eligible: true, timely: !respondedAt.After(deadline)}
			continue
		}
		status := normalizeQualityBugStatus(bug.Status)
		waiting := status == "triage" || status == "todo"
		if waiting && !now.Before(deadline) {
			result[bug.ID] = biTeamBugResponseFact{eligible: true}
		}
	}
	return result
}

func newBITeamAccumulator() *biTeamAccumulator {
	return &biTeamAccumulator{
		riskIDs: map[uint]struct{}{}, submittedItems: map[uint]struct{}{}, reworkedItems: map[uint]struct{}{},
	}
}

func (r *biTeamRoleAccumulator) member(userID uint) *biTeamAccumulator {
	if r.members[userID] == nil {
		r.members[userID] = newBITeamAccumulator()
	}
	return r.members[userID]
}

func (a *biTeamAccumulator) addResponse(hours float64) {
	if hours < 0 {
		return
	}
	a.responseHours += hours
	a.responseSamples++
}

func (a *biTeamAccumulator) addOnTime(onTime bool) {
	a.onTimeSamples++
	if onTime {
		a.onTime++
	}
}

func (a *biTeamAccumulator) addBugResponse(timely bool) {
	a.bugResponseSamples++
	if timely {
		a.bugResponseTimely++
	}
}

func (r *biTeamRoleAccumulator) summary(profile biTeamScoreProfileDTO) biTeamSummaryRow {
	metrics := r.total
	result := biTeamSummaryRow{
		RoleKey: r.key, RoleLabel: r.label, MemberCount: len(r.members),
		Workload: metrics.workload, CompletedWorkload: metrics.completedWorkload,
		AvgResponseHours: metrics.averageResponse(), ResponseSampleSize: metrics.responseSamples,
		OnTimeRatePct: metrics.onTimeRate(), OnTimeSampleSize: metrics.onTimeSamples,
		QualityRiskRatePct: metrics.riskRate(), QualityRiskSampleSize: metrics.riskSamples,
		ReworkCount: metrics.reworkCount, ReworkRatePct: metrics.reworkRate(),
		ReworkSampleSize: len(metrics.submittedItems), FirstPassRatePct: metrics.firstPassRate(),
	}
	applyBITeamScoreToSummary(&result, r.key, metrics, profile, metrics.workload, len(r.members))
	return result
}

func (r *biTeamRoleAccumulator) group(
	ctx biWorkspaceContext,
	profile biTeamScoreProfileDTO,
) biTeamMemberGroup {
	members := make([]biTeamMember, 0, len(r.members))
	for userID, metrics := range r.members {
		name := biUserDisplayName(ctx, &userID)
		if name == "" {
			name = retiredUserDisplayName(userID)
		}
		result := biTeamMember{
			UserID: userID, UserName: name, UserAvatarURL: biUserAvatarURL(ctx, &userID),
			Workload: metrics.workload, CompletedWorkload: metrics.completedWorkload,
			AvgResponseHours: metrics.averageResponse(), ResponseSampleSize: metrics.responseSamples,
			OnTimeRatePct: metrics.onTimeRate(), OnTimeSampleSize: metrics.onTimeSamples,
			QualityRiskRatePct: metrics.riskRate(), QualityRiskSampleSize: metrics.riskSamples,
			ReworkCount:   metrics.reworkCount,
			ReworkRatePct: metrics.reworkRate(), FirstPassRatePct: metrics.firstPassRate(),
			ReworkSampleSize: len(metrics.submittedItems),
		}
		applyBITeamScoreToMember(&result, r.key, metrics, profile, r.total.workload, len(r.members))
		members = append(members, result)
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].CompositeScore != nil && members[j].CompositeScore != nil &&
			*members[i].CompositeScore != *members[j].CompositeScore {
			return *members[i].CompositeScore > *members[j].CompositeScore
		}
		if members[i].CompositeScore != nil || members[j].CompositeScore != nil {
			return members[i].CompositeScore != nil
		}
		if members[i].Workload != members[j].Workload {
			return members[i].Workload > members[j].Workload
		}
		if members[i].CompletedWorkload != members[j].CompletedWorkload {
			return members[i].CompletedWorkload > members[j].CompletedWorkload
		}
		return members[i].UserID < members[j].UserID
	})
	return biTeamMemberGroup{RoleKey: r.key, RoleLabel: r.label, Members: members}
}

func (a *biTeamAccumulator) averageResponse() float64 {
	if a.responseSamples == 0 {
		return 0
	}
	return biTeamRound(float64(a.responseHours) / float64(a.responseSamples))
}

func (a *biTeamAccumulator) onTimeRate() float64 {
	return biTeamPercent(a.onTime, a.onTimeSamples)
}

func (a *biTeamAccumulator) riskRate() float64 {
	risk := a.risk
	if len(a.riskIDs) > 0 {
		risk = len(a.riskIDs)
	}
	return biTeamPercent(risk, a.riskSamples)
}

func (a *biTeamAccumulator) reworkRate() float64 {
	return biTeamPercent(len(a.reworkedItems), len(a.submittedItems))
}

func (a *biTeamAccumulator) bugResponseRate() float64 {
	return biTeamPercent(a.bugResponseTimely, a.bugResponseSamples)
}

func (a *biTeamAccumulator) firstPassRate() float64 {
	if len(a.submittedItems) == 0 {
		return 0
	}
	return biTeamRound(100 - a.reworkRate())
}

func biTeamPercent(part, total int) float64 {
	if part <= 0 || total <= 0 {
		return 0
	}
	value := float64(part) * 100 / float64(total)
	if value > 100 {
		value = 100
	}
	return biTeamRound(value)
}

func biTeamRound(value float64) float64 {
	return math.Round(value*100) / 100
}

func biRequirementDone(status string) bool {
	switch normalizeStatus(status) {
	case "done", "completed", "released", "archived", "closed":
		return true
	default:
		return false
	}
}

func biFirstTaskCreatedAt(tasks []task.Snapshot) *time.Time {
	var result *time.Time
	for _, task := range tasks {
		if task.CreatedAt.IsZero() || (result != nil && !task.CreatedAt.Before(*result)) {
			continue
		}
		createdAt := task.CreatedAt
		result = &createdAt
	}
	return result
}

func biTeamBugOwner(bug quality.BugSnapshot) uint {
	if bug.ReviewerUserID != nil && *bug.ReviewerUserID > 0 {
		return *bug.ReviewerUserID
	}
	if bug.ReporterUserID != nil {
		return *bug.ReporterUserID
	}
	return 0
}

func biInvalidBugResolution(bug quality.BugSnapshot) bool {
	reason := normalizeStatus(bug.ResolutionReason)
	return reason == "duplicate" || reason == "as_designed" || reason == "external"
}

func biBugHasResolutionSample(bug quality.BugSnapshot) bool {
	return normalizeStatus(bug.ResolutionReason) != ""
}

func biEffectiveQualityBug(bug quality.BugSnapshot) bool {
	status := normalizeStatus(bug.Status)
	return status != "canceled" && status != "cancelled" && !biInvalidBugResolution(bug)
}

func biRequirementHasEffectiveBug(bugs []quality.BugSnapshot) bool {
	for _, bug := range bugs {
		if biEffectiveQualityBug(bug) {
			return true
		}
	}
	return false
}

func dimensionOrDefault(value string) string {
	if value == biTeamDimensionMonth || value == biTeamDimensionWeek {
		return value
	}
	return biTeamDimensionVersion
}

func biTeamMetricNotes() []string {
	return []string{
		"统计范围：仅统计当前工作区及当前项目筛选范围内、具备明确岗位归属的数据；未分配记录不进入人员指标。",
		"月/周口径：需求、任务和 Bug 数按创建时间；产品评审按决策时间、一次通过按最终通过时间、逾期率按截止时间、测试耗时按关闭时间、研发响应按首次响应时间。",
		"产品评审：仅统计工作区配置的一套已发布需求工作流和一个审批节点；每次通过或驳回均计为一次决策。",
		"测试指标：Bug 数归属报告人；解决到关闭耗时归属最终验证人；无效 Bug 率统计重复、符合设计和外部原因，取消记录不参与。",
		"研发指标：逾期率分母是自身已设置或可继承最近父级截止时间的任务，交付时间取最后一次有效进入待验证的时间；未经过待验证直接完成时取完成时间，退回研发后原提测时间失效；Bug/任务率不封顶；P0/P1 团队占比使用当前范围团队对应严重级别 Bug 总数。",
		"研发响应：归属最终负责人，从其最近一次被指派起算，到首次进入处理中、待评审或已解决；缺少可靠活动记录时不计算耗时。",
		"旧综合分、权重和样本资格字段仅为接口兼容保留，当前团队页面不展示或使用。",
	}
}
