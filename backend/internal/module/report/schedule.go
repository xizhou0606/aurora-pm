package report

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

// Schedule is the reporting cadence contract: daily at 09:00, weekly on
// Monday 09:00, monthly on the 1st 09:00 — all in the schedule location
// (the deployment's business timezone).
type Schedule struct {
	Type PeriodType
	Hour int
}

const (
	scheduleHour        = 9
	scheduleCronOffset  = 8 * time.Hour
	scheduleLocFallback = "Asia/Shanghai"
	// workspaceConcurrency bounds how many workspaces a scheduled pass
	// processes in parallel. Each workspace costs two fact queries plus
	// per-subject in-memory aggregation; a small pool overlaps the slowest
	// workspaces without multiplying DB pressure.
	workspaceConcurrency = 4
)

// ScheduleLocation returns the business timezone used for scheduled report
// generation. Deployments run in China; a missing tzdata falls back to UTC+8.
func ScheduleLocation() *time.Location {
	location, err := time.LoadLocation(scheduleLocFallback)
	if err != nil {
		return time.FixedZone("CST", int(scheduleCronOffset/time.Second))
	}
	return location
}

// RunDedupQueries is the idempotency port for scheduled generation.
type RunDedupQueries interface {
	HasReportRun(ctx context.Context, query ai.ReportRunDedupQuery) (bool, error)
	ListReportRunSubjectIDs(ctx context.Context, query ai.ReportRunDedupBatchQuery) (map[uint]bool, error)
}

// WorkspaceQueries enumerates the workspaces the scheduler covers.
type WorkspaceQueries interface {
	ListEnabledWorkspaceIDs(ctx context.Context) ([]uint, error)
}

// ScheduleRunSummary reports what one scheduled pass generated.
type ScheduleRunSummary struct {
	WorkspaceID uint `json:"workspace_id"`
	PeriodType  PeriodType
	AnchorDate  string
	Created     int
	Skipped     int
	Failed      int
}

// ReportScheduler composes the scheduled generation use case on top of the
// report service. It reuses the same AI run pipeline as interactive
// generation; runs are marked Scheduled so they carry no operator principal.
type ReportScheduler struct {
	service    *Service
	dedup      RunDedupQueries
	workspaces WorkspaceQueries
	now        func() time.Time
}

func NewReportScheduler(service *Service, dedup RunDedupQueries, workspaces WorkspaceQueries) (*ReportScheduler, error) {
	if service == nil || dedup == nil || workspaces == nil {
		return nil, ErrReportUnavailable
	}
	return &ReportScheduler{service: service, dedup: dedup, workspaces: workspaces, now: time.Now}, nil
}

// TriggerTime returns the 09:00 occurrence of periodType inside the calendar
// frame that contains the local moment, in the schedule location: the
// containing day (daily), the containing ISO week's Monday (weekly), or the
// containing month's first day (monthly). ok is false for unknown period
// types.
func TriggerTime(periodType PeriodType, location *time.Location, moment time.Time) (time.Time, bool) {
	if location == nil {
		location = ScheduleLocation()
	}
	local := moment.In(location)
	switch periodType {
	case PeriodDaily:
		return scheduleMoment(local.Year(), local.Month(), local.Day(), location), true
	case PeriodWeekly:
		// ISO week starts Monday: (weekday+6)%7 counts the days since Monday.
		monday := local.AddDate(0, 0, -(int(local.Weekday())+6)%7)
		return scheduleMoment(monday.Year(), monday.Month(), monday.Day(), location), true
	case PeriodMonthly:
		return scheduleMoment(local.Year(), local.Month(), 1, location), true
	default:
		return time.Time{}, false
	}
}

func scheduleMoment(year int, month time.Month, day int, location *time.Location) time.Time {
	return time.Date(year, month, day, scheduleHour, 0, 0, 0, location)
}

