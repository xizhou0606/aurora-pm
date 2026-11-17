CREATE TABLE document_resources (
 id varchar(64) PRIMARY KEY,
 workspace_id bigint NOT NULL REFERENCES workspaces(id),
 type_key varchar(64) NOT NULL,
 name varchar(220) NOT NULL,
 parent_ref varchar(128),
 owner_user_id bigint NOT NULL,
 created_by bigint NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 lifecycle varchar(20) NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'archived')),
 archived_at timestamptz,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 UNIQUE(workspace_id, id),
 UNIQUE(workspace_id, id, type_key),
 CHECK (type_key <> ''),
 CHECK (name <> '')
);
CREATE INDEX idx_document_resources_workspace_lifecycle
 ON document_resources(workspace_id, lifecycle, updated_at DESC);
CREATE INDEX idx_document_resources_owner ON document_resources(workspace_id, owner_user_id);
CREATE INDEX idx_document_resources_type ON document_resources(workspace_id, type_key);

CREATE TABLE document_resource_bindings (
 id bigserial PRIMARY KEY,
 workspace_id bigint NOT NULL REFERENCES workspaces(id),
 resource_id varchar(64) NOT NULL,
 type_key varchar(64) NOT NULL,
 payload_ref varchar(128) NOT NULL,
 created_at timestamptz NOT NULL,
 UNIQUE(resource_id),
 UNIQUE(workspace_id, type_key, payload_ref),
 FOREIGN KEY(workspace_id, resource_id, type_key)
   REFERENCES document_resources(workspace_id, id, type_key)
);
CREATE INDEX idx_document_resource_bindings_workspace ON document_resource_bindings(workspace_id);

INSERT INTO document_resources (
 id, workspace_id, type_key, name, parent_ref, owner_user_id, created_by,
 revision, lifecycle, archived_at, created_at, updated_at
)
SELECT
 'dr_' || md5('rich_text:' || d.workspace_id::text || ':' || d.id::text),
 d.workspace_id,
 'rich_text',
 n.title,
 CASE WHEN n.parent_id IS NULL THEN NULL ELSE 'document-node:' || n.parent_id::text END,
 d.owner_user_id,
 d.created_by,
 1,
 'active',
 NULL,
 d.created_at,
 GREATEST(d.updated_at, n.updated_at)
FROM documents d
JOIN document_nodes n ON n.id = d.node_id AND n.workspace_id = d.workspace_id
WHERE d.archived_at IS NULL AND n.archived_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO document_resource_bindings (workspace_id, resource_id, type_key, payload_ref, created_at)
SELECT
 d.workspace_id,
 'dr_' || md5('rich_text:' || d.workspace_id::text || ':' || d.id::text),
 'rich_text',
 d.id::text,
 d.created_at
FROM documents d
JOIN document_nodes n ON n.id = d.node_id AND n.workspace_id = d.workspace_id
WHERE d.archived_at IS NULL AND n.archived_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO document_resources (
 id, workspace_id, type_key, name, parent_ref, owner_user_id, created_by,
 revision, lifecycle, archived_at, created_at, updated_at
)
SELECT
 'dr_' || md5('bitable:' || r.workspace_id::text || ':' || r.id::text),
 r.workspace_id,
 'bitable',
 r.name,
 CASE WHEN r.parent_folder_id IS NULL THEN NULL ELSE 'document-node:' || r.parent_folder_id::text END,
 r.owner_user_id,
 r.created_by,
 1,
 'active',
 NULL,
 r.created_at,
 r.updated_at
FROM pm_bitable_resources r
WHERE r.deleted = false
ON CONFLICT DO NOTHING;

INSERT INTO document_resource_bindings (workspace_id, resource_id, type_key, payload_ref, created_at)
SELECT
 r.workspace_id,
 'dr_' || md5('bitable:' || r.workspace_id::text || ':' || r.id::text),
 'bitable',
 r.id::text,
 r.created_at
FROM pm_bitable_resources r
WHERE r.deleted = false
ON CONFLICT DO NOTHING;

-- No folders, documents, bitables, permissions, links, or user grants are created.
