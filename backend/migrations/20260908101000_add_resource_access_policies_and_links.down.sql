DO $$
BEGIN
 IF EXISTS (
   SELECT 1 FROM document_resource_access_policies
   WHERE mode <> 'legacy' OR revision <> 1 OR workspace_level IS NOT NULL
 ) OR EXISTS (SELECT 1 FROM document_resource_access_rules)
   OR EXISTS (SELECT 1 FROM document_resource_share_links)
   OR EXISTS (SELECT 1 FROM document_resource_link_access_contexts)
   OR EXISTS (SELECT 1 FROM document_resource_access_receipts)
   OR EXISTS (SELECT 1 FROM document_resource_access_audits)
   OR EXISTS (SELECT 1 FROM document_resource_access_outbox)
   OR EXISTS (SELECT 1 FROM document_share_link_exchange_limits)
 THEN
   RAISE EXCEPTION 'refusing to drop resource access data after runtime writes';
 END IF;
END $$;

DROP TABLE document_share_link_exchange_limits;
DROP TABLE document_resource_access_outbox;
DROP TABLE document_resource_access_audits;
DROP TABLE document_resource_access_receipts;
DROP TABLE document_resource_link_access_contexts;
DROP TABLE document_resource_share_links;
DROP TABLE document_resource_access_rules;
DROP TABLE document_resource_access_policies;
