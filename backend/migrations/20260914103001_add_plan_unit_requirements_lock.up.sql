-- 项目/版本计划单元的需求录入锁版字段（2026-09-11 需求录入与锁版二次确认设计）。
-- 旧数据默认 NULL = 未锁版；不按历史项目状态自动初始化锁版记录。
ALTER TABLE plan_units ADD COLUMN IF NOT EXISTS requirements_locked_at TIMESTAMPTZ NULL;
ALTER TABLE plan_units ADD COLUMN IF NOT EXISTS requirements_locked_by_user_id BIGINT NULL;
