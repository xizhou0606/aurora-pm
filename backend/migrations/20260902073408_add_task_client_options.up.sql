CREATE TABLE IF NOT EXISTS task_client_options (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    name VARCHAR(64) NOT NULL,
    normalized_name VARCHAR(64) NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_task_client_option_scope
    ON task_client_options (workspace_id, normalized_name);
CREATE INDEX IF NOT EXISTS idx_task_client_options_workspace_id
    ON task_client_options (workspace_id);
CREATE INDEX IF NOT EXISTS idx_task_client_options_created_by_user_id
    ON task_client_options (created_by_user_id);

ALTER TABLE work_tasks ADD COLUMN IF NOT EXISTS task_client_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_work_tasks_task_client_id ON work_tasks (task_client_id);

INSERT INTO task_client_options (workspace_id, name, normalized_name, created_by_user_id)
SELECT workspace.id, defaults.name, defaults.normalized_name, COALESCE(
    (SELECT member.user_id FROM workspace_members member
     WHERE member.workspace_id = workspace.id AND member.enabled = TRUE
     ORDER BY member.id ASC LIMIT 1),
    0
)
FROM workspaces workspace
CROSS JOIN (VALUES
    ('桌面端', '桌面端'),
    ('H5', 'h5'),
    ('App', 'app')
) AS defaults(name, normalized_name)
ON CONFLICT (workspace_id, normalized_name) DO NOTHING;
