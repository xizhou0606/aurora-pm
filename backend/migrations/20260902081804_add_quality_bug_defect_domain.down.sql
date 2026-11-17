DROP INDEX IF EXISTS idx_quality_bugs_defect_domain;

ALTER TABLE quality_bugs
    DROP COLUMN IF EXISTS defect_domain;
