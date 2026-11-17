-- The schema may predate this bridge and may already contain review evidence.
-- Refuse rollback rather than delete objects whose ownership cannot be established.
DO $$ BEGIN
    RAISE EXCEPTION 'design-review reconciliation is forward-only; restore an approved backup instead';
END $$;
