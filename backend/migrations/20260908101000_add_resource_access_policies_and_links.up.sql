CREATE TABLE document_resource_access_policies (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL REFERENCES workspaces(id),
 resource_id varchar(64) NOT NULL,
 mode varchar(20) NOT NULL CHECK (mode IN ('private', 'restricted', 'workspace', 'link', 'legacy')),
 workspace_level varchar(16) CHECK (workspace_level IN ('viewer', 'editor')),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 created_by bigint NOT NULL,
 updated_by bigint NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 UNIQUE(workspace_id, resource_id),
 FOREIGN KEY(workspace_id, resource_id) REFERENCES document_resources(workspace_id, id),
 CHECK ((mode = 'workspace' AND workspace_level IS NOT NULL) OR (mode <> 'workspace' AND workspace_level IS NULL))
);

CREATE TABLE document_resource_access_rules (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 subject_kind varchar(32) NOT NULL CHECK (subject_kind IN ('user', 'role', 'user_group', 'organization_unit', 'project_set')),
 subject_id varchar(128) NOT NULL CHECK (subject_id <> ''),
 level varchar(16) NOT NULL CHECK (level IN ('viewer', 'editor')),
 include_descendants boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL,
 UNIQUE(workspace_id, resource_id, subject_kind, subject_id),
 FOREIGN KEY(workspace_id, resource_id)
   REFERENCES document_resource_access_policies(workspace_id, resource_id) ON DELETE CASCADE,
 CHECK (include_descendants = false OR subject_kind = 'organization_unit')
);
CREATE INDEX idx_document_resource_access_rules_subject
 ON document_resource_access_rules(workspace_id, subject_kind, subject_id);

CREATE TABLE document_resource_share_links (
 id varchar(40) PRIMARY KEY CHECK (id <> ''),
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 secret_hash varchar(64) NOT NULL UNIQUE CHECK (secret_hash ~ '^[0-9a-f]{64}$'),
 level varchar(16) NOT NULL CHECK (level IN ('viewer', 'editor')),
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
 expires_at timestamptz,
 revoked_at timestamptz,
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 UNIQUE(workspace_id, resource_id, id),
 FOREIGN KEY(workspace_id, resource_id)
   REFERENCES document_resource_access_policies(workspace_id, resource_id) ON DELETE CASCADE
);
CREATE INDEX idx_document_resource_share_links_resource
 ON document_resource_share_links(workspace_id, resource_id, created_at DESC);

CREATE TABLE document_resource_link_access_contexts (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 link_id varchar(40) NOT NULL,
 link_version bigint NOT NULL CHECK (link_version > 0),
 user_id bigint NOT NULL,
 token_hash varchar(64) NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 FOREIGN KEY(workspace_id, resource_id, link_id)
   REFERENCES document_resource_share_links(workspace_id, resource_id, id) ON DELETE CASCADE,
 CHECK (expires_at > created_at)
);
CREATE INDEX idx_document_resource_link_context_scope
 ON document_resource_link_access_contexts(workspace_id, resource_id, user_id, expires_at);

CREATE TABLE document_resource_access_receipts (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 operation varchar(40) NOT NULL CHECK (operation <> ''),
 idempotency_key varchar(160) NOT NULL CHECK (idempotency_key <> ''),
 input_digest varchar(64) NOT NULL CHECK (input_digest ~ '^[0-9a-f]{64}$'),
 result_json text NOT NULL,
 created_at timestamptz NOT NULL,
 UNIQUE(workspace_id, resource_id, operation, idempotency_key),
 FOREIGN KEY(workspace_id, resource_id)
   REFERENCES document_resource_access_policies(workspace_id, resource_id) ON DELETE CASCADE
);

CREATE TABLE document_resource_access_audits (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 event_type varchar(64) NOT NULL CHECK (event_type <> ''),
 actor_user_id bigint NOT NULL,
 before_json text NOT NULL,
 after_json text NOT NULL,
 created_at timestamptz NOT NULL,
 FOREIGN KEY(workspace_id, resource_id) REFERENCES document_resources(workspace_id, id)
);
CREATE INDEX idx_document_resource_access_audits_resource
 ON document_resource_access_audits(workspace_id, resource_id, created_at DESC);

CREATE TABLE document_resource_access_outbox (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 resource_id varchar(64) NOT NULL,
 event_type varchar(64) NOT NULL CHECK (event_type <> ''),
 payload_json text NOT NULL,
 created_at timestamptz NOT NULL,
 published_at timestamptz,
 FOREIGN KEY(workspace_id, resource_id) REFERENCES document_resources(workspace_id, id)
);
CREATE INDEX idx_document_resource_access_outbox_pending
 ON document_resource_access_outbox(published_at, id);

CREATE TABLE document_share_link_exchange_limits (
 workspace_id bigint NOT NULL REFERENCES workspaces(id),
 user_id bigint NOT NULL,
 window_started_at timestamptz NOT NULL,
 attempts bigint NOT NULL CHECK (attempts > 0),
 updated_at timestamptz NOT NULL,
 PRIMARY KEY(workspace_id, user_id, window_started_at)
);
CREATE INDEX idx_document_share_link_exchange_limits_cleanup
 ON document_share_link_exchange_limits(window_started_at);

INSERT INTO document_resource_access_policies (
 workspace_id, resource_id, mode, workspace_level, revision,
 created_by, updated_by, created_at, updated_at
)
SELECT
 workspace_id, id, 'legacy', NULL, 1,
 owner_user_id, owner_user_id, created_at, updated_at
FROM document_resources
ON CONFLICT (workspace_id, resource_id) DO NOTHING;

-- Existing resources enter explicit compatibility mode only. This migration
-- creates no rules, links, access contexts, receipts, audit rows, or grants.
