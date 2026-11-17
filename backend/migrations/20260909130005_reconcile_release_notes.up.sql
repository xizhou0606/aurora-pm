-- main introduced 20260909100000 after feature/table databases had already
-- advanced to 20260909130004. Reconcile that skipped migration without changing
-- its immutable source or seeding business data. Fresh databases are unchanged.
CREATE TABLE IF NOT EXISTS release_notes (
    id BIGSERIAL PRIMARY KEY,
    version TEXT NOT NULL UNIQUE,
    release_sha TEXT NOT NULL DEFAULT '',
    content_md TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'autogen',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
