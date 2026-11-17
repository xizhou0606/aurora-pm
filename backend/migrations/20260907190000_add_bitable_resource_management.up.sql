ALTER TABLE pm_bitable_resources
 ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 ADD COLUMN owner_user_id bigint,
 ADD COLUMN parent_folder_id bigint,
 ADD COLUMN deleted boolean NOT NULL DEFAULT false;
UPDATE pm_bitable_resources SET owner_user_id = created_by;
ALTER TABLE pm_bitable_resources ALTER COLUMN owner_user_id SET NOT NULL;
CREATE TABLE pm_bitable_resource_preferences (
 resource_id uuid NOT NULL REFERENCES pm_bitable_resources(id),
 actor_id bigint NOT NULL,
 workspace_id bigint NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 pinned boolean NOT NULL,
 favorited boolean NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY(resource_id,actor_id)
);
CREATE TABLE pm_bitable_shortcuts (
 id uuid PRIMARY KEY,
 target_resource_id uuid NOT NULL REFERENCES pm_bitable_resources(id),
 workspace_id bigint NOT NULL,
 parent_folder_id bigint,
 revision bigint NOT NULL CHECK (revision > 0),
 created_by bigint NOT NULL,
 deleted boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL
);
CREATE INDEX idx_pm_bitable_shortcuts_workspace_id ON pm_bitable_shortcuts(workspace_id);
CREATE INDEX idx_pm_bitable_shortcuts_target_resource_id ON pm_bitable_shortcuts(target_resource_id);
-- No bootstrap resources, folders, shortcuts, preferences or other business rows.
