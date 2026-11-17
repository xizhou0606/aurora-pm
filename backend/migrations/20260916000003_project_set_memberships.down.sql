DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM project_set_memberships) OR EXISTS (SELECT 1 FROM resource_access_requests WHERE target_kind='project_set') THEN
  RAISE EXCEPTION 'Project set memberships or requests exist; preserve data before rollback';
 END IF;
END $$;
DROP INDEX idx_project_set_access_requests_scope;
DROP INDEX ux_project_set_access_requests_pending;
ALTER TABLE resource_access_requests DROP CONSTRAINT ck_access_request_target;
ALTER TABLE resource_access_requests DROP COLUMN project_set_id;
ALTER TABLE resource_access_requests ADD CONSTRAINT ck_access_request_target CHECK (
 (target_kind='document_resource' AND resource_id IS NOT NULL AND plan_unit_id IS NULL) OR
 (target_kind='plan_unit' AND resource_id IS NULL AND plan_unit_id IS NOT NULL) OR
 (target_kind='workspace' AND resource_id IS NULL AND plan_unit_id IS NULL)
);
DROP TABLE project_set_memberships;
