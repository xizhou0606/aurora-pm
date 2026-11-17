-- Refuse rollback while message receipts exist instead of deleting user history.
-- The single statement preserves the expanded constraint on failure.
ALTER TABLE notification_inbox_receipts
 DROP CONSTRAINT IF EXISTS notification_inbox_receipts_category_check,
 ADD CONSTRAINT notification_inbox_receipts_category_check
 CHECK (category IN ('mention', 'access'));
