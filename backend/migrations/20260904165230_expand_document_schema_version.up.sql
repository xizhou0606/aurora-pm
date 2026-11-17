ALTER TABLE documents
    DROP CONSTRAINT IF EXISTS chk_documents_schema_version;

ALTER TABLE documents
    ADD CONSTRAINT chk_documents_schema_version
        CHECK (schema_version IN (1, 2));
