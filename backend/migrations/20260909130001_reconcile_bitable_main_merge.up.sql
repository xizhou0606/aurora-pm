-- Compatibility bridge for feature/table databases already past main's migration versions.
-- Main source migrations remain unchanged. Only additive, idempotent SQL is replayed.
-- Version exception approved for the existing future-dated 20260909130000 marker.

-- Source: 20260907070735_add_feishu_org_structure.up.sql
CREATE TABLE IF NOT EXISTS feishu_departments (
    id BIGSERIAL PRIMARY KEY,
    open_department_id VARCHAR(64) NOT NULL,
    parent_department_id VARCHAR(64) NOT NULL DEFAULT '',
    name VARCHAR(255) NOT NULL,
    dept_order BIGINT NOT NULL DEFAULT 0,
    member_count INT NOT NULL DEFAULT 0,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_feishu_departments_open_id
    ON feishu_departments (open_department_id);
CREATE INDEX IF NOT EXISTS idx_feishu_departments_parent
    ON feishu_departments (parent_department_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS feishu_department_id VARCHAR(64) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_users_feishu_department_id ON users (feishu_department_id);

-- Source: 20260907120000_add_scm_code_contrib.up.sql
-- SCM code contribution domain: admin-managed configuration tables plus
-- collector-written fact tables. All tables are workspace-scoped. The admin
-- CRUD tables hold credentials and mappings, the fact tables hold immutable
-- collection output and per-day aggregates for the report module.

CREATE TABLE IF NOT EXISTS scm_connections (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    token TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT NOT NULL DEFAULT 0,
    updated_by BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_connections_workspace_name
    ON scm_connections (workspace_id, name);

CREATE TABLE IF NOT EXISTS scm_repositories (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    connection_id BIGINT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT '',
    filter_rules_json TEXT NOT NULL DEFAULT '{}',
    project_set_id BIGINT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT NOT NULL DEFAULT 0,
    updated_by BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_repositories_connection_path
    ON scm_repositories (workspace_id, connection_id, path);
CREATE INDEX IF NOT EXISTS idx_scm_repositories_workspace
    ON scm_repositories (workspace_id);

CREATE TABLE IF NOT EXISTS scm_collection_settings (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai',
    sync_interval_minutes INT NOT NULL DEFAULT 30,
    overlap_days INT NOT NULL DEFAULT 7,
    coauthor_policy TEXT NOT NULL DEFAULT 'primary',
    bot_patterns_json TEXT NOT NULL DEFAULT '[]',
    page_cap INT NOT NULL DEFAULT 40,
    version BIGINT NOT NULL DEFAULT 1,
    updated_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_collection_settings_workspace
    ON scm_collection_settings (workspace_id);

CREATE TABLE IF NOT EXISTS scm_author_bindings (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    provider TEXT NOT NULL,
    key_kind TEXT NOT NULL,
    author_key TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    user_id BIGINT,
    suggested_user_id BIGINT,
    status TEXT NOT NULL DEFAULT 'unmatched',
    matched_via TEXT NOT NULL DEFAULT '',
    confirmed_by BIGINT NOT NULL DEFAULT 0,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_author_bindings_key
    ON scm_author_bindings (workspace_id, provider, key_kind, author_key);
CREATE INDEX IF NOT EXISTS idx_scm_author_bindings_status
    ON scm_author_bindings (workspace_id, status);

CREATE TABLE IF NOT EXISTS scm_commits (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    repo_id BIGINT NOT NULL,
    sha TEXT NOT NULL,
    author_email TEXT NOT NULL DEFAULT '',
    author_name TEXT NOT NULL DEFAULT '',
    author_login TEXT NOT NULL DEFAULT '',
    committer_email TEXT NOT NULL DEFAULT '',
    co_author_emails_json TEXT NOT NULL DEFAULT '[]',
    author_at TIMESTAMPTZ NOT NULL,
    committed_at TIMESTAMPTZ NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    additions INT NOT NULL DEFAULT 0,
    deletions INT NOT NULL DEFAULT 0,
    is_merge BOOLEAN NOT NULL DEFAULT FALSE,
    pr_delivered BOOLEAN NOT NULL DEFAULT FALSE,
    pr_number INT NOT NULL DEFAULT 0,
    files_json TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_commits_repo_sha
    ON scm_commits (repo_id, sha);
CREATE INDEX IF NOT EXISTS idx_scm_commits_workspace_committed
    ON scm_commits (workspace_id, committed_at);

CREATE TABLE IF NOT EXISTS scm_pull_requests (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    repo_id BIGINT NOT NULL,
    number BIGINT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    author_login TEXT NOT NULL DEFAULT '',
    author_email TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    merged_at TIMESTAMPTZ,
    merge_sha TEXT NOT NULL DEFAULT '',
    base_branch TEXT NOT NULL DEFAULT '',
    additions INT NOT NULL DEFAULT 0,
    deletions INT NOT NULL DEFAULT 0,
    commit_shas_json TEXT NOT NULL DEFAULT '[]',
    reviews_json TEXT NOT NULL DEFAULT '[]',
    files_json TEXT NOT NULL DEFAULT '[]',
    -- created_at 即 provider 的请求创建时间（上方列），这里只补审计更新时间。
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_pull_requests_repo_number
    ON scm_pull_requests (repo_id, number);
CREATE INDEX IF NOT EXISTS idx_scm_pull_requests_workspace_merged
    ON scm_pull_requests (workspace_id, merged_at);

CREATE TABLE IF NOT EXISTS scm_daily_contributions (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    repo_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL DEFAULT 0,
    author_key TEXT NOT NULL DEFAULT '',
    day TEXT NOT NULL,
    commits INT NOT NULL DEFAULT 0,
    additions INT NOT NULL DEFAULT 0,
    deletions INT NOT NULL DEFAULT 0,
    prs_merged INT NOT NULL DEFAULT 0,
    reviews_completed INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_daily_contributions_bucket
    ON scm_daily_contributions (workspace_id, repo_id, user_id, author_key, day);
CREATE INDEX IF NOT EXISTS idx_scm_daily_contributions_workspace_day
    ON scm_daily_contributions (workspace_id, day);

CREATE TABLE IF NOT EXISTS scm_sync_runs (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    repo_id BIGINT NOT NULL,
    last_synced_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    consecutive_failures INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_scm_sync_runs_repo
    ON scm_sync_runs (repo_id);

-- Source: 20260908120000_add_scm_daily_line_breakdown.up.sql
-- 代码活动构成拆分（A）与评审覆盖信号（B）：
-- 行数按工作类型（源码/测试/文档/配置）落列，合入 PR 记录"带评审合入"数。
-- 历史行保持 0（仅总数），新窗口同步后按日重写补齐。
ALTER TABLE scm_daily_contributions
    ADD COLUMN IF NOT EXISTS source_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS test_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS doc_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS config_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS prs_reviewed INT NOT NULL DEFAULT 0;

-- Source: 20260908130000_add_scm_commit_diff_sample.up.sql
-- 逐提交 AI 质量分析的数据底座：
-- diff_sample 保留新增行内容（截断），供报告把每次提交交给 AI 点评；
-- debug_hits/todo_hits 是采集时的廉价命中计数，构成可审计的质量扣分。
ALTER TABLE scm_commits
    ADD COLUMN IF NOT EXISTS diff_sample TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS debug_hits INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS todo_hits INT NOT NULL DEFAULT 0;

-- Source: 20260908200000_add_users_feishu_job_title.up.sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS feishu_job_title TEXT NOT NULL DEFAULT '';
