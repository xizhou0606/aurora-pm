package report

import (
	"context"
	"sort"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

// ReportTargetKind selects the reporting subject: a single member or a whole
// role group.
type ReportTargetKind string

const (
	TargetMember     ReportTargetKind = "member"
	TargetGroup      ReportTargetKind = "group"
	TargetDepartment ReportTargetKind = "department"
)

// ReportTarget is the reporting subject rendered by reporting surfaces.
type ReportTarget struct {
	Kind      ReportTargetKind `json:"kind"`
	ID        uint             `json:"id"`
	Name      string           `json:"name"`
	SubLabel  string           `json:"sub_label,omitempty"`
	RoleNames []string         `json:"role_names,omitempty"`
	// JobTitle mirrors the synced Feishu job title (职务) of a member target so
	// surfaces can tell the 职务兜底 source apart from granted roles and fall
	// back to it when RoleNames is empty. Optional HTTP field; group and
	// department targets never carry it.
	JobTitle  string `json:"job_title,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// TaskMetrics summarises the task load of the report target for the window.
type TaskMetrics struct {
	Completed      int              `json:"completed"`
	CompletedItems []TaskReportItem `json:"completed_items"`
	// CompletedOnTime counts completed items that met their due date (undated
	// items count as on time) over the FULL completed set, so the punctuality
	// score is computed before the displayed list is truncated to 20.
	CompletedOnTime int              `json:"completed_on_time"`
	Created         int              `json:"created"`
	Active          int              `json:"active"`
	ActiveItems     []TaskReportItem `json:"active_items"`
	Overdue         int              `json:"overdue"`
	OverdueItems    []TaskReportItem `json:"overdue_items"`
}

// TaskReportItem is a completed task line rendered in the report body.
type TaskReportItem struct {
	ID          uint       `json:"id"`
	Key         string     `json:"key,omitempty"`
	Title       string     `json:"title"`
	Status      string     `json:"status,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
}

// BugMetrics summarises the bug handling of the report target for the window.
type BugMetrics struct {
	Resolved      int             `json:"resolved"`
	ResolvedItems []BugReportItem `json:"resolved_items"`
	// ResolvedOnTime is the bug-side counterpart of CompletedOnTime.
	ResolvedOnTime int             `json:"resolved_on_time"`
	Closed         int             `json:"closed"`
	Open           int             `json:"open"`
	OpenItems      []BugReportItem `json:"open_items"`
	Overdue        int             `json:"overdue"`
}

// BugReportItem is a resolved bug line rendered in the report body.
type BugReportItem struct {
	ID         uint       `json:"id"`
	Key        string     `json:"key,omitempty"`
	Title      string     `json:"title"`
	Status     string     `json:"status,omitempty"`
	Severity   string     `json:"severity,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	DueAt      *time.Time `json:"due_at,omitempty"`
}

// MemberRollup is one member's share of a group report so 组内导出 can carry
// a per-member section instead of only the group aggregate.
type MemberRollup struct {
	Member MemberRef           `json:"member"`
	Tasks  TaskMetrics         `json:"tasks"`
	Bugs   BugMetrics          `json:"bugs"`
	Code   *CodeActivityReport `json:"code,omitempty"`
	// BugReports 是测试成员的「提出缺陷」分档（Code 置空，二者互斥）；
	// Requirements 是产品成员的「需求数量」事实，UIBugs 是设计成员的
	// 「UI 缺陷」处理事实，均与 Code 互斥。
	BugReports   *BugReportMetrics   `json:"bug_reports,omitempty"`
	Requirements *RequirementMetrics `json:"requirements,omitempty"`
	UIBugs       *UIBugMetrics       `json:"ui_bugs,omitempty"`
}

// CodeActivityReport is the 代码活动 block: code volume is a display fact,
// never a scored dimension (LOC 不计分).
type CodeActivityReport struct {
	Repos            int `json:"repos"`
	Commits          int `json:"commits"`
	Additions        int `json:"additions"`
	Deletions        int `json:"deletions"`
	PRsMerged        int `json:"prs_merged"`
	ReviewsCompleted int `json:"reviews_completed"`
	// 行数构成（四类合计 = additions）与"带评审合入"数：把 5557 这类总数
	// 拆成可解释的部分，评审覆盖 = prs_reviewed / prs_merged。
	SourceAdditions int                `json:"source_additions"`
	TestAdditions   int                `json:"test_additions"`
	DocAdditions    int                `json:"doc_additions"`
	ConfigAdditions int                `json:"config_additions"`
	PRsReviewed     int                `json:"prs_reviewed"`
	DebugHits       int                `json:"debug_hits"`
	TodoHits        int                `json:"todo_hits"`
	CommitReviews   []CommitReviewItem `json:"commit_reviews,omitempty"`
	Freshness       CodeFreshness      `json:"freshness"`
}

// CommitReviewItem hands one commit (its stats, quality hits, and a truncated
// added-lines sample) to the AI for per-commit review in the narrative.
type CommitReviewItem struct {
	SHA        string `json:"sha"`
	Day        string `json:"day"`
	Subject    string `json:"subject"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	Files      int    `json:"files"`
	DebugHits  int    `json:"debug_hits,omitempty"`
	TodoHits   int    `json:"todo_hits,omitempty"`
	DiffSample string `json:"diff_sample,omitempty"`
	// AI 逐条结论（代码质量审查技能写回，随报告输入持久化）。
	AiVerdict string `json:"ai_verdict,omitempty"`
	AiSummary string `json:"ai_summary,omitempty"`
}

// 逐提交证据的容量预算：明细最多 100 条（超出按体量保留最大的），diff 样本
// 总预算 48KB（按体量优先分配），防止单份报告输入无限膨胀。
const (
	commitReviewItemCap   = 200
	commitReviewSampleCap = 128 * 1024
)

// CodeFreshness distinguishes "no activity" from "collection broken"; a
// degraded report must say so instead of implying zero contribution.
type CodeFreshness struct {
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	Degraded      bool       `json:"degraded"`
	Error         string     `json:"error,omitempty"`
}

// Report is the generated daily/weekly/monthly report for a member or group.
// It is a computed projection; reports are never persisted by this domain.
type Report struct {
	Target      ReportTarget        `json:"target"`
	Period      Period              `json:"period"`
	GeneratedAt time.Time           `json:"generated_at"`
	Tasks       TaskMetrics         `json:"tasks"`
	Bugs        BugMetrics          `json:"bugs"`
	Code        *CodeActivityReport `json:"code,omitempty"`
	// BugReports（测试岗/测试团队）与 Requirements（产品岗/产品团队）是
	// 岗位焦点事实：二者出现时 Code 置空，代码活动不再参与该汇报。UIBugs
	// 是设计岗/设计团队的焦点事实，同样与 Code 互斥。
	BugReports   *BugReportMetrics   `json:"bug_reports,omitempty"`
	Requirements *RequirementMetrics `json:"requirements,omitempty"`
	UIBugs       *UIBugMetrics       `json:"ui_bugs,omitempty"`
	Scores       ReportScores        `json:"scores"`
	Members      []MemberRollup      `json:"members,omitempty"`
}

// MemberReport derives the facts-only report preview for one workspace member.
func (service *Service) MemberReport(
	ctx context.Context,
	workspaceID uint,
	factID uint,
	periodType PeriodType,
	anchorDate string,
	location *time.Location,
) (Report, error) {
	report, _, err := service.buildReportWithPeriod(ctx, workspaceID, TargetMember, factID, periodType, anchorDate, location)
	return report, err
}

// DepartmentReport derives the facts-only report preview for one synced
// Feishu department: the aggregation covers every active workspace member in
// the department subtree, so intermediate nodes report as a whole.
func (service *Service) DepartmentReport(
	ctx context.Context,
	workspaceID uint,
	departmentID uint,
	periodType PeriodType,
	anchorDate string,
	location *time.Location,
) (Report, error) {
	report, _, err := service.buildReportWithPeriod(ctx, workspaceID, TargetDepartment, departmentID, periodType, anchorDate, location)
	return report, err
}

// GroupReport derives the facts-only report preview for one role group: the
// aggregation covers every active member of the group.
func (service *Service) GroupReport(
	ctx context.Context,
	workspaceID uint,
	groupID uint,
	periodType PeriodType,
	anchorDate string,
	location *time.Location,
) (Report, error) {
	report, _, err := service.buildReportWithPeriod(ctx, workspaceID, TargetGroup, groupID, periodType, anchorDate, location)
	return report, err
}

func (service *Service) buildReportWithPeriod(
	ctx context.Context,
	workspaceID uint,
	kind ReportTargetKind,
	targetID uint,
	periodType PeriodType,
	anchorDate string,
	location *time.Location,
) (Report, Period, error) {
	if workspaceID == 0 || targetID == 0 || (kind != TargetMember && kind != TargetGroup && kind != TargetDepartment) {
		return Report{}, Period{}, ErrReportQueryInvalid
	}
	anchor, err := ParseAnchorDate(anchorDate, location, service.now())
	if err != nil {
		return Report{}, Period{}, err
	}
	period, err := ResolvePeriod(periodType, anchor, location, service.now())
	if err != nil {
		return Report{}, Period{}, err
	}
	now := service.now()
	tasks, _, err := service.dependencies.Tasks.ListBIFacts(ctx, workspaceID)
	if err != nil {
		return Report{}, Period{}, err
	}
	bugs, _, err := service.dependencies.Bugs.ListBIFacts(ctx, workspaceID)
	if err != nil {
		return Report{}, Period{}, err
	}
	// 提出缺陷按 reporter 分桶、需求按产品负责人分桶，各主体只聚合自己的
	// 桶；需求端口未接线时桶为 nil，产品岗指标退化为零（未接入）。
	reportedBugBuckets := BucketBugsByReporter(bugs)
	var ownedRequirementBuckets map[uint][]requirement.Snapshot
	if service.dependencies.Requirements != nil {
		requirements, err := service.dependencies.Requirements.ListBIFacts(ctx, workspaceID)
		if err != nil {
			return Report{}, Period{}, err
		}
		ownedRequirementBuckets = BucketRequirementsByProductOwner(requirements)
	}
	taskBuckets := BucketTasksByAssignee(tasks)
	bugBuckets := BucketBugsByAssignee(bugs)
	contributionBuckets, contributionFreshness, commitBuckets, err := service.loadContributionFacts(
		ctx, workspaceID, period.Start, period.End,
	)
	if err != nil {
		return Report{}, Period{}, err
	}
	var facts ReportFacts
	if kind == TargetMember {
		member, err := service.dependencies.Directory.FindWorkspaceMember(ctx, workspaceID, targetID)
		if err != nil {
			return Report{}, Period{}, err
		}
		facts = ReportFacts{
			Target:                memberTarget(member),
			Tasks:                 taskBuckets[member.FactID],
			Bugs:                  bugBuckets[member.FactID],
			ReportedBugs:          reportedBugBuckets,
			OwnedRequirements:     ownedRequirementBuckets,
			ContributionRows:      contributionBuckets,
			ContributionFreshness: contributionFreshness,
			CommitRows:            commitBuckets,
		}
	} else if kind == TargetDepartment {
		department, err := service.dependencies.Directory.FindWorkspaceDepartment(ctx, workspaceID, targetID)
		if err != nil {
			return Report{}, Period{}, err
		}
		facts = departmentFacts(department, taskBuckets, bugBuckets, reportedBugBuckets, ownedRequirementBuckets, contributionBuckets, contributionFreshness, commitBuckets)
	} else {
		group, err := service.dependencies.Directory.FindWorkspaceGroup(ctx, workspaceID, targetID)
		if err != nil {
			return Report{}, Period{}, err
		}
		facts = groupFacts(group, taskBuckets, bugBuckets, reportedBugBuckets, ownedRequirementBuckets, contributionBuckets, contributionFreshness, commitBuckets)
	}
	return composeReport(facts, period, now), period, nil
}

// composeReport is the pure facts-to-report projection shared by the
// interactive preview, interactive generation, and the scheduler. 岗位焦点
// （测试岗/产品岗成员、测试/产品团队）在收尾处把代码活动换成各自的核心
// 事实，其余主体保持任务/缺陷/代码投影不变。
func composeReport(facts ReportFacts, period Period, now time.Time) Report {
	focus := reportFocusFor(facts.Target)
	memberRollups := make([]MemberRollup, 0, len(facts.Members))
	targetRows := facts.targetContributionRows(facts.Target.ID)
	targetCommits := facts.targetCommitRows(facts.Target.ID)
	for _, member := range facts.Members {
		rollup := MemberRollup{
			Member: member.Member,
			Tasks:  aggregateTasks(member.Tasks, period, now),
			Bugs:   aggregateBugs(member.Bugs, period, now),
		}
		switch focus {
		case focusTesting:
			// 测试团队成员不再展示代码活动，改为提出缺陷分档。
			metrics := aggregateBugReports(facts.ReportedBugs[member.Member.ID], period)
			rollup.BugReports = &metrics
		case focusProduct:
			// 产品团队成员同样隐藏代码活动，改为需求数量事实。
			metrics := aggregateRequirementMetrics(facts.OwnedRequirements[member.Member.ID], period)
			rollup.Requirements = &metrics
		case focusDesign:
			// 设计团队成员隐藏代码活动，改为 UI 缺陷处理事实（负责人口径）。
			metrics := aggregateUIBugs(member.Bugs, period, now)
			rollup.UIBugs = &metrics
		default:
			memberCommits := facts.CommitRows[member.Member.ID]
			memberCode := aggregateCode(facts.ContributionRows[member.Member.ID], period)
			// 有逐提交证据但无归属贡献行时保留代码块，代码质量评比（调试
			// 残留/TODO 命中）不因聚合为空而丢失。
			if memberCode == nil && len(memberCommits) > 0 {
				memberCode = &CodeActivityReport{}
			}
			attachCommitHygiene(memberCode, memberCommits)
			rollup.Code = memberCode
		}
		memberRollups = append(memberRollups, rollup)
		targetRows = append(targetRows, facts.ContributionRows[member.Member.ID]...)
	}
	tasks := aggregateTasks(facts.Tasks, period, now)
	bugs := aggregateBugs(facts.Bugs, period, now)
	code := aggregateCode(targetRows, period)
	// 有提交证据但无归属行时也保留代码块，逐提交点评不因聚合为空而丢失。
	if code == nil && len(targetCommits) > 0 {
		code = &CodeActivityReport{}
	}
	attachCommitReviews(code, targetCommits, period)
	report := Report{
		Target:      facts.Target,
		Period:      period,
		GeneratedAt: now,
		Tasks:       tasks,
		Bugs:        bugs,
		Code:        code,
		Scores:      ComputeReportScores(tasks, bugs, period, code),
		Members:     memberRollups,
	}
	if facts.ContributionFreshness != nil {
		if report.Code == nil {
			report.Code = &CodeActivityReport{}
		}
		report.Code.Freshness = *facts.ContributionFreshness
	}
	applyRoleFocus(&report, facts, period, now, focus)
	return report
}

// reportFocusFor classifies a composed report subject: members by their role
// keywords, teams by their display name.
func reportFocusFor(target ReportTarget) roleFocus {
	if target.Kind == TargetMember {
		return memberRoleFocus(target)
	}
	return teamRoleFocus(target)
}

// applyRoleFocus swaps the code-activity block for the subject's role core
// facts. It runs last so the collection-freshness garnish never resurrects
// the code block for testing/product subjects. Scores are recomputed without
// the code aggregate so the radar drops the code axes instead of scoring an
// invisible block.
func applyRoleFocus(report *Report, facts ReportFacts, period Period, now time.Time, focus roleFocus) {
	switch focus {
	case focusTesting:
		report.Code = nil
		if report.Target.Kind == TargetMember {
			metrics := aggregateBugReports(facts.ReportedBugs[report.Target.ID], period)
			report.BugReports = &metrics
		} else {
			metrics := aggregateBugReports(facts.teamReportedBugs(), period)
			report.BugReports = &metrics
		}
		report.Scores = ComputeReportScores(report.Tasks, report.Bugs, period, nil)
	case focusProduct:
		report.Code = nil
		if report.Target.Kind == TargetMember {
			metrics := aggregateRequirementMetrics(facts.OwnedRequirements[report.Target.ID], period)
			report.Requirements = &metrics
		} else {
			metrics := aggregateRequirementMetricsDistinct(facts.teamOwnedRequirementBuckets(), period)
			report.Requirements = &metrics
		}
		report.Scores = ComputeReportScores(report.Tasks, report.Bugs, period, nil)
	case focusDesign:
		// 设计岗（成员与团队同口径）：facts.Bugs 对成员即本人负责桶、对团队
		// 即成员桶并集；UI 缺陷处理事实替换代码活动。
		report.Code = nil
		metrics := aggregateUIBugs(facts.Bugs, period, now)
		report.UIBugs = &metrics
		report.Scores = ComputeReportScores(report.Tasks, report.Bugs, period, nil)
	}
}

// aggregateCode sums one person's day buckets inside the period. Day keys are
// stored in the workspace timezone and compared against the period's calendar
// days, so the window edges are day-accurate for the configured timezone.
func aggregateCode(rows []DailyContributionFact, period Period) *CodeActivityReport {
	if rows == nil {
		return nil
	}
	start := period.Start.Format(anchorDateLayout)
	// End 是右开边界（次日 00:00），按天字符串过滤必须用 < 而不是 <=，
	// 否则下一周期第一天（如周一）会被算进上一份周报。
	end := period.End.Format(anchorDateLayout)
	code := &CodeActivityReport{}
	repos := make(map[uint]struct{}, 2)
	for _, row := range rows {
		if row.Day < start || row.Day >= end {
			continue
		}
		repos[row.RepoID] = struct{}{}
		code.Commits += row.Commits
		code.Additions += row.Additions
		code.Deletions += row.Deletions
		code.PRsMerged += row.PRsMerged
		code.ReviewsCompleted += row.ReviewsCompleted
		code.SourceAdditions += row.SourceAdditions
		code.TestAdditions += row.TestAdditions
		code.DocAdditions += row.DocAdditions
		code.ConfigAdditions += row.ConfigAdditions
		code.PRsReviewed += row.PRsReviewed
	}
	code.Repos = len(repos)
	return code
}

// memberTarget projects a directory member onto the report target.
func memberTarget(member identity.DirectoryMember) ReportTarget {
	return ReportTarget{
		Kind: TargetMember, ID: member.FactID, Name: member.DisplayName,
		SubLabel: member.Username, RoleNames: member.RoleNames, JobTitle: member.JobTitle,
		AvatarURL: member.AvatarURL,
	}
}

// groupFacts assembles the group composition facts: the union of member
// buckets (group aggregate) plus every member's own bucket so 组内导出 can
// carry a per-member section. Rollups are aggregated inside composeReport
// where the period is known.
func groupFacts(
	group identity.WorkspaceGroup,
	taskBuckets map[uint][]task.Snapshot,
	bugBuckets map[uint][]quality.BugSnapshot,
	reportedBugBuckets map[uint][]quality.BugSnapshot,
	ownedRequirementBuckets map[uint][]requirement.Snapshot,
	contributionBuckets map[uint][]DailyContributionFact,
	contributionFreshness *CodeFreshness,
	commitBuckets map[uint][]MemberCommitFact,
) ReportFacts {
	facts := ReportFacts{
		Target: ReportTarget{
			Kind: TargetGroup, ID: group.ID, Name: group.Name,
			SubLabel: group.Code, RoleNames: []string{},
		},
		Tasks:             []task.Snapshot{},
		Bugs:              []quality.BugSnapshot{},
		ReportedBugs:      reportedBugBuckets,
		OwnedRequirements: ownedRequirementBuckets,
	}
	for _, member := range group.Members {
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
	facts.ContributionRows = contributionBuckets
	facts.ContributionFreshness = contributionFreshness
	facts.CommitRows = commitBuckets
	return facts
}

// departmentFacts assembles the department composition facts: the union of
// the subtree members' buckets (department aggregate) plus every member's own
// bucket so 组内导出 can carry a per-member section. Rollups aggregate inside
// composeReport where the period is known.
func departmentFacts(
	department identity.WorkspaceDepartment,
	taskBuckets map[uint][]task.Snapshot,
	bugBuckets map[uint][]quality.BugSnapshot,
	reportedBugBuckets map[uint][]quality.BugSnapshot,
	ownedRequirementBuckets map[uint][]requirement.Snapshot,
	contributionBuckets map[uint][]DailyContributionFact,
	contributionFreshness *CodeFreshness,
	commitBuckets map[uint][]MemberCommitFact,
) ReportFacts {
	facts := ReportFacts{
		Target: ReportTarget{
			Kind: TargetDepartment, ID: department.ID, Name: department.Name,
			RoleNames: []string{},
		},
		Tasks:             []task.Snapshot{},
		Bugs:              []quality.BugSnapshot{},
		ReportedBugs:      reportedBugBuckets,
		OwnedRequirements: ownedRequirementBuckets,
	}
	for _, member := range department.Members {
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
	facts.ContributionRows = contributionBuckets
	facts.ContributionFreshness = contributionFreshness
	facts.CommitRows = commitBuckets
	return facts
}

// BucketTasksByAssignee partitions workspace task snapshots once so every
// report subject aggregates only its own bucket instead of rescanning the
// workspace per subject. Unassigned tasks belong to no bucket.
func BucketTasksByAssignee(snapshots []task.Snapshot) map[uint][]task.Snapshot {
	buckets := make(map[uint][]task.Snapshot)
	for _, snapshot := range snapshots {
		if snapshot.AssigneeID == nil {
			continue
		}
		buckets[*snapshot.AssigneeID] = append(buckets[*snapshot.AssigneeID], snapshot)
	}
	return buckets
}

// BucketBugsByAssignee is the bug-side counterpart of BucketTasksByAssignee.
func BucketBugsByAssignee(snapshots []quality.BugSnapshot) map[uint][]quality.BugSnapshot {
	buckets := make(map[uint][]quality.BugSnapshot)
	for _, snapshot := range snapshots {
		if snapshot.AssigneeUserID == nil {
			continue
		}
		buckets[*snapshot.AssigneeUserID] = append(buckets[*snapshot.AssigneeUserID], snapshot)
	}
	return buckets
}

func aggregateTasks(snapshots []task.Snapshot, period Period, now time.Time) TaskMetrics {
	metrics := TaskMetrics{
		CompletedItems: []TaskReportItem{},
		ActiveItems:    []TaskReportItem{},
		OverdueItems:   []TaskReportItem{},
	}
	completedAll := []TaskReportItem{}
	for _, snapshot := range snapshots {
		if task.IsCancelledStatus(snapshot.Status) {
			continue
		}
		if withinPeriod(snapshot.CompletedAt, period) {
			metrics.Completed++
			if completedOnTime(snapshot.CompletedAt, snapshot.DueAt) {
				metrics.CompletedOnTime++
			}
			// Collect every completed item first: the newest win after the
			// sort below, never "the newest of an arbitrary first 20" (R3).
			completedAll = append(completedAll, TaskReportItem{
				ID:          snapshot.ID,
				Key:         snapshot.TaskKey,
				Title:       snapshot.Title,
				Status:      snapshot.Status,
				CompletedAt: snapshot.CompletedAt,
			})
		}
		if withinPeriod(&snapshot.CreatedAt, period) {
			metrics.Created++
		}
		if !task.IsTerminalStatus(snapshot.Status) {
			metrics.Active++
			item := TaskReportItem{
				ID:     snapshot.ID,
				Key:    snapshot.TaskKey,
				Title:  snapshot.Title,
				Status: snapshot.Status,
				DueAt:  snapshot.DueAt,
			}
			overdue := snapshot.DueAt != nil && snapshot.DueAt.Before(now)
			if len(metrics.ActiveItems) < reportedItemsLimit {
				metrics.ActiveItems = append(metrics.ActiveItems, item)
			}
			if overdue {
				metrics.Overdue++
				if len(metrics.OverdueItems) < reportedItemsLimit {
					metrics.OverdueItems = append(metrics.OverdueItems, item)
				}
			}
		}
	}
	sort.SliceStable(completedAll, func(left, right int) bool {
		a, b := completedAll[left], completedAll[right]
		before := timeBeforeDesc(a.CompletedAt, b.CompletedAt)
		after := timeBeforeDesc(b.CompletedAt, a.CompletedAt)
		if !before && !after {
			return a.ID > b.ID
		}
		return before
	})
	if len(completedAll) > reportedItemsLimit {
		completedAll = completedAll[:reportedItemsLimit]
	}
	metrics.CompletedItems = completedAll
	// In-progress and overdue lists read best in urgency order: earliest due
	// date first, undated last (R3: ordering before truncation everywhere).
	sort.SliceStable(metrics.ActiveItems, func(left, right int) bool {
		return dueBefore(metrics.ActiveItems[left].DueAt, metrics.ActiveItems[right].DueAt,
			metrics.ActiveItems[left].ID, metrics.ActiveItems[right].ID)
	})
	sort.SliceStable(metrics.OverdueItems, func(left, right int) bool {
		return dueBefore(metrics.OverdueItems[left].DueAt, metrics.OverdueItems[right].DueAt,
			metrics.OverdueItems[left].ID, metrics.OverdueItems[right].ID)
	})
	return metrics
}

// dueBefore orders by due date ascending with undated last, id ascending as
// the stable tie-break.
func dueBefore(left, right *time.Time, leftID, rightID uint) bool {
	if left == nil && right == nil {
		return leftID < rightID
	}
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	if !left.Equal(*right) {
		return left.Before(*right)
	}
	return leftID < rightID
}

func aggregateBugs(snapshots []quality.BugSnapshot, period Period, now time.Time) BugMetrics {
	metrics := BugMetrics{ResolvedItems: []BugReportItem{}, OpenItems: []BugReportItem{}}
	resolvedAll := []BugReportItem{}
	for _, snapshot := range snapshots {
		if withinPeriod(snapshot.ResolvedAt, period) {
			metrics.Resolved++
			if completedOnTime(snapshot.ResolvedAt, snapshot.DueAt) {
				metrics.ResolvedOnTime++
			}
			resolvedAll = append(resolvedAll, BugReportItem{
				ID:         snapshot.ID,
				Key:        snapshot.BugKey,
				Title:      snapshot.Title,
				Status:     snapshot.Status,
				Severity:   snapshot.SeverityKey,
				ResolvedAt: snapshot.ResolvedAt,
			})
		}
		if snapshot.Status == "done" && withinPeriod(snapshot.CompletedAt, period) {
			metrics.Closed++
		}
		if openBugStatuses[snapshot.Status] {
			metrics.Open++
			if len(metrics.OpenItems) < reportedItemsLimit {
				metrics.OpenItems = append(metrics.OpenItems, BugReportItem{
					ID:       snapshot.ID,
					Key:      snapshot.BugKey,
					Title:    snapshot.Title,
					Status:   snapshot.Status,
					Severity: snapshot.SeverityKey,
					DueAt:    snapshot.DueAt,
				})
			}
			if snapshot.DueAt != nil && snapshot.DueAt.Before(now) {
				metrics.Overdue++
			}
		}
	}
	sort.SliceStable(resolvedAll, func(left, right int) bool {
		a, b := resolvedAll[left], resolvedAll[right]
		before := timeBeforeDesc(a.ResolvedAt, b.ResolvedAt)
		after := timeBeforeDesc(b.ResolvedAt, a.ResolvedAt)
		if !before && !after {
			return a.ID > b.ID
		}
		return before
	})
	if len(resolvedAll) > reportedItemsLimit {
		resolvedAll = resolvedAll[:reportedItemsLimit]
	}
	metrics.ResolvedItems = resolvedAll
	// Open defects read best in severity order (p0 first), then earliest due
	// date; the id keeps the order stable.
	sort.SliceStable(metrics.OpenItems, func(left, right int) bool {
		a, b := metrics.OpenItems[left], metrics.OpenItems[right]
		if severityRank(a.Severity) != severityRank(b.Severity) {
			return severityRank(a.Severity) < severityRank(b.Severity)
		}
		return dueBefore(a.DueAt, b.DueAt, a.ID, b.ID)
	})
	return metrics
}

// severityRank orders the known defect severity keys; unknown keys sort last.
func severityRank(severity string) int {
	switch severity {
	case "p0":
		return 0
	case "p1":
		return 1
	case "p2":
		return 2
	case "p3":
		return 3
	default:
		return 4
	}
}

// openBugStatuses mirrors the quality module's non-terminal lifecycle states.
var openBugStatuses = map[string]bool{
	"triage": true, "todo": true, "in_progress": true, "reopened": true, "review": true,
}

func timeBeforeDesc(left, right *time.Time) bool {
	if left == nil && right == nil {
		return false
	}
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	return left.After(*right)
}

// completedOnTime reports whether a completed/resolved item met its due
// date. Undated work cannot be late, so it counts as on time.
// BucketContributionsByUser partitions daily contribution rows once per
// workspace, mirroring BucketTasksByAssignee. UserID 0（未归属）不进任何人的桶。
func BucketContributionsByUser(rows []DailyContributionFact) map[uint][]DailyContributionFact {
	buckets := make(map[uint][]DailyContributionFact)
	for _, row := range rows {
		if row.UserID == 0 {
			continue
		}
		buckets[row.UserID] = append(buckets[row.UserID], row)
	}
	return buckets
}

// loadContributionFacts loads the shared code facts once per workspace pass:
// daily contribution buckets, collection freshness, and member-attributed
// commit rows for the period window. A nil port yields nil buckets (未接入).
func (service *Service) loadContributionFacts(
	ctx context.Context, workspaceID uint, since, until time.Time,
) (map[uint][]DailyContributionFact, *CodeFreshness, map[uint][]MemberCommitFact, error) {
	if service.dependencies.Contributions == nil {
		return nil, nil, nil, nil
	}
	rows, err := service.dependencies.Contributions.ListDailyContributions(ctx, workspaceID)
	if err != nil {
		return nil, nil, nil, err
	}
	buckets := BucketContributionsByUser(rows)
	states, err := service.dependencies.Contributions.ListRepoSyncStates(ctx, workspaceID)
	if err != nil {
		return nil, nil, nil, err
	}
	freshness := codeFreshnessFor(states)
	commits, err := service.dependencies.Contributions.ListMemberCommits(ctx, workspaceID, since, until)
	if err != nil {
		return nil, nil, nil, err
	}
	return buckets, freshness, BucketCommitsByUser(commits), nil
}

// BucketCommitsByUser partitions member commit facts once per workspace pass.
func BucketCommitsByUser(rows []MemberCommitFact) map[uint][]MemberCommitFact {
	buckets := make(map[uint][]MemberCommitFact, 8)
	for _, row := range rows {
		buckets[row.UserID] = append(buckets[row.UserID], row)
	}
	return buckets
}

// attachCommitHygiene sums one member's per-commit hygiene hits into their
// code block so the 代码贡献排行 can rank 代码质量 (调试残留/TODO) beside
// volume. Diff samples stay target-only: the rollup stays lean.
func attachCommitHygiene(code *CodeActivityReport, commits []MemberCommitFact) {
	if code == nil || len(commits) == 0 {
		return
	}
	for _, commit := range commits {
		code.DebugHits += commit.DebugHits
		code.TodoHits += commit.TodoHits
	}
}

// attachCommitReviews fills the code block's per-commit review list. Items
// are capped by count (largest first) and diff samples by a total budget so
// one report input stays bounded; the list itself stays chronological.
func attachCommitReviews(code *CodeActivityReport, commits []MemberCommitFact, period Period) {
	if code == nil || len(commits) == 0 {
		return
	}
	for _, commit := range commits {
		code.DebugHits += commit.DebugHits
		code.TodoHits += commit.TodoHits
	}
	location := period.Start.Location()
	items := make([]CommitReviewItem, 0, len(commits))
	for _, commit := range commits {
		items = append(items, CommitReviewItem{
			SHA:     shortSHA(commit.SHA),
			Day:     commit.CommittedAt.In(location).Format(anchorDateLayout),
			Subject: commit.Subject, Additions: commit.Additions,
			Deletions: commit.Deletions, Files: commit.Files,
			DebugHits: commit.DebugHits, TodoHits: commit.TodoHits,
			DiffSample: commit.DiffSample,
		})
	}
	if len(items) > commitReviewItemCap {
		sort.SliceStable(items, func(left, right int) bool { return items[left].Additions > items[right].Additions })
		items = items[:commitReviewItemCap]
	}
	bySize := append([]CommitReviewItem{}, items...)
	sort.SliceStable(bySize, func(left, right int) bool { return bySize[left].Additions > bySize[right].Additions })
	budget := commitReviewSampleCap
	kept := make(map[string]bool, len(bySize))
	for _, item := range bySize {
		if len(item.DiffSample) <= budget {
			budget -= len(item.DiffSample)
			kept[item.SHA] = true
		}
	}
	for index := range items {
		if !kept[items[index].SHA] {
			items[index].DiffSample = ""
		}
	}
	sort.SliceStable(items, func(left, right int) bool { return items[left].Day < items[right].Day })
	code.CommitReviews = items
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func codeFreshnessFor(states []RepoSyncStatus) *CodeFreshness {
	freshness := &CodeFreshness{}
	for _, state := range states {
		if state.LastSuccessAt != nil &&
			(freshness.LastSuccessAt == nil || state.LastSuccessAt.After(*freshness.LastSuccessAt)) {
			freshness.LastSuccessAt = state.LastSuccessAt
		}
		if state.ConsecutiveFailures > 0 && state.LastError != "" {
			freshness.Degraded = true
			if freshness.Error == "" {
				freshness.Error = state.LastError
			}
		}
	}
	return freshness
}

func completedOnTime(doneAt, dueAt *time.Time) bool {
	if dueAt == nil {
		return true
	}
	if doneAt == nil {
		return false
	}
	return !doneAt.After(*dueAt)
}
