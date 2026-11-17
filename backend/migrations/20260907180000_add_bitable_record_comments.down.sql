-- Explicit rollback only; never discard comments or their committed receipts.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pm_bitable_comments)
       OR EXISTS (SELECT 1 FROM pm_bitable_receipts WHERE kind = 'comment') THEN
        RAISE EXCEPTION 'record comments exist; rollback requires a separately reviewed retention plan';
    END IF;
END $$;
DROP TABLE pm_bitable_comments;
