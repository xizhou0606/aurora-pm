package codecontrib

import (
	"context"
	"time"
)

// Storage ports. The gorm subpackage implements them; the admin service and
// the sync engine only see these interfaces so the pure core stays testable
// with in-memory fakes. All workspace-scoped operations take the workspace ID
// explicitly — tenancy isolation is a store contract, never a filter the
// caller can forget.

// Connection is one workspace's read-only credential for one SCM instance.
// Token stays populated only for internal collection paths; HTTP projections
// must go through MaskedToken and never serialize the secret.
type Connection struct {
	ID          uint      `json:"id"`
	WorkspaceID uint      `json:"workspace_id"`
	Name        string    `json:"name"`
	Provider    Provider  `json:"provider"`
	BaseURL     string    `json:"base_url,omitempty"`
	Token       string    `json:"-"`
	Enabled     bool      `json:"enabled"`
	CreatedBy   uint      `json:"created_by"`
	UpdatedBy   uint      `json:"updated_by"`
	Version     uint64    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MaskedToken renders the token for admin surfaces: a boolean plus the last
// four characters, so operators can tell which secret is configured without
// the value ever leaving the server.
func (connection Connection) MaskedToken() string {
	if connection.Token == "" {
		return ""
	}
	if len(connection.Token) <= 4 {
		return "••••"
	}
	return "••••" + connection.Token[len(connection.Token)-4:]
}

// RepositoryConfig binds one provider repository to a connection, with
// per-repository calibration overrides on top of the workspace defaults.
type RepositoryConfig struct {
	ID            uint        `json:"id"`
	WorkspaceID   uint        `json:"workspace_id"`
	ConnectionID  uint        `json:"connection_id"`
	ExternalID    string      `json:"external_id,omitempty"`
	Path          string      `json:"path"`
	DefaultBranch string      `json:"default_branch,omitempty"`
	FilterRules   FilterRules `json:"filter_rules"`
	ProjectSetID  *uint       `json:"project_set_id,omitempty"`
	Enabled       bool        `json:"enabled"`
	CreatedBy     uint        `json:"created_by"`
	UpdatedBy     uint        `json:"updated_by"`
	Version       uint64      `json:"version"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// CollectionConfig is the per-workspace statistics calibration (统计口径).
// 采集节奏 = 接入全量 + 每日/每周/每月三个独立触发时刻（工作区时区，精确到
// 分钟）；时区决定日/周/月边界，仅供高级部署调整。
type CollectionConfig struct {
	Timezone          string   `json:"timezone"`
	CoauthorPolicy    string   `json:"coauthor_policy"` // primary (v1) | split (reserved)
	BotPatterns       []string `json:"bot_patterns"`
	PageCap           int      `json:"page_cap"`
	DailySyncHour     int      `json:"daily_sync_hour"`
	DailySyncMinute   int      `json:"daily_sync_minute"`
	WeeklySyncWeekday int      `json:"weekly_sync_weekday"`
	WeeklySyncHour    int      `json:"weekly_sync_hour"`
	WeeklySyncMinute  int      `json:"weekly_sync_minute"`
	MonthlySyncDay    int      `json:"monthly_sync_day"`
	MonthlySyncHour   int      `json:"monthly_sync_hour"`
	MonthlySyncMinute int      `json:"monthly_sync_minute"`
	Version           uint64   `json:"version"`
	UpdatedBy         uint     `json:"updated_by"`
}

// BindingRow is one persisted provider-identity mapping row. UserID (when
// set) is the platform fact ID — the same ID task assignees key on.
type BindingRow struct {
	ID              uint          `json:"id"`
	WorkspaceID     uint          `json:"workspace_id"`
	Provider        Provider      `json:"provider"`
	KeyKind         string        `json:"key_kind"` // email | login
	AuthorKey       string        `json:"author_key"`
	DisplayName     string        `json:"display_name,omitempty"`
	UserID          *uint         `json:"user_id,omitempty"`
	SuggestedUserID *uint         `json:"suggested_user_id,omitempty"`
	Status          BindingStatus `json:"status"`
	MatchedVia      MatchVia      `json:"matched_via,omitempty"`
	ConfirmedBy     uint          `json:"confirmed_by"`
	ConfirmedAt     *time.Time    `json:"confirmed_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// DailyContributionRow is the person×repo×day aggregate the report module
// reads. UserID 0 is the unattributed bucket (未归属活动); AuthorKey disambiguates
// unattributed rows per raw identity so the confirm queue can explain them.
type DailyContributionRow struct {
	WorkspaceID      uint   `json:"workspace_id"`
	RepoID           uint   `json:"repo_id"`
	UserID           uint   `json:"user_id"`
	AuthorKey        string `json:"author_key"`
	Day              string `json:"day"`
	Commits          int    `json:"commits"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	PRsMerged        int    `json:"prs_merged"`
	ReviewsCompleted int    `json:"reviews_completed"`
	// 行数构成（四类合计 = Additions）与"带评审合入"数，支撑报表的构成
	// 拆分与评审覆盖信号。历史行四类为 0，仅总数可用。
	SourceAdditions int `json:"source_additions"`
	TestAdditions   int `json:"test_additions"`
	DocAdditions    int `json:"doc_additions"`
	ConfigAdditions int `json:"config_additions"`
	PRsReviewed     int `json:"prs_reviewed"`
}

// MemberCommitRow 是一条已归属到平台成员的提交事实,供报告把周期内全部
// 提交(含 diff 样本与质量命中)交给 AI 逐条点评。UserID 0 表示未归属,
// 报告侧按主体过滤后不会出现。
type MemberCommitRow struct {
	UserID      uint      `json:"user_id"`
	SHA         string    `json:"sha"`
	CommittedAt time.Time `json:"committed_at"`
	Subject     string    `json:"subject"`
	Additions   int       `json:"additions"`
	Deletions   int       `json:"deletions"`
	Files       int       `json:"files"`
	DebugHits   int       `json:"debug_hits"`
	TodoHits    int       `json:"todo_hits"`
	DiffSample  string    `json:"diff_sample,omitempty"`
}

// RepoSyncState distinguishes "no activity" from "collection broken" (数据
// 新鲜度): the report surface must never render a failed collection as zero
// contribution.
type RepoSyncState struct {
	RepoID              uint       `json:"repo_id"`
	WorkspaceID         uint       `json:"workspace_id"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	// FullSyncedAt 是接入全量拉取完成的时间（nil=尚未全量，下轮必须先
	// 补全量）；此后采集走每日/每周/每月定时自然窗。
	FullSyncedAt *time.Time `json:"full_synced_at,omitempty"`
}

// ConnectionStore persists workspace connections.
type ConnectionStore interface {
	ListConnections(ctx context.Context, workspaceID uint) ([]Connection, error)
	GetConnection(ctx context.Context, workspaceID, id uint) (Connection, error)
	CreateConnection(ctx context.Context, connection Connection) (Connection, error)
	UpdateConnection(ctx context.Context, connection Connection) (Connection, error)
	DeleteConnection(ctx context.Context, workspaceID, id uint) error
}

// RepositoryStore persists repository bindings.
type RepositoryStore interface {
	ListRepositories(ctx context.Context, workspaceID uint) ([]RepositoryConfig, error)
	ListEnabledRepositories(ctx context.Context) ([]RepositoryConfig, error)
	GetRepository(ctx context.Context, workspaceID, id uint) (RepositoryConfig, error)
	CreateRepository(ctx context.Context, repository RepositoryConfig) (RepositoryConfig, error)
	UpdateRepository(ctx context.Context, repository RepositoryConfig) (RepositoryConfig, error)
	DeleteRepository(ctx context.Context, workspaceID, id uint) error
}

// SettingsStore persists the workspace calibration. Get returns exists=false
// when the workspace never saved one; callers then apply the defaults.
type SettingsStore interface {
	GetSettings(ctx context.Context, workspaceID uint) (CollectionConfig, bool, error)
	SaveSettings(ctx context.Context, workspaceID uint, config CollectionConfig) (CollectionConfig, error)
}

// BindingStore persists provider-identity mappings. UpsertBinding must never
// downgrade an admin-confirmed row: re-collection replays old identities all
// the time, and 管理员确认是最高优先级.
type BindingStore interface {
	UpsertBinding(ctx context.Context, row BindingRow) error
	ListBindings(ctx context.Context, workspaceID uint, statuses []BindingStatus, limit int) ([]BindingRow, error)
	ConfirmBinding(ctx context.Context, workspaceID, id, userID, actorID uint, at time.Time) (BindingRow, error)
	RejectBinding(ctx context.Context, workspaceID, id, actorID uint) (BindingRow, error)
	ListBindingRows(ctx context.Context, workspaceID uint) ([]AuthorBinding, error)
}

// FactStore persists collection output and the derived daily aggregates.
type FactStore interface {
	UpsertCommits(ctx context.Context, workspaceID, repoID uint, facts []CommitFact) error
	UpsertPullRequests(ctx context.Context, workspaceID, repoID uint, facts []PullRequestFact) error
	ReplaceDailyWindow(ctx context.Context, workspaceID, repoID uint, days []string, rows []DailyContributionRow) error
	ListDailyContributions(ctx context.Context, workspaceID uint) ([]DailyContributionRow, error)
	ListCommitsInWindow(ctx context.Context, workspaceID, repoID uint, window Window) ([]CommitFact, error)
	ListMergedPullRequestsInWindow(ctx context.Context, workspaceID, repoID uint, window Window) ([]PullRequestFact, error)
	GetSyncRun(ctx context.Context, repoID uint) (RepoSyncState, bool, error)
	SaveSyncRun(ctx context.Context, state RepoSyncState) error
	ListRepoSyncStates(ctx context.Context, workspaceID uint) ([]RepoSyncState, error)
}

// MemberCandidate is one workspace member's matching surface, keyed by the
// platform fact ID. Email and Username are comparison facts only.
type MemberCandidate struct {
	FactID   uint
	Username string
	Email    string
}

// MemberCandidateSource supplies the binding pipeline's user candidates. The
// identity module implements it at assembly time; this package does not import
// identity directly.
type MemberCandidateSource interface {
	FindActiveMemberCandidates(ctx context.Context, workspaceID uint) ([]MemberCandidate, error)
}

// BindingContextFor adapts stored rows and workspace members into the pure
// pipeline's input, keeping the resolution logic independent of storage.
func BindingContextFor(config CollectionConfig, rows []AuthorBinding, members []MemberCandidate) BindingContext {
	bindings := make([]AuthorBinding, 0, len(rows))
	for _, row := range rows {
		if row.UserID == 0 {
			continue
		}
		bindings = append(bindings, row)
	}
	candidates := make([]UserCandidate, 0, len(members))
	for _, member := range members {
		candidates = append(candidates, UserCandidate{UserID: member.FactID, Email: member.Email, Username: member.Username})
	}
	return BindingContext{
		BotPatterns:     config.BotPatterns,
		Bindings:        bindings,
		Candidates:      candidates,
		AllowLoginExact: false,
	}
}
