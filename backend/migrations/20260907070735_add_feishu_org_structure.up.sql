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
