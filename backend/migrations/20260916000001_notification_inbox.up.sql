CREATE TABLE IF NOT EXISTS notification_inbox_receipts (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL,
 recipient_user_id BIGINT NOT NULL,
 event_id BIGINT NOT NULL REFERENCES outbox_events(id) ON DELETE RESTRICT,
 event_key TEXT NOT NULL,
 category TEXT NOT NULL CHECK (category IN ('mention','access')),
 read_at TIMESTAMPTZ,
 CONSTRAINT ux_notification_receipt_event UNIQUE (workspace_id, recipient_user_id, event_id),
 CONSTRAINT ux_notification_receipt_key UNIQUE (workspace_id, recipient_user_id, event_key)
);
CREATE INDEX IF NOT EXISTS ix_notification_receipt_category ON notification_inbox_receipts(workspace_id,recipient_user_id,category,id DESC);
CREATE INDEX IF NOT EXISTS ix_notification_receipt_recipient ON notification_inbox_receipts(workspace_id,recipient_user_id,id DESC);
CREATE INDEX IF NOT EXISTS ix_notification_receipt_unread ON notification_inbox_receipts(workspace_id,recipient_user_id,category) WHERE read_at IS NULL;
-- Preserve existing site notifications without replaying external delivery.
INSERT INTO notification_inbox_receipts(workspace_id,recipient_user_id,event_id,event_key,category)
SELECT e.workspace_id, recipient.value::bigint, e.id,
 (e.payload::jsonb->>'scene') || ':outbox:' || e.id::text,
 CASE WHEN e.event_type = 'pm.notification.document.comment_mentioned' THEN 'mention' ELSE 'access' END
FROM outbox_events e
CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(e.payload::jsonb->'recipient_user_ids','[]'::jsonb)) recipient(value)
WHERE e.event_type IN ('pm.notification.document.comment_mentioned','pm.notification.document.access_requested','pm.notification.document.access_reviewed')
 AND recipient.value ~ '^[1-9][0-9]*$'
ON CONFLICT DO NOTHING;
