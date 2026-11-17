CREATE TABLE document_comment_threads (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    document_id BIGINT NOT NULL REFERENCES documents(id),
    anchor_kind VARCHAR(40) NOT NULL,
    anchor_payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    document_revision BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    created_by BIGINT NOT NULL REFERENCES users(id),
    resolved_by BIGINT REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_document_comment_thread_anchor_kind
        CHECK (anchor_kind IN ('document', 'selection_snapshot_v1')),
    CONSTRAINT chk_document_comment_thread_anchor_revision
        CHECK (
            (anchor_kind = 'document' AND document_revision = 0) OR
            (anchor_kind = 'selection_snapshot_v1' AND document_revision > 0)
        ),
    CONSTRAINT chk_document_comment_thread_status
        CHECK (status IN ('open', 'resolved'))
);

CREATE INDEX idx_document_comment_threads_scope
    ON document_comment_threads (workspace_id, document_id, status, updated_at DESC, id DESC);

CREATE TABLE document_comments (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    document_id BIGINT NOT NULL REFERENCES documents(id),
    thread_id BIGINT NOT NULL REFERENCES document_comment_threads(id),
    parent_comment_id BIGINT REFERENCES document_comments(id),
    author_user_id BIGINT NOT NULL REFERENCES users(id),
    body_json JSONB NOT NULL,
    deleted_by BIGINT REFERENCES users(id),
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_document_comments_scope
    ON document_comments (workspace_id, document_id, thread_id, id);
CREATE INDEX idx_document_comments_parent ON document_comments (parent_comment_id);
CREATE INDEX idx_document_comments_author ON document_comments (author_user_id);
CREATE INDEX idx_document_comments_deleted_at ON document_comments (deleted_at);

CREATE TABLE document_comment_mentions (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
    comment_id BIGINT NOT NULL REFERENCES document_comments(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ux_document_comment_mentions_scope UNIQUE (workspace_id, comment_id, user_id)
);

CREATE INDEX idx_document_comment_mentions_workspace
    ON document_comment_mentions (workspace_id, user_id, comment_id);
