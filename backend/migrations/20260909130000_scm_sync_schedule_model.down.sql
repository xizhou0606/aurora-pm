ALTER TABLE scm_collection_settings ADD COLUMN IF NOT EXISTS sync_interval_minutes int NOT NULL DEFAULT 30;
ALTER TABLE scm_collection_settings ADD COLUMN IF NOT EXISTS overlap_days int NOT NULL DEFAULT 7;
ALTER TABLE scm_sync_runs ADD COLUMN IF NOT EXISTS collected_since DATE NULL;
ALTER TABLE scm_sync_runs DROP COLUMN IF EXISTS full_synced_at;
