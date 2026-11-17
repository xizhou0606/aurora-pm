DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM resource_access_requests WHERE target_kind <> 'document_resource') THEN
  RAISE EXCEPTION 'Project access requests exist; rollback requires explicit data preservation';
 END IF;
END $$;
DROP INDEX idx_workspace_access_requests_scope;
DROP INDEX ux_workspace_access_requests_pending;
DROP INDEX idx_project_access_requests_scope;
DROP INDEX ux_project_access_requests_pending;
ALTER TABLE resource_access_requests DROP CONSTRAINT ck_access_request_target;
ALTER TABLE resource_access_requests DROP COLUMN plan_unit_id;
ALTER TABLE resource_access_requests DROP COLUMN target_kind;
ALTER TABLE resource_access_requests ALTER COLUMN resource_id SET NOT NULL;
ALTER TABLE resource_access_requests RENAME TO document_resource_access_requests;
