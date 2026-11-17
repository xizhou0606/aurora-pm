DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM document_resource_access_requests WHERE grant_level <> 'editor' OR grant_expires_at IS NOT NULL) THEN
  RAISE EXCEPTION 'Refusing to discard non-default access request grant terms';
 END IF;
END $$;
ALTER TABLE document_resource_access_requests
 DROP CONSTRAINT document_resource_access_requests_grant_level_check,
 DROP COLUMN grant_expires_at,
 DROP COLUMN grant_level;
