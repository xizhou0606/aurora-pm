CREATE TABLE collaboration_resource_streams (
 workspace_id bigint NOT NULL REFERENCES workspaces(id),
 resource_id varchar(64) NOT NULL,
 next_sequence bigint NOT NULL DEFAULT 1 CHECK (next_sequence > 0),
 retained_from_sequence bigint NOT NULL DEFAULT 1 CHECK (retained_from_sequence > 0),
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (workspace_id, resource_id),
 FOREIGN KEY (workspace_id, resource_id) REFERENCES document_resources(workspace_id, id)
);

CREATE TABLE collaboration_committed_events (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 sequence bigint NOT NULL CHECK (sequence > 0),
 operation_id varchar(128) NOT NULL,
 actor_user_id bigint NOT NULL REFERENCES users(id),
 event_type varchar(64) NOT NULL CHECK (event_type <> ''),
 payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 committed_at timestamptz NOT NULL,
 UNIQUE (workspace_id, resource_id, sequence),
 UNIQUE (workspace_id, resource_id, operation_id),
 FOREIGN KEY (workspace_id, resource_id)
   REFERENCES collaboration_resource_streams(workspace_id, resource_id)
);
CREATE INDEX idx_collaboration_events_replay
 ON collaboration_committed_events(workspace_id, resource_id, sequence);

CREATE TABLE collaboration_presence (
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 connection_id varchar(128) NOT NULL,
 actor_user_id bigint NOT NULL REFERENCES users(id),
 actor_display_name varchar(220) NOT NULL,
 state jsonb NOT NULL CHECK (jsonb_typeof(state) = 'object'),
 expires_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (workspace_id, resource_id, connection_id),
 FOREIGN KEY (workspace_id, resource_id)
   REFERENCES collaboration_resource_streams(workspace_id, resource_id)
);
CREATE INDEX idx_collaboration_presence_expiry ON collaboration_presence(expires_at);
