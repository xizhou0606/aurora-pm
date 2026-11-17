-- Compatibility bridge for databases that advanced past 20260909031500 before
-- the design-review branch reached main. Keep this DDL in sync with that migration.
CREATE TABLE IF NOT EXISTS design_reviews (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    requirement_id bigint REFERENCES requirements(id) ON DELETE SET NULL,
    requirement_title_snapshot varchar(255) NOT NULL,
    project_set_id_snapshot bigint NOT NULL,
    project_set_name_snapshot varchar(255) NOT NULL,
    plan_unit_id_snapshot bigint NOT NULL,
    plan_unit_title_snapshot varchar(255) NOT NULL,
    chain_root_id varchar(36),
    source_review_id varchar(36),
    round_number integer NOT NULL DEFAULT 1 CHECK (round_number > 0),
    created_by_user_id bigint NOT NULL,
    created_by_name varchar(255) NOT NULL DEFAULT '',
    implementation_kind varchar(16) NOT NULL CHECK (implementation_kind IN ('web', 'mobile')),
    implementation_target jsonb NOT NULL DEFAULT '{}'::jsonb,
    implementation_url text,
    implementation_actual_url text,
    figma_url text NOT NULL,
    figma_file_key varchar(160) NOT NULL,
    figma_node_id varchar(120) NOT NULL,
    figma_node_name varchar(300) NOT NULL DEFAULT '',
    viewport_width integer NOT NULL CHECK (viewport_width BETWEEN 240 AND 7680),
    viewport_height integer NOT NULL CHECK (viewport_height BETWEEN 240 AND 7680),
    device_pixel_ratio double precision NOT NULL DEFAULT 1 CHECK (device_pixel_ratio > 0 AND device_pixel_ratio <= 8),
    page_zoom double precision NOT NULL DEFAULT 1 CHECK (page_zoom BETWEEN 0.1 AND 5),
    design_width integer NOT NULL DEFAULT 0,
    design_height integer NOT NULL DEFAULT 0,
    dimensions_match boolean NOT NULL DEFAULT false,
    design_structure_available boolean NOT NULL DEFAULT false,
    implementation_structure_complete boolean NOT NULL DEFAULT false,
    capture_warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
    analysis_scope jsonb NOT NULL DEFAULT '{"enabled":false}'::jsonb,
    mobile_topbar jsonb NOT NULL DEFAULT '{"enabled":false}'::jsonb,
    dom_normalization jsonb NOT NULL DEFAULT '{"enabled":false}'::jsonb,
    model_coverage_complete boolean NOT NULL DEFAULT false,
    heatmap_difference_ratio double precision NOT NULL DEFAULT 1 CHECK (heatmap_difference_ratio BETWEEN 0 AND 1),
    status varchar(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','queued','analyzing','completed','failed','abandoned','deleting')),
    conclusion varchar(20) CHECK (conclusion IN ('needs_fix','needs_review','passed')),
    analysis_summary text,
    model varchar(120),
    reasoning_effort varchar(40),
    prompt_version varchar(100),
    response_id varchar(160),
    analysis_duration_ms integer CHECK (analysis_duration_ms IS NULL OR analysis_duration_ms >= 0),
    analysis_attempts integer NOT NULL DEFAULT 0,
    cancel_requested boolean NOT NULL DEFAULT false,
    last_error_stage varchar(80),
    last_error text,
    completed_at timestamptz,
    abandoned_at timestamptz,
    deleting_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT design_reviews_chain_root_fk FOREIGN KEY (chain_root_id) REFERENCES design_reviews(id) ON DELETE CASCADE,
    CONSTRAINT design_reviews_source_fk FOREIGN KEY (source_review_id) REFERENCES design_reviews(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS design_reviews_workspace_created_idx ON design_reviews(workspace_id, created_at DESC, id);
CREATE INDEX IF NOT EXISTS design_reviews_requirement_idx ON design_reviews(workspace_id, requirement_id);
CREATE INDEX IF NOT EXISTS design_reviews_project_scope_idx ON design_reviews(workspace_id, project_set_id_snapshot, plan_unit_id_snapshot);
CREATE INDEX IF NOT EXISTS design_reviews_status_idx ON design_reviews(workspace_id, status);
CREATE INDEX IF NOT EXISTS design_reviews_chain_active_idx ON design_reviews(workspace_id, chain_root_id, round_number DESC)
    WHERE deleting_at IS NULL AND abandoned_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS design_reviews_chain_round_unique ON design_reviews(workspace_id, chain_root_id, round_number);
CREATE UNIQUE INDEX IF NOT EXISTS design_reviews_active_source_unique ON design_reviews(workspace_id, source_review_id)
    WHERE source_review_id IS NOT NULL AND abandoned_at IS NULL AND deleting_at IS NULL;

CREATE TABLE IF NOT EXISTS design_review_assets (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    kind varchar(64) NOT NULL,
    object_key text NOT NULL,
    content_type varchar(120) NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size >= 0),
    sha256 varchar(64) NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    width integer,
    height integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, review_id, kind),
    UNIQUE (object_key)
);
CREATE INDEX IF NOT EXISTS design_review_assets_review_idx ON design_review_assets(workspace_id, review_id);

CREATE TABLE IF NOT EXISTS design_review_difference_lineages (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    chain_root_review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    verification_status varchar(20) NOT NULL DEFAULT 'pending' CHECK (verification_status IN ('pending','confirmed','false_positive')),
    verified_by_user_id bigint,
    verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_lineages_chain_idx ON design_review_difference_lineages(workspace_id, chain_root_review_id);

CREATE TABLE IF NOT EXISTS design_review_differences (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    lineage_id varchar(36) REFERENCES design_review_difference_lineages(id) ON DELETE SET NULL,
    lineage_root_difference_id varchar(36),
    previous_difference_id varchar(36),
    deep_job_id varchar(36),
    ordinal integer NOT NULL CHECK (ordinal > 0),
    source varchar(24) NOT NULL DEFAULT 'ai' CHECK (source IN ('ai','manual','history_recovered','manual_reopen')),
    analysis_kind varchar(16) NOT NULL DEFAULT 'full' CHECK (analysis_kind IN ('full','deep')),
    created_by_user_id bigint,
    category varchar(24) NOT NULL CHECK (category IN ('layout','size','typography','color','component','icon_image','content','effect')),
    severity varchar(16) NOT NULL CHECK (severity IN ('critical','major','minor')),
    title varchar(160) NOT NULL,
    description text NOT NULL DEFAULT '',
    recommendation text NOT NULL DEFAULT '',
    confidence double precision NOT NULL DEFAULT 1 CHECK (confidence BETWEEN 0 AND 1),
    design_region jsonb,
    implementation_region jsonb,
    evidence_basis varchar(40) NOT NULL DEFAULT 'visual',
    evidence_notes text NOT NULL DEFAULT '',
    verification_status varchar(20) NOT NULL DEFAULT 'pending' CHECK (verification_status IN ('pending','confirmed','false_positive')),
    verified_by_user_id bigint,
    verified_at timestamptz,
    resolution_status varchar(16) NOT NULL DEFAULT 'unresolved' CHECK (resolution_status IN ('unresolved','resolved')),
    resolved_by_user_id bigint,
    resolved_at timestamptz,
    recheck_status varchar(20) CHECK (recheck_status IN ('new','still_present','resolved','uncertain','not_applicable')),
    design_identity_anchor jsonb,
    implementation_identity_anchor jsonb,
    ai_original jsonb,
    human_edited boolean NOT NULL DEFAULT false,
    edited_by_user_id bigint,
    edited_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, review_id, ordinal)
);
CREATE INDEX IF NOT EXISTS design_review_differences_review_idx ON design_review_differences(workspace_id, review_id);
CREATE INDEX IF NOT EXISTS design_review_differences_lineage_idx ON design_review_differences(workspace_id, lineage_id);
CREATE UNIQUE INDEX IF NOT EXISTS design_review_differences_review_lineage_unique ON design_review_differences(workspace_id, review_id, lineage_id)
    WHERE lineage_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS design_review_lineage_events (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    lineage_id varchar(36) NOT NULL REFERENCES design_review_difference_lineages(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    difference_id varchar(36) REFERENCES design_review_differences(id) ON DELETE SET NULL,
    actor_user_id bigint NOT NULL,
    previous_status varchar(20) NOT NULL,
    next_status varchar(20) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_lineage_events_idx ON design_review_lineage_events(workspace_id, lineage_id, created_at);

CREATE TABLE IF NOT EXISTS design_review_recheck_matches (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    candidate_id varchar(80) NOT NULL,
    suggested_lineage_id varchar(36) NOT NULL REFERENCES design_review_difference_lineages(id) ON DELETE CASCADE,
    candidate jsonb NOT NULL,
    design_identity_anchor jsonb,
    implementation_identity_anchor jsonb,
    confidence double precision NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    reason text NOT NULL DEFAULT '',
    status varchar(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','inherited','new')),
    resolved_by_user_id bigint,
    resolved_at timestamptz,
    difference_id varchar(36) REFERENCES design_review_differences(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, review_id, candidate_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS design_review_pending_lineage_match_unique ON design_review_recheck_matches(workspace_id, review_id, suggested_lineage_id)
    WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS design_review_recheck_verifications (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    historical_difference_id varchar(36) NOT NULL REFERENCES design_review_differences(id) ON DELETE RESTRICT,
    lineage_root_difference_id varchar(36) NOT NULL,
    lineage_id varchar(36) REFERENCES design_review_difference_lineages(id) ON DELETE SET NULL,
    current_difference_id varchar(36) REFERENCES design_review_differences(id) ON DELETE SET NULL,
    ai_status varchar(20) NOT NULL CHECK (ai_status IN ('still_present','resolved','uncertain','not_applicable')),
    manual_status_override varchar(20) CHECK (manual_status_override IN ('still_present','resolved','uncertain','not_applicable')),
    confidence double precision NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    reason text NOT NULL DEFAULT '',
    evidence_summary text NOT NULL DEFAULT '',
    current_design_region jsonb,
    current_implementation_region jsonb,
    overridden_by_user_id bigint,
    overridden_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, review_id, historical_difference_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS design_review_current_difference_unique ON design_review_recheck_verifications(workspace_id, current_difference_id)
    WHERE current_difference_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS design_review_resolution_events (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    difference_id varchar(36) REFERENCES design_review_differences(id) ON DELETE SET NULL,
    recheck_verification_id varchar(36) REFERENCES design_review_recheck_verifications(id) ON DELETE SET NULL,
    actor_user_id bigint NOT NULL,
    event_type varchar(60) NOT NULL,
    previous_value varchar(40),
    next_value varchar(40),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_resolution_events_idx ON design_review_resolution_events(workspace_id, review_id, created_at);

CREATE TABLE IF NOT EXISTS design_review_analysis_jobs (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    status varchar(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','succeeded','failed','cancelled')),
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    lease_owner varchar(160),
    lease_until timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_analysis_claim_idx ON design_review_analysis_jobs(status, available_at, lease_until);
CREATE UNIQUE INDEX IF NOT EXISTS design_review_active_analysis_unique ON design_review_analysis_jobs(workspace_id, review_id)
    WHERE status IN ('pending','running');

CREATE TABLE IF NOT EXISTS design_review_deep_jobs (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    review_id varchar(36) NOT NULL REFERENCES design_reviews(id) ON DELETE CASCADE,
    created_by_user_id bigint NOT NULL,
    retry_of_id varchar(36),
    status varchar(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','succeeded','failed','cancelled')),
    design_region jsonb NOT NULL,
    implementation_region jsonb NOT NULL,
    focus varchar(500) NOT NULL DEFAULT '',
    cancel_requested boolean NOT NULL DEFAULT false,
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    lease_owner varchar(160),
    lease_until timestamptz,
    model varchar(120),
    reasoning_effort varchar(40),
    prompt_version varchar(100),
    response_id varchar(160),
    duration_ms integer,
    generated_difference_count integer NOT NULL DEFAULT 0,
    last_error text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_deep_claim_idx ON design_review_deep_jobs(status, available_at, lease_until);
CREATE INDEX IF NOT EXISTS design_review_deep_review_idx ON design_review_deep_jobs(workspace_id, review_id, created_at);

CREATE TABLE IF NOT EXISTS design_review_object_cleanup_jobs (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    object_key text NOT NULL UNIQUE,
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    lease_owner varchar(160),
    lease_until timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_object_cleanup_claim_idx ON design_review_object_cleanup_jobs(available_at, lease_until);

CREATE TABLE IF NOT EXISTS design_review_mobile_collectors (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    name varchar(200) NOT NULL,
    token_hash varchar(64) NOT NULL UNIQUE CHECK (token_hash ~ '^[a-f0-9]{64}$'),
    version varchar(80) NOT NULL,
    mobilewright_version varchar(80) NOT NULL,
    hostname varchar(200) NOT NULL,
    architecture varchar(16) NOT NULL CHECK (architecture IN ('arm64','x64')),
    inventory jsonb NOT NULL DEFAULT '{}'::jsonb,
    doctor_ready boolean NOT NULL DEFAULT false,
    doctor_summary text NOT NULL DEFAULT '',
    last_seen_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_mobile_collectors_user_idx ON design_review_mobile_collectors(workspace_id, user_id, created_at);

CREATE TABLE IF NOT EXISTS design_review_mobile_pairing_codes (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    code_hash varchar(64) NOT NULL UNIQUE CHECK (code_hash ~ '^[a-f0-9]{64}$'),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_pairing_expiry_idx ON design_review_mobile_pairing_codes(expires_at);

CREATE TABLE IF NOT EXISTS design_review_mobile_capture_drafts (
    id varchar(36) PRIMARY KEY,
    workspace_id bigint NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    collector_id varchar(36) NOT NULL REFERENCES design_review_mobile_collectors(id) ON DELETE CASCADE,
    status varchar(16) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','claimed','succeeded','failed','cancelled','consumed','expired')),
    platform varchar(16) NOT NULL CHECK (platform IN ('ios','android')),
    device_id varchar(300) NOT NULL,
    app_id varchar(300),
    target jsonb,
    elements jsonb,
    structure_truncated boolean,
    warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
    screenshot_object_key text,
    screenshot_media_type varchar(120),
    screenshot_sha256 varchar(64) CHECK (screenshot_sha256 IS NULL OR screenshot_sha256 ~ '^[a-f0-9]{64}$'),
    screenshot_byte_size bigint CHECK (screenshot_byte_size IS NULL OR screenshot_byte_size > 0),
    claimed_at timestamptz,
    captured_at timestamptz,
    consumed_at timestamptz,
    expires_at timestamptz NOT NULL,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS design_review_mobile_drafts_user_idx ON design_review_mobile_capture_drafts(workspace_id, user_id, created_at);
CREATE INDEX IF NOT EXISTS design_review_mobile_drafts_claim_idx ON design_review_mobile_capture_drafts(collector_id, status, created_at);
CREATE INDEX IF NOT EXISTS design_review_mobile_drafts_expiry_idx ON design_review_mobile_capture_drafts(expires_at);
