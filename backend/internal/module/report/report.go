// Package report owns the workspace reporting domain: it groups active
// workspace members by their role group (用户组), derives daily, weekly, and
// monthly reports for a member or a whole group from the task and quality
// facts exposed by the owning modules, and hands the narrative generation to
// the built-in AI reporting skills. It deliberately contains no persistence
// of its own.
package report

import (
	"context"
	"errors"
	"time"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

var (
	ErrReportQueryInvalid      = errors.New("report query is invalid")
	ErrReportPeriodInvalid     = errors.New("report period is invalid")
	ErrReportMemberNotFound    = errors.New("report member not found")
	ErrReportUnavailable       = errors.New("report dependency is unavailable")
	ErrReportRunNotCompleted   = errors.New("report run is not completed yet")
	ErrReportExportUnavailable = errors.New("report export is unavailable")
	reportedItemsLimit         = 20
)

// PeriodType selects the report cadence. The codes are part of the HTTP
// contract and the frontend copy registry.
type PeriodType string

const (
	PeriodDaily   PeriodType = "daily"
	PeriodWeekly  PeriodType = "weekly"
	PeriodMonthly PeriodType = "monthly"
)

func (period PeriodType) Valid() bool {
	switch period {
	case PeriodDaily, PeriodWeekly, PeriodMonthly:
		return true
	default:
		return false
	}
}

// DirectoryQueries is the identity-owned workspace directory port.
type DirectoryQueries interface {
	ListWorkspaceMembers(ctx context.Context, workspaceID uint) ([]identity.DirectoryMember, error)
	FindWorkspaceMember(ctx context.Context, workspaceID uint, factID uint) (identity.DirectoryMember, error)
	FindWorkspaceGroup(ctx context.Context, workspaceID uint, groupID uint) (identity.WorkspaceGroup, error)
	// FindWorkspaceDepartment resolves one synced Feishu department with its
	// subtree members so departments can be reported like groups.
	FindWorkspaceDepartment(ctx context.Context, workspaceID uint, departmentID uint) (identity.WorkspaceDepartment, error)
	// ListEnabledGroups enumerates every enabled 用户组 so the directory keeps
	// empty groups visible in the organisational hierarchy.
	ListEnabledGroups(ctx context.Context) ([]identity.UserGroup, error)
	// ListFeishuDepartments reads the synced Feishu org tree; empty until the
	// background contact sync has run at least once.
	ListFeishuDepartments(ctx context.Context) ([]identity.DirectoryDepartment, error)
}

// TaskFactsQueries is the task-owned BI read port.
type TaskFactsQueries interface {
	ListBIFacts(ctx context.Context, workspaceID uint) ([]task.Snapshot, []task.ActivitySnapshot, error)
}

// BugFactsQueries is the quality-owned BI read port.
type BugFactsQueries interface {
	ListBIFacts(ctx context.Context, workspaceID uint) ([]quality.BugSnapshot, []quality.BugActivitySnapshot, error)
}

// RequirementFactsQueries is the requirement-owned BI read port. It is
// optional: without it product-role reports render with zero requirement
// metrics instead of failing the whole report surface.
type RequirementFactsQueries interface {
	ListBIFacts(ctx context.Context, workspaceID uint) ([]requirement.Snapshot, error)
}

// RunStarter creates queued AI reporting runs through the ai domain.
type RunStarter interface {
	StartReportRun(ctx context.Context, command ai.ReportRunStartCommand) (ai.ReportRunStartResult, error)
}

// RunResultReader reads one reporting run of the workspace. The ai domain
// scopes every read by workspace and the report capability prefix.
type RunResultReader interface {
	FindReportRun(ctx context.Context, workspaceID uint, runID uint) (ai.ReportRunResult, error)
	// FindReportRunWithInput loads the run's input/output payloads for the
	// export path; polling must use the light FindReportRun.
	FindReportRunWithInput(ctx context.Context, workspaceID uint, runID uint) (ai.ReportRunResult, error)
	// FindLatestReportRun rediscovers the newest run of one exact view so
	// scheduled runs (which have no frontend receipt) surface automatically.
	FindLatestReportRun(ctx context.Context, query ai.ReportRunLatestQuery) (ai.ReportRunResult, error)
}

// Dependencies wires the report service to the module-owned read surfaces.
// Runs and RunResults are required so generation fails closed instead of
// silently degrading. Exporter and ExportRecords are optional: without
// Feishu credentials the publish endpoint fails closed with a dedicated
// error; without the record store it still publishes but skips the
// duplicate-document guard.
type Dependencies struct {
	Directory     DirectoryQueries
	Tasks         TaskFactsQueries
	Bugs          BugFactsQueries
	Runs          RunStarter
	RunResults    RunResultReader
	Exporter      DocumentExporter
	ExportRecords ai.ReportExportStore
	// Requirements is the optional requirement read port backing the product
	// role's 需求数量 facts. When nil those metrics stay zero (未接入).
	Requirements RequirementFactsQueries
	// Contributions is the optional code-contribution read port. When nil the
	// report renders exactly as before; when wired it adds the 代码活动
	// section from the workspace's own collected facts.
	Contributions ContributionFactsQueries
}

func (dependencies Dependencies) Validate() error {
	if dependencies.Directory == nil || dependencies.Tasks == nil || dependencies.Bugs == nil ||
		dependencies.Runs == nil || dependencies.RunResults == nil {
		return ErrReportUnavailable
	}
	return nil
}

// ContributionFactsQueries is the code-contribution read port. The
// code-contrib module owns persistence and collection; reporting only reads
// the derived daily aggregates, collection freshness, and the member-mapped
// commit facts that carry per-commit evidence to the AI.
type ContributionFactsQueries interface {
	ListDailyContributions(ctx context.Context, workspaceID uint) ([]DailyContributionFact, error)
	ListRepoSyncStates(ctx context.Context, workspaceID uint) ([]RepoSyncStatus, error)
	ListMemberCommits(ctx context.Context, workspaceID uint, since, until time.Time) ([]MemberCommitFact, error)
}

// MemberCommitFact is one member-attributed commit inside the report window,
// with its quality evidence (truncated diff sample + cheap hit counters) so
// the AI narrative can review every commit individually.
type MemberCommitFact struct {
	UserID      uint
	SHA         string
	CommittedAt time.Time
	Subject     string
	Additions   int
	Deletions   int
	Files       int
	DebugHits   int
	TodoHits    int
	DiffSample  string
}

// DailyContributionFact is one person×repo×day code-contribution bucket.
// UserID 0 is the unattributed bucket, which reporting excludes from person
// totals (未归属活动只在配置队列解释).
type DailyContributionFact struct {
	RepoID           uint
	UserID           uint
	AuthorKey        string
	Day              string
	Commits          int
	Additions        int
	Deletions        int
	PRsMerged        int
	ReviewsCompleted int
	// 行数构成（四类合计 = Additions）与"带评审合入"数：支撑代码活动的
	// 构成拆分与评审覆盖信号。历史聚合行四类为 0，仅总数可用。
	SourceAdditions int
	TestAdditions   int
	DocAdditions    int
	ConfigAdditions int
	PRsReviewed     int
}

// RepoSyncStatus carries collection freshness so a broken sync is never
// rendered as zero contribution (数据新鲜度红线).
type RepoSyncStatus struct {
	RepoID              uint
	LastSuccessAt       *time.Time
	LastError           string
	ConsecutiveFailures int
}

// Service composes the reporting use cases on top of the module ports.
type Service struct {
	dependencies Dependencies
	now          func() time.Time
}

func NewService(dependencies Dependencies) (*Service, error) {
	if err := dependencies.Validate(); err != nil {
		return nil, err
	}
	return &Service{dependencies: dependencies, now: time.Now}, nil
}

// MemberRef is the person reference rendered by reporting surfaces. ID is the
// platform fact ID that task and bug assignees key on. The department fields
// mirror the synced Feishu org projection and are display facts only.
type MemberRef struct {
	ID          uint     `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	AvatarURL   string   `json:"avatar_url,omitempty"`
	GroupName   string   `json:"group_name,omitempty"`
	RoleNames   []string `json:"role_names"`
	// JobTitle mirrors the synced Feishu job title (职务) so surfaces and the
	// AI chain can tell the 职务兜底 source of RoleNames apart from granted
	// permission roles. Optional HTTP field; existing consumers ignore it.
	JobTitle       string `json:"job_title,omitempty"`
	DepartmentID   string `json:"department_id,omitempty"`
	DepartmentName string `json:"department_name,omitempty"`
}

// DirectoryGroup is one role-group bucket of the workspace directory.
type DirectoryGroup struct {
	ID      uint        `json:"id"`
	Code    string      `json:"code,omitempty"`
	Name    string      `json:"name"`
	Members []MemberRef `json:"members"`
}

// DirectoryDepartment is one synced Feishu org-tree node. ID is the
// PM-internal row id the frontend uses as the reporting subject reference.
type DirectoryDepartment struct {
	ID          uint   `json:"id"`
	OpenID      string `json:"open_department_id"`
	ParentID    string `json:"parent_department_id"`
	Name        string `json:"name"`
	Order       int64  `json:"order"`
	MemberCount int    `json:"member_count"`
}

// DirectoryView is the personnel list grouped by role group. Members without
// an enabled group surface in Ungrouped so the list stays exhaustive. When
// the Feishu org sync has run, Departments carries the org tree and every
// member references its department so the frontend can render it instead.
type DirectoryView struct {
	Groups      []DirectoryGroup      `json:"groups"`
	Ungrouped   []MemberRef           `json:"ungrouped"`
	Total       int                   `json:"total"`
	Departments []DirectoryDepartment `json:"departments,omitempty"`
}

// MemberDirectory returns the workspace personnel grouped by role group.
// Every enabled group appears — including groups with zero members in this
// workspace — so the displayed hierarchy matches the 用户组 catalog instead
// of silently dropping empty organizations. The synced Feishu org tree rides
// along when available so the frontend can switch to department grouping;
// a departments read failure only degrades that garnish, never the read.
func (service *Service) MemberDirectory(ctx context.Context, workspaceID uint) (DirectoryView, error) {
	if workspaceID == 0 {
		return DirectoryView{}, ErrReportQueryInvalid
	}
	members, err := service.dependencies.Directory.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return DirectoryView{}, err
	}
	groups, err := service.dependencies.Directory.ListEnabledGroups(ctx)
	if err != nil {
		return DirectoryView{}, err
	}
	view := DirectoryView{Groups: []DirectoryGroup{}, Ungrouped: []MemberRef{}}
	groupIndex := map[uint]int{}
	for _, group := range groups {
		groupIndex[group.ID] = len(view.Groups)
		view.Groups = append(view.Groups, DirectoryGroup{
			ID: group.ID, Code: group.Code, Name: group.Name,
			Members: []MemberRef{},
		})
	}
	for _, member := range members {
		ref := memberRef(member)
		index, grouped := groupIndex[member.GroupID]
		if !grouped {
			view.Ungrouped = append(view.Ungrouped, ref)
			view.Total++
			continue
		}
		view.Groups[index].Members = append(view.Groups[index].Members, ref)
		view.Total++
	}
	if departments, err := service.dependencies.Directory.ListFeishuDepartments(ctx); err == nil && len(departments) > 0 {
		view.Departments = make([]DirectoryDepartment, 0, len(departments))
		for _, department := range departments {
			view.Departments = append(view.Departments, DirectoryDepartment{
				ID:     department.ID,
				OpenID: department.OpenID, ParentID: department.ParentID,
				Name: department.Name, Order: department.Order, MemberCount: department.MemberCount,
			})
		}
	}
	return view, nil
}

func memberRef(member identity.DirectoryMember) MemberRef {
	roleNames := member.RoleNames
	if roleNames == nil {
		roleNames = []string{}
	}
	return MemberRef{
		ID:             member.FactID,
		Username:       member.Username,
		DisplayName:    member.DisplayName,
		AvatarURL:      member.AvatarURL,
		GroupName:      member.GroupName,
		RoleNames:      roleNames,
		JobTitle:       member.JobTitle,
		DepartmentID:   member.DepartmentID,
		DepartmentName: member.DepartmentName,
	}
}
