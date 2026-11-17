-- Replay personal-directory DDL for databases already past its original version.
ALTER TABLE document_nodes ADD COLUMN IF NOT EXISTS directory_owner_user_id bigint;
CREATE INDEX IF NOT EXISTS idx_document_nodes_directory_owner ON document_nodes(directory_owner_user_id);
CREATE TABLE IF NOT EXISTS personal_document_spaces (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL,
 owner_user_id bigint NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT ux_personal_document_space UNIQUE(workspace_id,owner_user_id)
);
CREATE TABLE IF NOT EXISTS personal_document_entries (
 id bigserial PRIMARY KEY,
 space_id bigint NOT NULL REFERENCES personal_document_spaces(id),
 node_id bigint NOT NULL REFERENCES document_nodes(id),
 resource_id text NOT NULL DEFAULT '',
 parent_id bigint,
 sort_order bigint NOT NULL CHECK(sort_order >= 0),
 title text NOT NULL DEFAULT '',
 removed boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT ux_personal_directory_entry UNIQUE(space_id,node_id),
 CONSTRAINT fk_personal_directory_parent FOREIGN KEY(space_id,parent_id) REFERENCES personal_document_entries(space_id,node_id) DEFERRABLE INITIALLY DEFERRED,
 CONSTRAINT ck_personal_directory_cycle CHECK(parent_id IS NULL OR parent_id <> node_id)
);
CREATE INDEX IF NOT EXISTS idx_personal_directory_parent ON personal_document_entries(space_id,parent_id,sort_order);

CREATE TABLE IF NOT EXISTS personal_document_resource_entries (
 id bigserial PRIMARY KEY,
 space_id bigint NOT NULL REFERENCES personal_document_spaces(id),
 resource_id text NOT NULL,
 parent_id bigint,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision > 0),
 last_operation_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT ux_personal_resource_entry UNIQUE(space_id,resource_id),
 CONSTRAINT fk_personal_resource_parent FOREIGN KEY(space_id,parent_id) REFERENCES personal_document_entries(space_id,node_id)
);
