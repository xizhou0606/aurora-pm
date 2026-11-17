-- 需求交付进度 / 范围修订（可配置工作流联动详细方案阶段 B）。
-- delivery_scope_revision 随当前需求集合变化单调递增，用于追溯与后续缓存失效；
-- 旧数据默认 0，不回填历史修订。
ALTER TABLE plan_units ADD COLUMN IF NOT EXISTS delivery_scope_revision BIGINT NOT NULL DEFAULT 0;
