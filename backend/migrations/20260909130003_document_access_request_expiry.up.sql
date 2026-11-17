ALTER TABLE document_resource_access_requests DROP CONSTRAINT document_resource_access_requests_status_check;
ALTER TABLE document_resource_access_requests ADD CONSTRAINT document_resource_access_requests_status_check
 CHECK (status IN ('pending','approved','rejected','revoked','expired'));
