-- Plan-unit level membership: version owners and participants become the
-- permission subjects. Project sets only carry a creator. See
-- backend/docs/migrations/2026-09-01-plan-unit-members.md for the backfill
-- report and visibility design.

CREATE TABLE plan_unit_participants (
    id                 BIGSERIAL PRIMARY KEY,
    workspace_id       BIGINT      NOT NULL,
    plan_unit_id       BIGINT      NOT NULL,
    user_id            BIGINT      NOT NULL,
    created_by_user_id BIGINT      NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ux_plan_unit_participants_unit_user UNIQUE (plan_unit_id, user_id)
);
CREATE INDEX idx_plan_unit_participants_workspace ON plan_unit_participants (workspace_id);
CREATE INDEX idx_plan_unit_participants_unit ON plan_unit_participants (plan_unit_id);
CREATE INDEX idx_plan_unit_participants_user ON plan_unit_participants (user_id);

-- The project set keeps a single membership fact: who created it. owner_user_id
-- stays in place for now but no longer participates in access decisions.
ALTER TABLE project_sets ADD COLUMN created_by_user_id BIGINT;
CREATE INDEX idx_project_sets_created_by_user_id ON project_sets (created_by_user_id);

-- Backfill created_by_user_id, most-trustworthy source first:
--   1. audit_logs project creation events whose actor still resolves,
--   2. the project's owner_user_id when it resolves to a live user,
--   3. the platform admin account (product decision 2026-09-01: dangling
--      ownership, e.g. the deleted user id=1, defaults to admin).
UPDATE project_sets ps
SET created_by_user_id = (
    SELECT al.user_id FROM audit_logs al
    WHERE al.action = 'create' AND al.resource = 'project' AND al.project_id = ps.id
      AND al.user_id IN (SELECT id FROM users)
    ORDER BY al.id LIMIT 1
)
WHERE created_by_user_id IS NULL;

UPDATE project_sets ps
SET created_by_user_id = ps.owner_user_id
WHERE created_by_user_id IS NULL
  AND ps.owner_user_id IN (SELECT id FROM users);

UPDATE project_sets ps
SET created_by_user_id = (SELECT id FROM users WHERE username = 'admin' ORDER BY id LIMIT 1)
WHERE created_by_user_id IS NULL;

-- Same-transaction audit trail for plan unit member replacements, mirroring
-- project_member_mutation_audits.
CREATE TABLE plan_unit_member_mutation_audits (
    id              BIGSERIAL PRIMARY KEY,
    workspace_id    BIGINT      NOT NULL,
    plan_unit_id    BIGINT      NOT NULL,
    actor_user_id   BIGINT      NOT NULL,
    operation       VARCHAR(32) NOT NULL,
    owner_user_id   BIGINT,
    participant_ids TEXT        NOT NULL DEFAULT '',
    reason          VARCHAR(512) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_plan_unit_member_audit_unit ON plan_unit_member_mutation_audits (plan_unit_id);
CREATE INDEX idx_plan_unit_member_audit_workspace ON plan_unit_member_mutation_audits (workspace_id);
