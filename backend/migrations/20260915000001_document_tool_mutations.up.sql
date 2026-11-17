CREATE TABLE IF NOT EXISTS pm_document_tool_mutations (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL,
    apply_key VARCHAR(128) NOT NULL,
    actor_user_id BIGINT NOT NULL,
    input_digest VARCHAR(64) NOT NULL,
    result_json TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ux_document_tool_mutation UNIQUE (workspace_id, apply_key)
);
