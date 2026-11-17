ALTER TABLE document_resource_access_requests
 ADD COLUMN grant_level varchar(16) NOT NULL DEFAULT 'editor',
 ADD COLUMN grant_expires_at timestamptz NULL,
 ADD CONSTRAINT document_resource_access_requests_grant_level_check CHECK (grant_level IN ('viewer','editor'));
