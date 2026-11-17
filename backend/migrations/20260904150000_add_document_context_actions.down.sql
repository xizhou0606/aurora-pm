DROP TABLE IF EXISTS document_asset_references;
DROP TABLE IF EXISTS document_user_preferences;

DROP INDEX IF EXISTS idx_documents_workspace_owner;
ALTER TABLE documents DROP COLUMN IF EXISTS owner_user_id;

DELETE FROM document_nodes WHERE type = 'shortcut';
DELETE FROM document_nodes WHERE system_managed = TRUE;

DROP INDEX IF EXISTS ux_document_favorite_reference;
DROP INDEX IF EXISTS ux_document_favorites_folder;
DROP INDEX IF EXISTS idx_document_nodes_system_owner;
DROP INDEX IF EXISTS idx_document_nodes_shortcut_target;

ALTER TABLE document_nodes
    DROP CONSTRAINT IF EXISTS chk_document_nodes_system_shape,
    DROP CONSTRAINT IF EXISTS chk_document_nodes_shortcut_target,
    DROP CONSTRAINT IF EXISTS chk_document_nodes_type,
    DROP COLUMN IF EXISTS system_owner_user_id,
    DROP COLUMN IF EXISTS system_kind,
    DROP COLUMN IF EXISTS system_managed,
    DROP COLUMN IF EXISTS shortcut_target_node_id,
    ADD CONSTRAINT chk_document_nodes_type CHECK (type IN ('folder', 'document'));
