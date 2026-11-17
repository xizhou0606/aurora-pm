ALTER TABLE document_nodes
    DROP CONSTRAINT chk_document_nodes_type;

ALTER TABLE document_nodes
    ADD COLUMN shortcut_target_node_id BIGINT REFERENCES document_nodes(id),
    ADD COLUMN system_managed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN system_kind VARCHAR(40) NOT NULL DEFAULT '',
    ADD COLUMN system_owner_user_id BIGINT REFERENCES users(id),
    ADD CONSTRAINT chk_document_nodes_type
        CHECK (type IN ('folder', 'document', 'shortcut')),
    ADD CONSTRAINT chk_document_nodes_shortcut_target
        CHECK (
            (type = 'shortcut' AND shortcut_target_node_id IS NOT NULL) OR
            (type <> 'shortcut' AND shortcut_target_node_id IS NULL)
        ),
    ADD CONSTRAINT chk_document_nodes_system_shape
        CHECK (
            (
                system_managed = FALSE AND
                system_kind = '' AND
                system_owner_user_id IS NULL
            ) OR
            (
                system_managed = TRUE AND
                system_owner_user_id IS NOT NULL AND
                (
                    (system_kind = 'favorites_folder' AND type = 'folder' AND parent_id IS NULL) OR
                    (system_kind = 'favorite_reference' AND type = 'shortcut')
                )
            )
        );

CREATE INDEX idx_document_nodes_shortcut_target
    ON document_nodes (workspace_id, shortcut_target_node_id)
    WHERE shortcut_target_node_id IS NOT NULL AND archived_at IS NULL;
CREATE INDEX idx_document_nodes_system_owner
    ON document_nodes (workspace_id, system_owner_user_id, system_kind)
    WHERE system_managed = TRUE AND archived_at IS NULL;
CREATE UNIQUE INDEX ux_document_favorites_folder
    ON document_nodes (workspace_id, system_owner_user_id)
    WHERE system_kind = 'favorites_folder' AND archived_at IS NULL;
CREATE UNIQUE INDEX ux_document_favorite_reference
    ON document_nodes (workspace_id, system_owner_user_id, shortcut_target_node_id)
    WHERE system_kind = 'favorite_reference' AND archived_at IS NULL;

ALTER TABLE documents
    ADD COLUMN owner_user_id BIGINT REFERENCES users(id);
UPDATE documents SET owner_user_id = created_by WHERE owner_user_id IS NULL;
ALTER TABLE documents ALTER COLUMN owner_user_id SET NOT NULL;
CREATE INDEX idx_documents_workspace_owner
    ON documents (workspace_id, owner_user_id)
    WHERE archived_at IS NULL;

CREATE TABLE document_user_preferences (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    node_id BIGINT NOT NULL REFERENCES document_nodes(id),
    pinned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ux_document_user_preferences_scope UNIQUE (workspace_id, user_id, node_id)
);

CREATE INDEX idx_document_user_preferences_user_pinned
    ON document_user_preferences (workspace_id, user_id, pinned_at DESC, node_id)
    WHERE pinned_at IS NOT NULL;

CREATE TABLE document_asset_references (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    document_id BIGINT NOT NULL REFERENCES documents(id),
    asset_id BIGINT NOT NULL REFERENCES document_assets(id),
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ux_document_asset_references_scope UNIQUE (workspace_id, document_id, asset_id)
);

CREATE INDEX idx_document_asset_references_asset
    ON document_asset_references (workspace_id, asset_id, document_id);
