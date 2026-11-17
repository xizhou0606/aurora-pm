-- Explicit migration rollback only; never run during API startup.
-- Refuse to discard source resources, document references or UUID audit history.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM documents WHERE schema_version = 3)
    OR EXISTS (SELECT 1 FROM pm_bitable_resources)
    OR EXISTS (SELECT 1 FROM audit_logs WHERE resource_ref <> '')
    OR EXISTS (SELECT 1 FROM outbox_events WHERE aggregate_ref <> '') THEN
  RAISE EXCEPTION 'Cannot roll back native bitables while resources, document references or audit history exist';
 END IF;
END $$;
DROP TABLE pm_bitable_assets;
DROP TABLE pm_bitable_receipts;
DROP TABLE pm_bitable_records;
DROP TABLE pm_bitable_tables;
DROP TABLE pm_bitable_resources;
ALTER TABLE documents DROP CONSTRAINT chk_documents_schema_version;
ALTER TABLE documents ADD CONSTRAINT chk_documents_schema_version CHECK (schema_version IN (1, 2));
ALTER TABLE audit_logs DROP COLUMN resource_ref;
ALTER TABLE outbox_events DROP COLUMN aggregate_ref;
