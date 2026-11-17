ALTER TABLE pm_external_tooling_workspace_configurations
    DROP CONSTRAINT chk_external_tooling_cli_grant_ttl_hours;

-- 回滚到“天”会丢失小时精度；向上取整可避免回滚后缩短现有授权周期。
UPDATE pm_external_tooling_workspace_configurations
SET cli_grant_ttl_hours = LEAST(365, GREATEST(1, CEIL(cli_grant_ttl_hours / 24.0)::SMALLINT));

ALTER TABLE pm_external_tooling_workspace_configurations
    RENAME COLUMN cli_grant_ttl_hours TO cli_grant_ttl_days;

ALTER TABLE pm_external_tooling_workspace_configurations
    ALTER COLUMN cli_grant_ttl_days SET DEFAULT 7;

ALTER TABLE pm_external_tooling_workspace_configurations
    ADD CONSTRAINT chk_external_tooling_cli_grant_ttl_days
    CHECK (cli_grant_ttl_days BETWEEN 1 AND 365);
