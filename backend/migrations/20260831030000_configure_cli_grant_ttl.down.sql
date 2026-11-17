ALTER TABLE pm_external_tooling_workspace_configurations
    DROP CONSTRAINT chk_external_tooling_cli_grant_ttl_days;

ALTER TABLE pm_external_tooling_workspace_configurations
    DROP COLUMN cli_grant_ttl_days;
