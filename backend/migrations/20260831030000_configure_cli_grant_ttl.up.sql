ALTER TABLE pm_external_tooling_workspace_configurations
    ADD COLUMN cli_grant_ttl_days SMALLINT NOT NULL DEFAULT 7;

ALTER TABLE pm_external_tooling_workspace_configurations
    ADD CONSTRAINT chk_external_tooling_cli_grant_ttl_days
    CHECK (cli_grant_ttl_days BETWEEN 1 AND 365);
