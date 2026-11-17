-- Some databases advanced past 20260831120000 without creating this table.
-- Keep this repair idempotent so healthy environments remain unchanged.
CREATE TABLE IF NOT EXISTS project_participants (
    id                 BIGSERIAL PRIMARY KEY,
    workspace_id       BIGINT      NOT NULL,
    project_set_id     BIGINT      NOT NULL,
    user_id            BIGINT      NOT NULL,
    created_by_user_id BIGINT      NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ux_project_participants_project_user UNIQUE (project_set_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_participants_workspace
    ON project_participants (workspace_id);
CREATE INDEX IF NOT EXISTS idx_project_participants_project
    ON project_participants (project_set_id);
CREATE INDEX IF NOT EXISTS idx_project_participants_user
    ON project_participants (user_id);
