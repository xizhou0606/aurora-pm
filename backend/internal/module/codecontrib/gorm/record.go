// Package codecontribgorm contains the persistence for the code-contribution
// domain: admin-managed configuration (connections, repositories, bindings,
// collection settings) and collector-written facts (commits, pull requests,
// daily aggregates, sync runs).
//
// 数据安全红线：Connection.Token 只允许被采集器与测试连接流程读取，任何列表
// 或导出路径都必须走脱敏投影；凭证永不写日志。
package codecontribgorm

import "time"

// Connection is one workspace's read-only credential for one SCM instance.
type Connection struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_scm_connections_workspace_name,priority:1" json:"workspace_id"`
	Name        string    `gorm:"size:128;not null;uniqueIndex:ux_scm_connections_workspace_name,priority:2" json:"name"`
	Provider    string    `gorm:"size:16;not null" json:"provider"`
	BaseURL     string    `gorm:"size:512;not null;default:''" json:"base_url,omitempty"`
	Token       string    `gorm:"size:512;not null" json:"-"`
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedBy   uint      `gorm:"not null;default:0" json:"created_by"`
	UpdatedBy   uint      `gorm:"not null;default:0" json:"updated_by"`
	Version     uint64    `gorm:"not null;default:1" json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Connection) TableName() string { return "scm_connections" }

// Repository binds one provider repository to a workspace connection.
type Repository struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint      `gorm:"not null;uniqueIndex:ux_scm_repositories_connection_path,priority:1" json:"workspace_id"`
	ConnectionID    uint      `gorm:"not null;uniqueIndex:ux_scm_repositories_connection_path,priority:2" json:"connection_id"`
	ExternalID      string    `gorm:"size:64;not null;default:''" json:"external_id,omitempty"`
	Path            string    `gorm:"size:512;not null;uniqueIndex:ux_scm_repositories_connection_path,priority:3" json:"path"`
	DefaultBranch   string    `gorm:"size:255;not null;default:''" json:"default_branch,omitempty"`
	FilterRulesJSON string    `gorm:"column:filter_rules_json;type:text;not null;default:'{}'" json:"filter_rules_json,omitempty"`
	ProjectSetID    *uint     `gorm:"" json:"project_set_id,omitempty"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedBy       uint      `gorm:"not null;default:0" json:"created_by"`
	UpdatedBy       uint      `gorm:"not null;default:0" json:"updated_by"`
	Version         uint64    `gorm:"not null;default:1" json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Repository) TableName() string { return "scm_repositories" }

// CollectionSetting is the per-workspace statistics calibration (口径).
type CollectionSetting struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID       uint      `gorm:"not null;uniqueIndex:ux_scm_collection_settings_workspace" json:"workspace_id"`
	Timezone          string    `gorm:"size:64;not null;default:'Asia/Shanghai'" json:"timezone"`
	CoauthorPolicy    string    `gorm:"size:16;not null;default:'primary'" json:"coauthor_policy"`
	BotPatternsJSON   string    `gorm:"column:bot_patterns_json;type:text;not null;default:'[]'" json:"bot_patterns_json"`
	PageCap           int       `gorm:"not null;default:40" json:"page_cap"`
	DailySyncHour     int       `gorm:"not null;default:9" json:"daily_sync_hour"`
	DailySyncMinute   int       `gorm:"not null;default:0" json:"daily_sync_minute"`
	WeeklySyncWeekday int       `gorm:"not null;default:1" json:"weekly_sync_weekday"`
	WeeklySyncHour    int       `gorm:"not null;default:9" json:"weekly_sync_hour"`
	WeeklySyncMinute  int       `gorm:"not null;default:0" json:"weekly_sync_minute"`
	MonthlySyncDay    int       `gorm:"not null;default:1" json:"monthly_sync_day"`
	MonthlySyncHour   int       `gorm:"not null;default:9" json:"monthly_sync_hour"`
	MonthlySyncMinute int       `gorm:"not null;default:0" json:"monthly_sync_minute"`
	Version           uint64    `gorm:"not null;default:1" json:"version"`
	UpdatedBy         uint      `gorm:"not null;default:0" json:"updated_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (CollectionSetting) TableName() string { return "scm_collection_settings" }

