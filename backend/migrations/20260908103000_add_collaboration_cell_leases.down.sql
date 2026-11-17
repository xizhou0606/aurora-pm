DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM collaboration_cell_leases
    WHERE lease_id IS NOT NULL AND expires_at > clock_timestamp()
    LIMIT 1
  ) THEN
    RAISE EXCEPTION 'refusing to drop active collaboration cell leases';
  END IF;
END $$;

DROP TABLE collaboration_cell_leases;
