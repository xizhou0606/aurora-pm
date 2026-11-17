-- 定时采集模型：接入后首次成功采集全量拉取（所有 refs、全部历史），
-- 此后每日/每周一/每月 1 号 09:00 同步对应自然窗。
-- full_synced_at 标记全量是否完成；水位列与采集间隔、重叠窗口随
-- 滚动窗口模型一并移除。
ALTER TABLE scm_sync_runs ADD COLUMN IF NOT EXISTS full_synced_at TIMESTAMPTZ NULL;
ALTER TABLE scm_sync_runs DROP COLUMN IF EXISTS collected_since;
ALTER TABLE scm_collection_settings DROP COLUMN IF EXISTS sync_interval_minutes;
ALTER TABLE scm_collection_settings DROP COLUMN IF EXISTS overlap_days;
