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
