DROP INDEX IF EXISTS idx_project_sets_created_by_user_id;
ALTER TABLE project_sets DROP COLUMN IF EXISTS created_by_user_id;

DROP TABLE IF EXISTS plan_unit_member_mutation_audits;
DROP TABLE IF EXISTS plan_unit_participants;
