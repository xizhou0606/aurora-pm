DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM collaboration_committed_events LIMIT 1) THEN
    RAISE EXCEPTION 'refusing to drop collaboration event history; export and approve destructive rollback first';
  END IF;
END $$;

DROP TABLE collaboration_presence;
DROP TABLE collaboration_committed_events;
DROP TABLE collaboration_resource_streams;
