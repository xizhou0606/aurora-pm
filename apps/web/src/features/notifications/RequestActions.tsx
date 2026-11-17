import { InboxMembershipActions, InboxDocumentActions } from '@/features/resource-access'
import { NOTIFICATIONS_COPY as copy } from '@/shared/copy/notifications'
import type { Mention } from './api'

export function RequestActions({ item }: { item: Mention }) {
 if (!item.scene?.endsWith('.access_requested') || !item.request_id) return null
 const status = item.request_status
 if (status && status in copy.requestStatuses) return <p className="px-3 pb-3 text-xs text-muted-foreground">{copy.requestStatuses[status as keyof typeof copy.requestStatuses]}</p>
 if (status !== 'pending') return null
 if (item.resource_type === 'workspace' && item.resource_id && /^[1-9]\d*$/.test(item.resource_id)) return <InboxMembershipActions target={{ kind: item.resource_type, id: Number(item.resource_id) }} requestId={item.request_id} />
 if (item.resource_type === 'plan_unit' && item.resource_id && /^[1-9]\d*\/[1-9]\d*$/.test(item.resource_id)) return <InboxMembershipActions target={{ kind: 'plan_unit', id: Number(item.resource_id.split('/')[1]) }} requestId={item.request_id} />
 if ((item.resource_type === 'rich_text' || item.resource_type === 'bitable') && item.resource_id) return <InboxDocumentActions resourceId={item.resource_id} resourceType={item.resource_type} requestId={item.request_id} />
 return null
}
