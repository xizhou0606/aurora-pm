DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM document_resource_access_requests WHERE status = 'expired') THEN
  RAISE EXCEPTION 'Refusing to remove support for recorded expired access requests';
 END IF;
END $$;
ALTER TABLE document_resource_access_requests DROP CONSTRAINT document_resource_access_requests_status_check;
ALTER TABLE document_resource_access_requests ADD CONSTRAINT document_resource_access_requests_status_check
 CHECK (status IN ('pending','approved','rejected','revoked'));
