CREATE TABLE document_resource_access_requests (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL,
 resource_id VARCHAR(64) NOT NULL,
 applicant_user_id BIGINT NOT NULL,
 reason TEXT NOT NULL DEFAULT '',
 notify_owner BOOLEAN NOT NULL DEFAULT TRUE,
 status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','revoked')),
 read BOOLEAN NOT NULL DEFAULT FALSE,
 created_at TIMESTAMPTZ NOT NULL,
 reviewed_at TIMESTAMPTZ,
 reviewed_by BIGINT,
 CONSTRAINT fk_access_request_resource FOREIGN KEY(workspace_id,resource_id)
 REFERENCES document_resource_access_policies(workspace_id,resource_id)
);
CREATE INDEX idx_access_requests_scope ON document_resource_access_requests(workspace_id,resource_id);
CREATE UNIQUE INDEX ux_access_requests_pending ON document_resource_access_requests(workspace_id,resource_id,applicant_user_id) WHERE status = 'pending';
CREATE INDEX idx_access_requests_grants ON document_resource_access_requests(workspace_id,applicant_user_id,resource_id) WHERE status = 'approved';
