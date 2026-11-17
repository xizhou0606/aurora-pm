ALTER TABLE document_resource_access_requests RENAME TO resource_access_requests;
ALTER TABLE resource_access_requests ALTER COLUMN resource_id DROP NOT NULL;
ALTER TABLE resource_access_requests ADD COLUMN target_kind VARCHAR(24) NOT NULL DEFAULT 'document_resource';
ALTER TABLE resource_access_requests ADD COLUMN plan_unit_id BIGINT NULL REFERENCES plan_units(id) ON DELETE RESTRICT;
ALTER TABLE resource_access_requests ADD CONSTRAINT ck_access_request_target CHECK (
 (target_kind = 'document_resource' AND resource_id IS NOT NULL AND plan_unit_id IS NULL) OR
 (target_kind = 'plan_unit' AND resource_id IS NULL AND plan_unit_id IS NOT NULL) OR
 (target_kind = 'workspace' AND resource_id IS NULL AND plan_unit_id IS NULL)
);
CREATE UNIQUE INDEX ux_project_access_requests_pending ON resource_access_requests(workspace_id,plan_unit_id,applicant_user_id) WHERE target_kind='plan_unit' AND status='pending';
CREATE INDEX idx_project_access_requests_scope ON resource_access_requests(workspace_id,plan_unit_id,id) WHERE target_kind='plan_unit';

CREATE UNIQUE INDEX ux_workspace_access_requests_pending ON resource_access_requests(workspace_id,applicant_user_id) WHERE target_kind='workspace' AND status='pending';
CREATE INDEX idx_workspace_access_requests_scope ON resource_access_requests(workspace_id,id) WHERE target_kind='workspace';
