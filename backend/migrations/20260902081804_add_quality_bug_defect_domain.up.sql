ALTER TABLE quality_bugs
    ADD COLUMN IF NOT EXISTS defect_domain VARCHAR(32);

CREATE INDEX IF NOT EXISTS idx_quality_bugs_defect_domain
    ON quality_bugs (defect_domain);
