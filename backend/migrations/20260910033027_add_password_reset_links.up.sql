ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_reset_required BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_password_reset_required
    ON users (password_reset_required);

CREATE TABLE IF NOT EXISTS password_reset_transactions (
    id BIGSERIAL PRIMARY KEY,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_password_reset_transactions_user_id
    ON password_reset_transactions (user_id);

CREATE INDEX IF NOT EXISTS idx_password_reset_transactions_expires_at
    ON password_reset_transactions (expires_at);

CREATE INDEX IF NOT EXISTS idx_password_reset_transactions_consumed_at
    ON password_reset_transactions (consumed_at);
