-- A default workspace is an authorization baseline: it must be enabled and
-- there must never be more than one. Prefer the workspace already serving the
-- most members when repairing legacy duplicates.
UPDATE workspaces
SET enabled = TRUE
WHERE is_default = TRUE
  AND enabled = FALSE;

WITH candidate AS (
    SELECT w.id
    FROM workspaces AS w
    WHERE w.enabled = TRUE
    ORDER BY
        (SELECT COUNT(*) FROM workspace_members AS wm WHERE wm.workspace_id = w.id) DESC,
        w.created_at ASC,
        w.id ASC
    LIMIT 1
)
UPDATE workspaces
SET is_default = TRUE
WHERE id = (SELECT id FROM candidate)
  AND NOT EXISTS (SELECT 1 FROM workspaces WHERE is_default = TRUE);

WITH ranked_defaults AS (
    SELECT
        w.id,
        ROW_NUMBER() OVER (
            ORDER BY
                w.enabled DESC,
                (SELECT COUNT(*) FROM workspace_members AS wm WHERE wm.workspace_id = w.id) DESC,
                w.created_at ASC,
                w.id ASC
        ) AS position
    FROM workspaces AS w
    WHERE w.is_default = TRUE
)
UPDATE workspaces
SET is_default = FALSE
WHERE id IN (
    SELECT id
    FROM ranked_defaults
    WHERE position > 1
);

ALTER TABLE workspaces
    ADD CONSTRAINT ck_workspaces_default_enabled
    CHECK (NOT is_default OR enabled);

CREATE UNIQUE INDEX ux_workspaces_single_default
    ON workspaces (is_default)
    WHERE is_default = TRUE;
