-- Columns and identifiers may have existed before this reconciliation.
-- Use a verified pre-upgrade backup rather than deleting existing history.
DO $$ BEGIN
  RAISE EXCEPTION 'authorization history reconciliation is forward-only; restore a verified pre-upgrade backup';
END $$;
