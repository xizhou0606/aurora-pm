CREATE TABLE project_member_mutation_audits (
    id              BIGSERIAL PRIMARY KEY,
    workspace_id    BIGINT       NOT NULL,
    project_set_id  BIGINT       NOT NULL,
    actor_user_id   BIGINT       NOT NULL,
    operation       VARCHAR(32)  NOT NULL,
    owner_user_id   BIGINT,
    participant_ids TEXT         NOT NULL DEFAULT '',
    reason          VARCHAR(512) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL
);
CREATE INDEX idx_member_audit_workspace ON project_member_mutation_audits (workspace_id);
CREATE INDEX idx_member_audit_project ON project_member_mutation_audits (project_set_id);