// AuthorBinding is one provider identity's binding state. user_id stores the
// platform fact ID (the same ID task assignees key on); a row with user_id
// NULL and status suggested/unmatched belongs to the confirm queue.
type AuthorBinding struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint       `gorm:"not null;uniqueIndex:ux_scm_author_bindings_key,priority:1" json:"workspace_id"`
	Provider        string     `gorm:"size:16;not null;uniqueIndex:ux_scm_author_bindings_key,priority:2" json:"provider"`
	KeyKind         string     `gorm:"size:8;not null;uniqueIndex:ux_scm_author_bindings_key,priority:3" json:"key_kind"`
	AuthorKey       string     `gorm:"size:255;not null;uniqueIndex:ux_scm_author_bindings_key,priority:4" json:"author_key"`
	DisplayName     string     `gorm:"size:255;not null;default:''" json:"display_name,omitempty"`
	UserID          *uint      `gorm:"" json:"user_id,omitempty"`
	SuggestedUserID *uint      `gorm:"" json:"suggested_user_id,omitempty"`
	Status          string     `gorm:"size:16;not null;default:'unmatched'" json:"status"`
	MatchedVia      string     `gorm:"size:24;not null;default:''" json:"matched_via,omitempty"`
	ConfirmedBy     uint       `gorm:"not null;default:0" json:"confirmed_by"`
	ConfirmedAt     *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (AuthorBinding) TableName() string { return "scm_author_bindings" }

// Commit is one collected commit's immutable fact row.
type Commit struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID        uint      `gorm:"not null;index:idx_scm_commits_workspace_committed,priority:1" json:"workspace_id"`
	RepoID             uint      `gorm:"not null;uniqueIndex:ux_scm_commits_repo_sha,priority:1" json:"repo_id"`
	SHA                string    `gorm:"size:64;not null;uniqueIndex:ux_scm_commits_repo_sha,priority:2" json:"sha"`
	AuthorEmail        string    `gorm:"size:255;not null;default:''" json:"author_email,omitempty"`
	AuthorName         string    `gorm:"size:255;not null;default:''" json:"author_name,omitempty"`
	AuthorLogin        string    `gorm:"size:128;not null;default:''" json:"author_login,omitempty"`
	CommitterEmail     string    `gorm:"size:255;not null;default:''" json:"committer_email,omitempty"`
	CoAuthorEmailsJSON string    `gorm:"column:co_author_emails_json;type:text;not null;default:'[]'" json:"co_author_emails_json,omitempty"`
	AuthorAt           time.Time `gorm:"not null" json:"author_at"`
	CommittedAt        time.Time `gorm:"not null;index:idx_scm_commits_workspace_committed,priority:2" json:"committed_at"`
	Subject            string    `gorm:"size:512;not null;default:''" json:"subject,omitempty"`
	Additions          int       `gorm:"not null;default:0" json:"additions"`
	Deletions          int       `gorm:"not null;default:0" json:"deletions"`
	IsMerge            bool      `gorm:"not null;default:false" json:"is_merge"`
	PRDelivered        bool      `gorm:"column:pr_delivered;not null;default:false" json:"pr_delivered"`
	PRNumber           int       `gorm:"column:pr_number;not null;default:0" json:"pr_number,omitempty"`
	FilesJSON          string    `gorm:"column:files_json;type:text;not null;default:'[]'" json:"files_json,omitempty"`
	// 逐提交质量证据：diff_sample 保留新增行内容（截断），报告把它交给 AI
	// 逐提交点评；debug/todo 是采集时的廉价命中计数（质量扣分依据）。
	DiffSample string    `gorm:"column:diff_sample;type:text;not null;default:''" json:"diff_sample,omitempty"`
	DebugHits  int       `gorm:"not null;default:0" json:"debug_hits"`
	TodoHits   int       `gorm:"not null;default:0" json:"todo_hits"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Commit) TableName() string { return "scm_commits" }

// PullRequest is one collected pull/merge request's fact row.
type PullRequest struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint       `gorm:"not null;index:idx_scm_pull_requests_workspace_merged,priority:1" json:"workspace_id"`
	RepoID         uint       `gorm:"not null;uniqueIndex:ux_scm_pull_requests_repo_number,priority:1" json:"repo_id"`
	Number         int64      `gorm:"not null;uniqueIndex:ux_scm_pull_requests_repo_number,priority:2" json:"number"`
	Title          string     `gorm:"size:512;not null;default:''" json:"title,omitempty"`
	AuthorLogin    string     `gorm:"size:128;not null;default:''" json:"author_login,omitempty"`
	AuthorEmail    string     `gorm:"size:255;not null;default:''" json:"author_email,omitempty"`
	CreatedAt      time.Time  `gorm:"not null" json:"created_at"`
	MergedAt       *time.Time `gorm:"index:idx_scm_pull_requests_workspace_merged,priority:2" json:"merged_at,omitempty"`
	MergeSHA       string     `gorm:"size:64;not null;default:''" json:"merge_sha,omitempty"`
	BaseBranch     string     `gorm:"size:255;not null;default:''" json:"base_branch,omitempty"`
	Additions      int        `gorm:"not null;default:0" json:"additions"`
	Deletions      int        `gorm:"not null;default:0" json:"deletions"`
	CommitSHAsJSON string     `gorm:"column:commit_shas_json;type:text;not null;default:'[]'" json:"commit_shas_json,omitempty"`
	ReviewsJSON    string     `gorm:"column:reviews_json;type:text;not null;default:'[]'" json:"reviews_json,omitempty"`
	FilesJSON      string     `gorm:"column:files_json;type:text;not null;default:'[]'" json:"files_json,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (PullRequest) TableName() string { return "scm_pull_requests" }

