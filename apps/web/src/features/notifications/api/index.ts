import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
export type NotificationCategory = '' | 'mention' | 'message' | 'access'
export type Mention = {
 project_id?: number
 node_id?: string
 node_name?: string
 previous_status?: string
 status?: string
 member_role?: string
 recipient_role?: 'assignee' | 'cc'
 actor_name?: string
 resource_title?: string
 request_status?: string
 grant_level?: string
 id: number
 scene?: 'quality_bug.created' | 'quality_bug.assigned' | 'quality_bug.review_requested' | 'quality_bug.returned' | 'quality_bug.commented' | 'work_task.assigned' | 'work_task.commented' | 'work_task.status_changed' | 'workflow.node_assigned' | 'workflow.pre_review_invited' | 'plan_unit.member_assigned' | 'workspace.access_requested' | 'workspace.access_reviewed' | 'project.access_requested' | 'project.access_reviewed' | 'document.comment_mentioned' | 'document.access_requested' | 'document.access_reviewed' | 'work_task.comment_mentioned' | 'quality_bug.comment_mentioned'
 request_id?: number
 resource_type?: 'requirement' | 'workflow' | 'project_set' | 'workspace' | 'plan_unit' | 'rich_text' | 'bitable' | 'work_task' | 'quality_bug'
 resource_id?: string
 document_id: number
 thread_id: number
 comment_id?: number
 actor_user_id: number
 occurred_at: string
 read_at?: string
}
export type MentionPage = {items: Mention[]; next_cursor?: number; unread_count: number; high_watermark: number; unread_by_category: Record<string, number>}
export async function listInbox(category: NotificationCategory, before: number, signal?: AbortSignal): Promise<MentionPage> {
 try {
  const response = await platformApi.get('/notifications/inbox', {signal, params: {category: category || undefined, before: before || undefined}})
  const data = unwrapApiData<MentionPage>(response)
  if (!data || !Array.isArray(data.items) || !Number.isSafeInteger(data.unread_count) || !Number.isSafeInteger(data.high_watermark)) throw new Error('INBOX_CONTRACT_INVALID')
  return data
 } catch(error) {throw toSafeApiError(error, 'INBOX_REQUEST_FAILED')}
}
export async function markInboxRead(id: number, all: boolean, category: NotificationCategory = ''): Promise<void> {
 try {
  await platformApi.post(all ? '/notifications/inbox/read-all' : `/notifications/inbox/${id}/read`, all ? {through_id:id, category: category || undefined} : {})
 } catch(error) {throw toSafeApiError(error, 'INBOX_UPDATE_FAILED')}
}
