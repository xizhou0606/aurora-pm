-- 同步时机精确到分钟：每日/每周/每月各自独立的触发时刻（工作区时区）。
-- 默认值延续历史行为：09:00 触发、整分。
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS daily_sync_minute INT NOT NULL DEFAULT 0;
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS weekly_sync_hour INT NOT NULL DEFAULT 9;
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS weekly_sync_minute INT NOT NULL DEFAULT 0;
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS monthly_sync_hour INT NOT NULL DEFAULT 9;
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS monthly_sync_minute INT NOT NULL DEFAULT 0;
