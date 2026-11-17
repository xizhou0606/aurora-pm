-- 代码贡献同步时机可配置：每日触发小时 / 每周触发星期 / 每月触发日。
-- 默认值与历史硬编码行为一致（09:00、周一、1 号），存量行随列默认值生效。
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS daily_sync_hour INT NOT NULL DEFAULT 9;
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS weekly_sync_weekday INT NOT NULL DEFAULT 1;
ALTER TABLE scm_collection_settings
    ADD COLUMN IF NOT EXISTS monthly_sync_day INT NOT NULL DEFAULT 1;
