DROP INDEX IF EXISTS ux_workspaces_single_default;

ALTER TABLE workspaces
    DROP CONSTRAINT IF EXISTS ck_workspaces_default_enabled;

-- The up migration deterministically normalizes duplicate or missing default
-- flags. Those historical flag values cannot be reconstructed safely, so the
-- down migration removes only the schema constraints.
