-- Public history identifiers and non-secret user confirmation codes only.
-- Previously digested confirmation codes cannot be recovered and remain NULL.
ALTER TABLE pm_external_device_authorizations ADD COLUMN record_id varchar(128);
ALTER TABLE pm_external_device_authorizations ADD COLUMN user_code varchar(16);
UPDATE pm_external_device_authorizations SET record_id = 'd_' || md5(random()::text || device_code_digest);
CREATE UNIQUE INDEX idx_pm_external_device_authorizations_record_id ON pm_external_device_authorizations(record_id);
CREATE INDEX idx_tool_device_history ON pm_external_device_authorizations(workspace_id, created_at DESC);
CREATE INDEX idx_tool_request_history ON pm_external_authorization_requests(workspace_id, created_at DESC);
