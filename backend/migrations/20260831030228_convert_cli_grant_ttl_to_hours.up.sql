ALTER TABLE pm_external_tooling_workspace_configurations
    DROP CONSTRAINT chk_external_tooling_cli_grant_ttl_days;

ALTER TABLE pm_external_tooling_workspace_configurations
    RENAME COLUMN cli_grant_ttl_days TO cli_grant_ttl_hours;

UPDATE pm_external_tooling_workspace_configurations
SET cli_grant_ttl_hours = cli_grant_ttl_hours * 24;

ALTER TABLE pm_external_tooling_workspace_configurations
    ALTER COLUMN cli_grant_ttl_hours SET DEFAULT 168;

ALTER TABLE pm_external_tooling_workspace_configurations
    ADD CONSTRAINT chk_external_tooling_cli_grant_ttl_hours
    CHECK (cli_grant_ttl_hours BETWEEN 1 AND 8760);
