ALTER TABLE scm_commits
    DROP COLUMN IF EXISTS diff_sample,
    DROP COLUMN IF EXISTS debug_hits,
    DROP COLUMN IF EXISTS todo_hits;
