DROP INDEX IF EXISTS idx_users_feishu_department_id;
ALTER TABLE users DROP COLUMN IF EXISTS feishu_department_id;
DROP INDEX IF EXISTS idx_feishu_departments_parent;
DROP INDEX IF EXISTS ux_feishu_departments_open_id;
DROP TABLE IF EXISTS feishu_departments;
