DROP TABLE IF EXISTS password_reset_transactions;

DROP INDEX IF EXISTS idx_users_password_reset_required;

ALTER TABLE users
    DROP COLUMN IF EXISTS password_reset_required;