// DailyContribution is the person×repo×day aggregate the report module reads.
// user_id 0 is the unattributed bucket (未归属活动). Additions 同时按工作类型
// 落列（源码/测试/文档/配置，合计等于 additions）；prs_reviewed 是"带评审
// 合入"的请求数，评审覆盖 = prs_reviewed / prs_merged。
type DailyContribution struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID      uint      `gorm:"not null;uniqueIndex:ux_scm_daily_contributions_bucket,priority:1" json:"workspace_id"`
	RepoID           uint      `gorm:"not null;uniqueIndex:ux_scm_daily_contributions_bucket,priority:2" json:"repo_id"`
	UserID           uint      `gorm:"not null;default:0;uniqueIndex:ux_scm_daily_contributions_bucket,priority:3" json:"user_id"`
	AuthorKey        string    `gorm:"size:255;not null;default:'';uniqueIndex:ux_scm_daily_contributions_bucket,priority:4" json:"author_key"`
	Day              string    `gorm:"size:10;not null;uniqueIndex:ux_scm_daily_contributions_bucket,priority:5" json:"day"`
	Commits          int       `gorm:"not null;default:0" json:"commits"`
	Additions        int       `gorm:"not null;default:0" json:"additions"`
	Deletions        int       `gorm:"not null;default:0" json:"deletions"`
	PRsMerged        int       `gorm:"column:prs_merged;not null;default:0" json:"prs_merged"`
	ReviewsCompleted int       `gorm:"not null;default:0" json:"reviews_completed"`
	SourceAdditions  int       `gorm:"not null;default:0" json:"source_additions"`
	TestAdditions    int       `gorm:"not null;default:0" json:"test_additions"`
	DocAdditions     int       `gorm:"not null;default:0" json:"doc_additions"`
	ConfigAdditions  int       `gorm:"not null;default:0" json:"config_additions"`
	PRsReviewed      int       `gorm:"column:prs_reviewed;not null;default:0" json:"prs_reviewed"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (DailyContribution) TableName() string { return "scm_daily_contributions" }

// SyncRun tracks per-repo collection watermark and failure state so the
// report surface can distinguish "no activity" from "collection broken".
type SyncRun struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID         uint       `gorm:"not null" json:"workspace_id"`
	RepoID              uint       `gorm:"not null;uniqueIndex:ux_scm_sync_runs_repo" json:"repo_id"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastError           string     `gorm:"size:1024;not null;default:''" json:"last_error,omitempty"`
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutive_failures"`
	// FullSyncedAt 是接入全量拉取完成的时间（nil=尚未全量，下轮必须先
	// 补全量）；此后采集走每日/每周/每月 1 号 09:00 定时自然窗。
	FullSyncedAt *time.Time `json:"full_synced_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (SyncRun) TableName() string { return "scm_sync_runs" }

// MigrationRecords lists every model for the sqlite/dev AutoMigrate path.
func MigrationRecords() []any {
	return []any{
		&Connection{},
		&Repository{},
		&CollectionSetting{},
		&AuthorBinding{},
		&Commit{},
		&PullRequest{},
		&DailyContribution{},
		&SyncRun{},
	}
}
