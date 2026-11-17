DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM document_resources r
    LEFT JOIN document_resource_bindings b
      ON b.workspace_id = r.workspace_id
     AND b.resource_id = r.id
     AND b.type_key = r.type_key
    WHERE b.resource_id IS NULL
       OR r.revision <> 1
       OR NOT (
         (r.type_key = 'rich_text' AND r.id = 'dr_' || md5('rich_text:' || r.workspace_id::text || ':' || b.payload_ref))
         OR
         (r.type_key = 'bitable' AND r.id = 'dr_' || md5('bitable:' || r.workspace_id::text || ':' || b.payload_ref))
       )
  ) THEN
    RAISE EXCEPTION 'refusing to drop unified document resources after new resources or metadata changes exist';
  END IF;
END $$;

DROP TABLE document_resource_bindings;
DROP TABLE document_resources;
