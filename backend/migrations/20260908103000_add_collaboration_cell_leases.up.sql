CREATE TABLE collaboration_cell_leases (
 workspace_id bigint NOT NULL REFERENCES workspaces(id),
 resource_id varchar(64) NOT NULL,
 table_id uuid NOT NULL,
 record_id uuid NOT NULL,
 field_id uuid NOT NULL,
 lease_id uuid,
 fencing_token bigint NOT NULL DEFAULT 0 CHECK (fencing_token >= 0),
 owner_connection_id varchar(128),
 owner_editor_instance_id varchar(128),
 actor_user_id bigint REFERENCES users(id),
 actor_display_name varchar(220),
 policy_revision bigint CHECK (policy_revision > 0),
 expires_at timestamptz,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (workspace_id, resource_id, table_id, record_id, field_id),
 FOREIGN KEY (workspace_id, resource_id) REFERENCES document_resources(workspace_id, id),
 CHECK (
   (lease_id IS NULL AND owner_connection_id IS NULL AND owner_editor_instance_id IS NULL
     AND actor_user_id IS NULL AND actor_display_name IS NULL AND policy_revision IS NULL AND expires_at IS NULL)
   OR
   (lease_id IS NOT NULL AND owner_connection_id IS NOT NULL AND owner_editor_instance_id IS NOT NULL
     AND actor_user_id IS NOT NULL AND actor_display_name IS NOT NULL AND policy_revision IS NOT NULL AND expires_at IS NOT NULL)
 )
);

CREATE INDEX idx_collaboration_cell_leases_connection
 ON collaboration_cell_leases(workspace_id, owner_connection_id)
 WHERE lease_id IS NOT NULL;
CREATE INDEX idx_collaboration_cell_leases_expiry
 ON collaboration_cell_leases(expires_at)
 WHERE lease_id IS NOT NULL;
