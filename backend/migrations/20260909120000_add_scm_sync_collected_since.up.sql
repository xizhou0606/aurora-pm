-- 覆盖水位：仓库已确认采集覆盖的最早自然日（工作区时区）。
-- 采集循环据此自动向 initialSyncLookbackDays 地板逐轮回补历史，
-- 无需人工回填。历史行为 NULL，按上次成功时间保守起步。
ALTER TABLE scm_sync_runs ADD COLUMN IF NOT EXISTS collected_since DATE NULL;
