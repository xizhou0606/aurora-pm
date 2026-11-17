DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM collaboration_document_states LIMIT 1)
     OR EXISTS (SELECT 1 FROM collaboration_document_updates LIMIT 1) THEN
    RAISE EXCEPTION 'refusing to drop authoritative collaboration document facts';
  END IF;
END $$;

DROP TABLE collaboration_document_updates;
DROP TABLE collaboration_document_states;
