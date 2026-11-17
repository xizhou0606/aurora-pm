DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM pm_bitable_resource_preferences)
 OR EXISTS (SELECT 1 FROM pm_bitable_shortcuts)
 OR EXISTS (SELECT 1 FROM pm_bitable_resources WHERE revision <> 1 OR owner_user_id <> created_by OR parent_folder_id IS NOT NULL OR deleted)
 THEN RAISE EXCEPTION 'bitable resource management has data; rollback refused'; END IF;
END $$;
DROP TABLE pm_bitable_shortcuts;
DROP TABLE pm_bitable_resource_preferences;
ALTER TABLE pm_bitable_resources DROP COLUMN revision, DROP COLUMN owner_user_id, DROP COLUMN parent_folder_id, DROP COLUMN deleted;
