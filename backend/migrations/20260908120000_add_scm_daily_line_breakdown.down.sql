ALTER TABLE scm_daily_contributions
    DROP COLUMN IF EXISTS source_additions,
    DROP COLUMN IF EXISTS test_additions,
    DROP COLUMN IF EXISTS doc_additions,
    DROP COLUMN IF EXISTS config_additions,
    DROP COLUMN IF EXISTS prs_reviewed;
