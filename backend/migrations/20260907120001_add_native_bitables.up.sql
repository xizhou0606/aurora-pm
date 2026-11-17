-- Renumbered after the main merge collision, tolerating the already-applied branch schema.
CREATE TABLE IF NOT EXISTS pm_bitable_resources (
 id uuid PRIMARY KEY, workspace_id bigint NOT NULL REFERENCES workspaces(id), name varchar(200) NOT NULL,
 created_by bigint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pm_bitable_resources_workspace ON pm_bitable_resources(workspace_id);
CREATE TABLE IF NOT EXISTS pm_bitable_tables (
 id uuid PRIMARY KEY, resource_id uuid NOT NULL REFERENCES pm_bitable_resources(id), workspace_id bigint NOT NULL REFERENCES workspaces(id),
 name varchar(200) NOT NULL, schema_revision bigint NOT NULL CHECK (schema_revision > 0), data_revision bigint NOT NULL CHECK (data_revision >= 0),
 fields jsonb NOT NULL CHECK (jsonb_typeof(fields) = 'array'), views jsonb NOT NULL CHECK (jsonb_typeof(views) = 'array'),
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pm_bitable_tables_scope ON pm_bitable_tables(workspace_id, resource_id);
CREATE TABLE IF NOT EXISTS pm_bitable_records (
 id uuid PRIMARY KEY, table_id uuid NOT NULL REFERENCES pm_bitable_tables(id), resource_id uuid NOT NULL REFERENCES pm_bitable_resources(id), workspace_id bigint NOT NULL REFERENCES workspaces(id),
 revision bigint NOT NULL CHECK (revision > 0), sort_order bigint NOT NULL, values jsonb NOT NULL CHECK (jsonb_typeof(values) = 'object'), deleted boolean NOT NULL DEFAULT false,
 created_by bigint NOT NULL, updated_by bigint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pm_bitable_records_scope ON pm_bitable_records(workspace_id, resource_id, table_id, deleted);
CREATE TABLE IF NOT EXISTS pm_bitable_receipts (
 scope_key varchar(160) NOT NULL, operation_id uuid NOT NULL, workspace_id bigint NOT NULL, actor_id bigint NOT NULL,
 resource_id uuid NOT NULL, table_id uuid NOT NULL, kind varchar(20) NOT NULL, request_hash char(64) NOT NULL,
 result jsonb NOT NULL, created_at timestamptz NOT NULL, PRIMARY KEY(scope_key, operation_id)
);
CREATE TABLE IF NOT EXISTS pm_bitable_assets (
 id uuid PRIMARY KEY, workspace_id bigint NOT NULL, resource_id uuid NOT NULL REFERENCES pm_bitable_resources(id),
 table_id uuid NOT NULL REFERENCES pm_bitable_tables(id), record_id uuid NOT NULL REFERENCES pm_bitable_records(id), field_id uuid NOT NULL,
 metadata jsonb NOT NULL, created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pm_bitable_assets_scope ON pm_bitable_assets(workspace_id, resource_id, table_id, record_id, field_id);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS resource_ref varchar(128) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_ref ON audit_logs(resource_ref);
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS aggregate_ref varchar(128) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate_ref ON outbox_events(aggregate_ref);
ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_schema_version;
ALTER TABLE documents ADD CONSTRAINT chk_documents_schema_version CHECK (schema_version IN (1, 2, 3));
