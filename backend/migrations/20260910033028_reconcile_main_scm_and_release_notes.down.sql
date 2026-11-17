-- Forward-only compatibility bridge: do not remove shared notes or schema on rollback.
DO $$ BEGIN
    RAISE EXCEPTION 'main reconciliation is forward-only; restore an approved backup instead';
END $$;
