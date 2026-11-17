-- Existing personal organization may predate this bridge; do not delete it.
DO $$ BEGIN
 RAISE EXCEPTION 'personal-directory reconciliation is forward-only; restore an approved backup instead';
END $$;
