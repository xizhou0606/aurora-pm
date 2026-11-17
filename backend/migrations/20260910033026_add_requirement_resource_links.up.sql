CREATE TABLE IF NOT EXISTS requirement_resource_links (
    workspace_id BIGINT NOT NULL,
    requirement_id BIGINT NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    resource_id VARCHAR(64) NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, requirement_id, resource_id),
    FOREIGN KEY (workspace_id, resource_id) REFERENCES document_resources(workspace_id, id)
);
CREATE INDEX IF NOT EXISTS idx_requirement_resource_links_resource
    ON requirement_resource_links(workspace_id, resource_id);