// TriggerAnchor is the stable per-granularity identifier of the schedule
// occurrence that contains the local moment: the trigger's own date (daily →
// that day, weekly → that week's Monday, monthly → the month's 1st). Workers
// record the anchor when a pass fires and skip a cadence whose current anchor
// already fired, so one occurrence never generates twice within a process
// lifetime; the per-subject run dedup table (WindowStart/WindowEnd) remains
// the durable cross-restart guarantee.
func TriggerAnchor(periodType PeriodType, location *time.Location, moment time.Time) (string, bool) {
	trigger, ok := TriggerTime(periodType, location, moment)
	if !ok {
		return "", false
	}
	return trigger.Format(anchorDateLayout), true
}

// Due reports whether the 09:00 occurrence of periodType was crossed inside
// the half-open window (previous, now] in the schedule location. Each tick
// therefore fires every cadence at most once per occurrence, and a stalled
// ticker backfills the missed 09:00 on its next tick instead of skipping it —
// the per-subject dedup keeps the extra pass idempotent (回补). A process
// that starts after the occurrence (previous already past the trigger) does
// not re-fire it; the next occurrence triggers normally.
func Due(periodType PeriodType, location *time.Location, previous, now time.Time) bool {
	trigger, ok := TriggerTime(periodType, location, now)
	if !ok {
		return false
	}
	return trigger.After(previous) && !trigger.After(now)
}

// AnchorDateFor returns the business date string the schedule covers.
func AnchorDateFor(location *time.Location, now time.Time) string {
	if location == nil {
		location = ScheduleLocation()
	}
	local := now.In(location)
	return fmt.Sprintf("%04d-%02d-%02d", local.Year(), local.Month(), local.Day())
}

// GenerateForAllWorkspaces walks every enabled workspace and generates the
// scheduled reports for the anchor date. Workspaces run through a bounded
// pool: each pass costs two fact queries per workspace, and overlapping the
// slowest ones shortens the wall clock without multiplying DB pressure.
// Per-subject dedup keeps the whole pass idempotent across restarts and
// concurrent ticks.
func (scheduler *ReportScheduler) GenerateForAllWorkspaces(
	ctx context.Context,
	periodType PeriodType,
	location *time.Location,
) ([]ScheduleRunSummary, error) {
	if location == nil {
		location = ScheduleLocation()
	}
	now := scheduler.now()
	localNow := now.In(location)
	// 日报在 09:00 触发时生成前一日日报：锚点回退一个自然日，覆盖已经
	// 结束的完整一天。周报/月报保持触发日锚点，由 ResolvePeriod 的"上一
	// 完整周期"回补语义自然落到上一周/上一月。去重窗口仍是触发日
	// [00:00, 24:00)，保证同一触发窗口内的重复 pass 幂等。
	anchorDay := localNow
	if periodType == PeriodDaily {
		anchorDay = localNow.AddDate(0, 0, -1)
	}
	anchor := AnchorDateFor(location, anchorDay)
	windowStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	windowEnd := windowStart.AddDate(0, 0, 1)
	workspaceIDs, err := scheduler.workspaces.ListEnabledWorkspaceIDs(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]ScheduleRunSummary, len(workspaceIDs))
	passErrors := make([]error, len(workspaceIDs))
	workers := workspaceConcurrency
	if workers > len(workspaceIDs) {
		workers = len(workspaceIDs)
	}
	if workers < 1 {
		return summaries, nil
	}
	indexes := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range indexes {
				if ctx.Err() != nil {
					continue
				}
				summary, err := scheduler.GenerateForWorkspace(
					ctx, workspaceIDs[index], periodType, anchor, windowStart, windowEnd, location,
				)
				summaries[index] = summary
				passErrors[index] = err
			}
		}()
	}
	for index := range workspaceIDs {
		indexes <- index
	}
	close(indexes)
	wg.Wait()
	for _, err := range passErrors {
		if err != nil {
			return summaries, err
		}
	}
	return summaries, nil
}

