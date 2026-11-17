-- Retire the project-set level participants table. Version-level membership
-- lives in plan_unit_participants (20260901090000). The project set keeps only
-- created_by_user_id. The table never held rows in any environment (the
-- participant backfill was superseded by the version-level model), so the
-- drop is data-safe.
DROP TABLE IF EXISTS project_participants;
