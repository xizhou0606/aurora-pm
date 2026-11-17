ALTER TABLE scm_collection_settings
    DROP COLUMN IF EXISTS daily_sync_minute;
ALTER TABLE scm_collection_settings
    DROP COLUMN IF EXISTS weekly_sync_hour;
ALTER TABLE scm_collection_settings
    DROP COLUMN IF EXISTS weekly_sync_minute;
ALTER TABLE scm_collection_settings
    DROP COLUMN IF EXISTS monthly_sync_hour;
ALTER TABLE scm_collection_settings
    DROP COLUMN IF EXISTS monthly_sync_minute;
