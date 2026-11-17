CREATE TABLE pm_bitable_comments (
    id uuid PRIMARY KEY,
    workspace_id bigint NOT NULL,
    resource_id uuid NOT NULL REFERENCES pm_bitable_resources(id),
    table_id uuid NOT NULL REFERENCES pm_bitable_tables(id),
    record_id uuid NOT NULL REFERENCES pm_bitable_records(id),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 4000),
    author_id bigint NOT NULL CHECK (author_id > 0),
    created_at timestamptz NOT NULL
);
CREATE INDEX idx_pm_bitable_comments_scope
    ON pm_bitable_comments(workspace_id, resource_id, table_id, record_id, created_at, id);
-- No default comments. Existing sort_order owns adjacent insertion ordering.
