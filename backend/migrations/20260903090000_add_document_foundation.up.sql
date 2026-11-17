CREATE TABLE document_spaces (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    name VARCHAR(120) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ux_document_spaces_workspace UNIQUE (workspace_id)
);

CREATE TABLE document_nodes (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    space_id BIGINT NOT NULL REFERENCES document_spaces(id),
    parent_id BIGINT REFERENCES document_nodes(id),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(220) NOT NULL,
    sort_order BIGINT NOT NULL DEFAULT 0,
    created_by BIGINT NOT NULL REFERENCES users(id),
    updated_by BIGINT NOT NULL REFERENCES users(id),
    archived_by BIGINT REFERENCES users(id),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_document_nodes_type CHECK (type IN ('folder', 'document')),
    CONSTRAINT chk_document_nodes_sort_order CHECK (sort_order >= 0),
    CONSTRAINT chk_document_nodes_not_self_parent CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE INDEX idx_document_nodes_workspace_space_parent
    ON document_nodes (workspace_id, space_id, parent_id, sort_order, id);
CREATE INDEX idx_document_nodes_archived_at ON document_nodes (archived_at);

CREATE TABLE documents (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    node_id BIGINT NOT NULL REFERENCES document_nodes(id),
    content_json JSONB NOT NULL,
    schema_version BIGINT NOT NULL DEFAULT 1,
    revision BIGINT NOT NULL DEFAULT 1,
    content_hash VARCHAR(64) NOT NULL,
    plain_text TEXT NOT NULL DEFAULT '',
    collaboration_cutover_at TIMESTAMPTZ,
    created_by BIGINT NOT NULL REFERENCES users(id),
    updated_by BIGINT NOT NULL REFERENCES users(id),
    archived_by BIGINT REFERENCES users(id),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ux_documents_workspace_node UNIQUE (workspace_id, node_id),
    CONSTRAINT chk_documents_schema_version CHECK (schema_version = 1),
    CONSTRAINT chk_documents_revision CHECK (revision > 0)
);

CREATE INDEX idx_documents_workspace_id ON documents (workspace_id, id);
CREATE INDEX idx_documents_archived_at ON documents (archived_at);

CREATE TABLE document_bindings (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    document_id BIGINT NOT NULL REFERENCES documents(id),
    requirement_id BIGINT NOT NULL REFERENCES requirements(id),
    kind VARCHAR(40) NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id),
    archived_by BIGINT REFERENCES users(id),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_document_binding_kind CHECK (kind = 'primary_product_doc')
);

CREATE UNIQUE INDEX ux_document_primary_requirement
    ON document_bindings (workspace_id, requirement_id, kind)
    WHERE archived_at IS NULL;
CREATE INDEX idx_document_bindings_workspace_document
    ON document_bindings (workspace_id, document_id);
CREATE INDEX idx_document_bindings_archived_at ON document_bindings (archived_at);

CREATE TABLE document_assets (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    document_id BIGINT NOT NULL REFERENCES documents(id),
    object_key VARCHAR(1000) NOT NULL UNIQUE,
    file_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id),
    archived_by BIGINT REFERENCES users(id),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_document_assets_size CHECK (size_bytes >= 0)
);

CREATE INDEX idx_document_assets_workspace_document
    ON document_assets (workspace_id, document_id);
CREATE INDEX idx_document_assets_archived_at ON document_assets (archived_at);
