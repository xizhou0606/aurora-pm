CREATE TABLE collaboration_document_states (
 workspace_id bigint NOT NULL REFERENCES workspaces(id),
 resource_id varchar(64) NOT NULL,
 source_revision bigint NOT NULL CHECK (source_revision > 0),
 source_hash char(64) NOT NULL,
 durable_revision bigint NOT NULL CHECK (durable_revision > 0),
 snapshot bytea NOT NULL CHECK (octet_length(snapshot) > 0 AND octet_length(snapshot) <= 4194304),
 snapshot_hash char(64) NOT NULL,
 state_vector bytea NOT NULL CHECK (octet_length(state_vector) > 0 AND octet_length(state_vector) <= 262144),
 state_vector_hash char(64) NOT NULL,
 projection jsonb NOT NULL CHECK (jsonb_typeof(projection) = 'object' AND pg_column_size(projection) <= 4194304),
 projection_hash char(64) NOT NULL,
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 seeded_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (workspace_id, resource_id),
 FOREIGN KEY (workspace_id, resource_id) REFERENCES document_resources(workspace_id, id),
 CHECK (source_hash ~ '^[0-9a-f]{64}$'),
 CHECK (snapshot_hash ~ '^[0-9a-f]{64}$'),
 CHECK (state_vector_hash ~ '^[0-9a-f]{64}$'),
 CHECK (projection_hash ~ '^[0-9a-f]{64}$')
);

CREATE TABLE collaboration_document_updates (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 durable_revision bigint NOT NULL CHECK (durable_revision > 1),
 base_durable_revision bigint NOT NULL CHECK (base_durable_revision > 0),
 operation_id varchar(128) NOT NULL,
 actor_user_id bigint NOT NULL REFERENCES users(id),
 update bytea NOT NULL CHECK (octet_length(update) > 0 AND octet_length(update) <= 262144),
 update_hash char(64) NOT NULL CHECK (update_hash ~ '^[0-9a-f]{64}$'),
 base_state_vector_hash char(64) NOT NULL CHECK (base_state_vector_hash ~ '^[0-9a-f]{64}$'),
 result_state_vector_hash char(64) NOT NULL CHECK (result_state_vector_hash ~ '^[0-9a-f]{64}$'),
 result_snapshot_hash char(64) NOT NULL CHECK (result_snapshot_hash ~ '^[0-9a-f]{64}$'),
 result_projection_hash char(64) NOT NULL CHECK (result_projection_hash ~ '^[0-9a-f]{64}$'),
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 committed_at timestamptz NOT NULL,
 UNIQUE (workspace_id, resource_id, durable_revision),
 UNIQUE (workspace_id, resource_id, operation_id),
 FOREIGN KEY (workspace_id, resource_id)
   REFERENCES collaboration_document_states(workspace_id, resource_id),
 CHECK (durable_revision = base_durable_revision + 1)
);
CREATE INDEX idx_collaboration_document_updates_recovery
 ON collaboration_document_updates(workspace_id, resource_id, durable_revision);
