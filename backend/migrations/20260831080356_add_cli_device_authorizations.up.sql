-- CLI 设备授权事务只保存设备码和用户码摘要；Token 继续存入既有凭证摘要表。
CREATE TABLE pm_external_device_authorizations (
    device_code_digest VARCHAR(64) PRIMARY KEY,
    user_code_digest VARCHAR(64) NOT NULL,
    workspace_id BIGINT NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    channel VARCHAR(16) NOT NULL,
    audience VARCHAR(32) NOT NULL,
    resource VARCHAR(2048) NOT NULL,
    scope VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL,
    authorization_request_id VARCHAR(128),
    principal_id BIGINT,
    grant_id BIGINT,
    expires_at TIMESTAMPTZ NOT NULL,
    poll_interval_seconds SMALLINT NOT NULL,
    last_polled_at TIMESTAMPTZ,
    decided_at TIMESTAMPTZ,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ux_tool_device_user_code UNIQUE (user_code_digest),
    CONSTRAINT chk_tool_device_cli_binding CHECK (
        client_id = 'pm-cli' AND channel = 'cli' AND audience = 'pm-tool-cli'
    ),
    CONSTRAINT chk_tool_device_status CHECK (
        status IN ('pending', 'approving', 'approved', 'denied', 'consumed')
    ),
    CONSTRAINT chk_tool_device_decision_binding CHECK (
        (status = 'pending' AND decided_at IS NULL AND authorization_request_id IS NULL AND principal_id IS NULL AND grant_id IS NULL AND consumed_at IS NULL)
        OR (status IN ('approving', 'denied') AND decided_at IS NOT NULL AND authorization_request_id IS NULL AND principal_id IS NULL AND grant_id IS NULL AND consumed_at IS NULL)
        OR (status = 'approved' AND decided_at IS NOT NULL AND authorization_request_id IS NOT NULL AND principal_id IS NOT NULL AND grant_id IS NOT NULL AND consumed_at IS NULL)
        OR (status = 'consumed' AND decided_at IS NOT NULL AND authorization_request_id IS NOT NULL AND principal_id IS NOT NULL AND grant_id IS NOT NULL AND consumed_at IS NOT NULL)
    ),
    CONSTRAINT chk_tool_device_poll_interval CHECK (poll_interval_seconds BETWEEN 1 AND 60)
);

CREATE INDEX idx_tool_device_workspace_status
    ON pm_external_device_authorizations (workspace_id, status);
CREATE INDEX idx_tool_device_expiry
    ON pm_external_device_authorizations (expires_at);
CREATE INDEX idx_tool_device_authorization_request
    ON pm_external_device_authorizations (authorization_request_id);
CREATE INDEX idx_tool_device_principal
    ON pm_external_device_authorizations (principal_id);
CREATE INDEX idx_tool_device_grant
    ON pm_external_device_authorizations (grant_id);
