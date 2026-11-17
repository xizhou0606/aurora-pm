-- These objects may predate this bridge and contain SCM or identity data.
-- Refuse rollback rather than delete objects whose ownership cannot be established.
DO $$ BEGIN
 RAISE EXCEPTION 'Cannot roll back the bitable/main compatibility bridge; restore an audited backup instead';
END $$;
