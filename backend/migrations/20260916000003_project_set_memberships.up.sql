CREATE TABLE project_set_memberships (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
 project_set_id BIGINT NOT NULL REFERENCES project_sets(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL,
 created_by_user_id BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CONSTRAINT ux_project_set_memberships_scope UNIQUE (workspace_id, project_set_id, user_id)
);
CREATE INDEX idx_project_set_memberships_user ON project_set_memberships(workspace_id,user_id);
ALTER TABLE resource_access_requests ADD COLUMN project_set_id BIGINT NULL REFERENCES project_sets(id) ON DELETE RESTRICT;
ALTER TABLE resource_access_requests DROP CONSTRAINT ck_access_request_target;
ALTER TABLE resource_access_requests ADD CONSTRAINT ck_access_request_target CHECK (
 (target_kind='document_resource' AND resource_id IS NOT NULL AND plan_unit_id IS NULL AND project_set_id IS NULL) OR
 (target_kind='plan_unit' AND resource_id IS NULL AND plan_unit_id IS NOT NULL AND project_set_id IS NULL) OR
 (target_kind='workspace' AND resource_id IS NULL AND plan_unit_id IS NULL AND project_set_id IS NULL) OR
 (target_kind='project_set' AND resource_id IS NULL AND plan_unit_id IS NULL AND project_set_id IS NOT NULL)
);
CREATE UNIQUE INDEX ux_project_set_access_requests_pending ON resource_access_requests(workspace_id,project_set_id,applicant_user_id) WHERE target_kind='project_set' AND status='pending';
CREATE INDEX idx_project_set_access_requests_scope ON resource_access_requests(workspace_id,project_set_id,id) WHERE target_kind='project_set';
