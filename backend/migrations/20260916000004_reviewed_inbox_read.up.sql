-- A completed review consumes only that reviewer's original request notification.
UPDATE notification_inbox_receipts AS receipt
SET read_at = request.reviewed_at
FROM outbox_events AS event, resource_access_requests AS request
WHERE receipt.event_id = event.id
  AND receipt.workspace_id = event.workspace_id
  AND request.workspace_id = event.workspace_id
  AND event.aggregate_id = request.id
  AND event.aggregate_type IN ('membership_access_request', 'document_access_request')
  AND event.event_type IN ('pm.notification.workspace.access_requested', 'pm.notification.project.access_requested', 'pm.notification.document.access_requested')
  AND receipt.recipient_user_id = request.reviewed_by
  AND request.status IN ('approved', 'rejected')
  AND request.reviewed_at IS NOT NULL
  AND receipt.read_at IS NULL;
