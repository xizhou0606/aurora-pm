DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM documents
        WHERE schema_version <> 1
    ) THEN
        RAISE EXCEPTION 'cannot restore schema_version = 1 constraint while v2 documents exist';
    END IF;
END
$$;

ALTER TABLE documents
    DROP CONSTRAINT IF EXISTS chk_documents_schema_version;

ALTER TABLE documents
    ADD CONSTRAINT chk_documents_schema_version
        CHECK (schema_version = 1);
