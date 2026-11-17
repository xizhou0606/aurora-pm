CREATE TABLE IF NOT EXISTS release_notes (
    id BIGSERIAL PRIMARY KEY,
    version TEXT NOT NULL UNIQUE,
    release_sha TEXT NOT NULL DEFAULT '',
    content_md TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'autogen',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