// GenerateForWorkspace queues one run per active member and per non-empty
// role group of the workspace. Performance contract for a full pass:
//
//   - workspace task/bug facts load exactly once (not once per subject);
//   - snapshots bucket by assignee once, so per-subject composition only
//     touches the subject's own work instead of rescanning the workspace;
//   - dedup is two batched queries (member batch, group batch) instead of
//     one query per subject;
//   - the fan-out itself is CPU-light in-memory composition; the AI
//     generation fan-out is bounded downstream by the worker's configured
//     run concurrency.
func (scheduler *ReportScheduler) GenerateForWorkspace(
	ctx context.Context,
	workspaceID uint,
	periodType PeriodType,
	anchorDate string,
	windowStart time.Time,
	windowEnd time.Time,
	location *time.Location,
) (ScheduleRunSummary, error) {
	summary := ScheduleRunSummary{
		WorkspaceID: workspaceID, PeriodType: periodType, AnchorDate: anchorDate,
	}
	capability := reportCapability(periodType)
	members, err := scheduler.service.dependencies.Directory.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return summary, err
	}
	groups, err := scheduler.service.dependencies.Directory.ListEnabledGroups(ctx)
	if err != nil {
		return summary, err
	}
	tasks, _, err := scheduler.service.dependencies.Tasks.ListBIFacts(ctx, workspaceID)
	if err != nil {
		return summary, err
	}
	bugs, _, err := scheduler.service.dependencies.Bugs.ListBIFacts(ctx, workspaceID)
	if err != nil {
		return summary, err
	}
	anchor, err := ParseAnchorDate(anchorDate, location, scheduler.now())
	if err != nil {
		return summary, err
	}
	period, err := ResolvePeriod(periodType, anchor, location, scheduler.now())
	if err != nil {
		return summary, err
	}
	now := scheduler.now()
	taskBuckets := BucketTasksByAssignee(tasks)
	bugBuckets := BucketBugsByAssignee(bugs)
	// 提出缺陷与产品负责人需求与任务/缺陷同批装载：定时报告的个人与团队
	// 主体共享同一组分桶，岗位焦点事实不因调度路径缺失。
	reportedBugBuckets := BucketBugsByReporter(bugs)
	var ownedRequirementBuckets map[uint][]requirement.Snapshot
	if scheduler.service.dependencies.Requirements != nil {
		requirements, err := scheduler.service.dependencies.Requirements.ListBIFacts(ctx, workspaceID)
		if err != nil {
			return summary, err
		}
		ownedRequirementBuckets = BucketRequirementsByProductOwner(requirements)
	}
	// 代码事实（贡献行 + 成员提交）一次装载、全体主体共享；此前调度路径
	// 完全没接代码数据，定时报告的代码活动因此一直缺失。
	contributionRows, contributionFreshness, commitBuckets, err := scheduler.service.loadContributionFacts(
		ctx, workspaceID, period.Start, period.End,
	)
	if err != nil {
		return summary, err
	}

	// Batched dedup: one query for all member targets, one for all groups.
	// Groups come from the enabled catalog so empty groups also receive their
	// scheduled team report.
	memberFactIDs := make([]uint, 0, len(members))
	groupIDs := make([]uint, 0, len(groups))
	for _, member := range members {
		memberFactIDs = append(memberFactIDs, member.FactID)
	}
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	existingMembers, err := scheduler.dedup.ListReportRunSubjectIDs(ctx, ai.ReportRunDedupBatchQuery{
		WorkspaceID: workspaceID, Capability: capability, SubjectType: reportSubjectType(TargetMember),
		SubjectIDs: memberFactIDs, WindowStart: windowStart, WindowEnd: windowEnd,
	})
	if err != nil {
		return summary, err
	}
	existingGroups, err := scheduler.dedup.ListReportRunSubjectIDs(ctx, ai.ReportRunDedupBatchQuery{
		WorkspaceID: workspaceID, Capability: capability, SubjectType: reportSubjectType(TargetGroup),
		SubjectIDs: groupIDs, WindowStart: windowStart, WindowEnd: windowEnd,
	})
	if err != nil {
		return summary, err
	}

	queue := func(facts ReportFacts) {
		if _, err := scheduler.service.QueueScheduledReport(
			ctx, workspaceID, periodType, period, now, facts,
		); err != nil {
			summary.Failed++
			return
		}
		summary.Created++
	}
	// Members and groups compose straight from the directory list and the
	// shared buckets: no per-subject fact reload, no per-subject directory
	// lookup. Group membership is built from the FULL directory first — a
	// member whose personal run already exists must still count toward the
	// group report (R2: dedup and group membership are independent).
	membersByGroup := map[uint][]identity.DirectoryMember{}
	for _, member := range members {
		if member.GroupID > 0 {
			membersByGroup[member.GroupID] = append(membersByGroup[member.GroupID], member)
		}
	}
	for _, member := range members {
		if existingMembers[member.FactID] {
			summary.Skipped++
			continue
		}
		queue(ReportFacts{
			Target:                memberTarget(member),
			Tasks:                 taskBuckets[member.FactID],
			Bugs:                  bugBuckets[member.FactID],
			ReportedBugs:          reportedBugBuckets,
			OwnedRequirements:     ownedRequirementBuckets,
			ContributionRows:      contributionRows,
			ContributionFreshness: contributionFreshness,
			CommitRows:            commitBuckets,
		})
	}
	for _, groupID := range groupIDs {
		if existingGroups[groupID] {
			summary.Skipped++
			continue
		}
		groupMembers := membersByGroup[groupID]
		groupName, groupCode := groupLabel(groups, groupID)
		facts := ReportFacts{
			Target: ReportTarget{
				Kind: TargetGroup, ID: groupID, Name: groupName,
				SubLabel: groupCode, RoleNames: []string{},
			},
			Tasks:                 []task.Snapshot{},
			Bugs:                  []quality.BugSnapshot{},
			ReportedBugs:          reportedBugBuckets,
			OwnedRequirements:     ownedRequirementBuckets,
			ContributionRows:      contributionRows,
			ContributionFreshness: contributionFreshness,
			CommitRows:            commitBuckets,
		}
		for _, member := range groupMembers {
			memberTasks := taskBuckets[member.FactID]
			memberBugs := bugBuckets[member.FactID]
			facts.Tasks = append(facts.Tasks, memberTasks...)
			facts.Bugs = append(facts.Bugs, memberBugs...)
			facts.Members = append(facts.Members, MemberRollupFacts{
				Member: memberRef(member),
				Tasks:  memberTasks,
				Bugs:   memberBugs,
			})
		}
		queue(facts)
	}
	// 部门与成员/用户组同批进入定时生成（评语自动跟着定时任务出，无需手动
	// 点击）：子树为空的部门跳过，去重窗口与其他主体一致。部门目录读取失
	// 败只降级本批次——成员与用户组的报告不因此失败。
	departments, err := scheduler.service.dependencies.Directory.ListFeishuDepartments(ctx)
	if err != nil {
		log.Printf("report scheduler department list error (workspace %d): %v", workspaceID, err)
		return summary, nil
	}
	departmentIDs := make([]uint, 0, len(departments))
	for _, department := range departments {
		departmentIDs = append(departmentIDs, department.ID)
	}
	existingDepartments := map[uint]bool{}
	if len(departmentIDs) > 0 {
		existingDepartments, err = scheduler.dedup.ListReportRunSubjectIDs(ctx, ai.ReportRunDedupBatchQuery{
			WorkspaceID: workspaceID, Capability: capability, SubjectType: reportSubjectType(TargetDepartment),
			SubjectIDs: departmentIDs, WindowStart: windowStart, WindowEnd: windowEnd,
		})
		if err != nil {
			return summary, err
		}
	}
	for _, department := range departments {
		if existingDepartments[department.ID] {
			summary.Skipped++
			continue
		}
		subtree, err := scheduler.service.dependencies.Directory.FindWorkspaceDepartment(ctx, workspaceID, department.ID)
		if err != nil {
			summary.Failed++
			continue
		}
		if len(subtree.Members) == 0 {
			continue
		}
		queue(departmentFacts(subtree, taskBuckets, bugBuckets, reportedBugBuckets, ownedRequirementBuckets, contributionRows, contributionFreshness, commitBuckets))
	}
	return summary, nil
}

// groupLabel resolves the enabled group's display name without extra queries.
func groupLabel(groups []identity.UserGroup, groupID uint) (string, string) {
	for _, group := range groups {
		if group.ID == groupID {
			return group.Name, group.Code
		}
	}
	return "未命名用户组", ""
}
