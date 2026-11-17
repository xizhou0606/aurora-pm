-- Reconcile the 20260918000000 authorization/SCM version collision.
-- Existing public history identifiers and confirmation codes must remain unchanged.
ALTER TABLE pm_external_device_authorizations ADD COLUMN IF NOT EXISTS record_id varchar(128);
ALTER TABLE pm_external_device_authorizations ADD COLUMN IF NOT EXISTS user_code varchar(16);
UPDATE pm_external_device_authorizations
SET record_id = 'd_' || md5(random()::text || device_code_digest)
WHERE record_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_pm_external_device_authorizations_record_id ON pm_external_device_authorizations(record_id);
CREATE INDEX IF NOT EXISTS idx_tool_device_history ON pm_external_device_authorizations(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tool_request_history ON pm_external_authorization_requests(workspace_id, created_at DESC);
