-- Compatibility marker for databases upgraded through the short-lived
-- 000003_project_participants migration. The original migration created
-- project_participants before this repository adopted timestamp migration
-- versions. Keep version 3 readable so golang-migrate can continue from
-- those databases to the timestamped migrations below.
SELECT 1;
