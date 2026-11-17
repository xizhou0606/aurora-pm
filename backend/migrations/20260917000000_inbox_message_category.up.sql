-- One atomic ALTER keeps the category constraint in force if validation fails.
-- No historical receipts or external delivery states are changed.
ALTER TABLE notification_inbox_receipts
 DROP CONSTRAINT IF EXISTS notification_inbox_receipts_category_check,
 ADD CONSTRAINT notification_inbox_receipts_category_check
 CHECK (category IN ('mention', 'message', 'access'));
