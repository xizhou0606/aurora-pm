DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM document_resource_access_requests LIMIT 1) THEN
  RAISE EXCEPTION 'Refusing to remove nonempty document access requests and granted permissions';
 END IF;
END $$;
DROP TABLE document_resource_access_requests;
