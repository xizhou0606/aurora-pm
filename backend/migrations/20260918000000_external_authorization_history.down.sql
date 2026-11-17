DROP INDEX IF EXISTS idx_tool_request_history;
DROP INDEX IF EXISTS idx_tool_device_history;
ALTER TABLE pm_external_device_authorizations DROP COLUMN user_code;
ALTER TABLE pm_external_device_authorizations DROP COLUMN record_id;
