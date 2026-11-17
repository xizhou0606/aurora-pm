DROP INDEX IF EXISTS idx_work_tasks_task_client_id;
ALTER TABLE work_tasks DROP COLUMN IF EXISTS task_client_id;
DROP TABLE IF EXISTS task_client_options;
