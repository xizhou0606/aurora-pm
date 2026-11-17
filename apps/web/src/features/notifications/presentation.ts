import { NOTIFICATIONS_COPY as copy } from '@/shared/copy/notifications'
import { QUALITY_COPY } from '@/shared/copy/quality-management'
import { workItemStatusLabel, isCanonicalWorkItemStatus } from '@/shared/model'
import type { Mention } from './useMentions'

function messageRoute(item: Mention): string | undefined {
  const scene = item.scene ?? ''
  const id = item.resource_id ?? ''
  const positiveId = /^[1-9]\d*$/.test(id)
  if (scene.startsWith('quality_bug.') || scene.startsWith('work_task.')) {
    const task = scene.startsWith('work_task.')
    if (!positiveId || item.resource_type !== (task ? 'work_task' : 'quality_bug')) return undefined
    const path = `/workspace/${task ? 'work-items' : 'quality-management/bugs'}/${id}`
    if (scene.endsWith('.commented')) {
      if (!Number.isSafeInteger(item.comment_id) || (item.comment_id ?? 0) <= 0) return undefined
      return `${path}#comment-${item.comment_id}`
    }
    return scene === 'work_task.status_changed' || scene === 'quality_bug.returned' || scene === 'quality_bug.review_requested' ? `${path}#activity` : path
  }
  const node = item.node_id ? `?workflow_node=${encodeURIComponent(item.node_id)}` : ''
  if (item.resource_type === 'plan_unit' && /^[1-9]\d*\/[1-9]\d*$/.test(id)) {
    const [project, unit] = id.split('/')
    return `/workspace/projects/${project}/items/${unit}${scene.startsWith('workflow.') ? node : ''}`
  }
  if (scene.startsWith('workflow.') && item.resource_type === 'requirement' && positiveId) return `/workspace/requirements/${id}${node}`
  return undefined
}

function messageDetail(item: Mention) {
  if (item.node_name?.trim()) return copy.node(item.node_name.trim())
  if (!item.previous_status || !item.status) return undefined
  const label = (status: string) => item.resource_type === 'quality_bug'
    ? QUALITY_COPY.enums.status[status]
    : isCanonicalWorkItemStatus(status) ? workItemStatusLabel(status, '') : undefined
  const previous = label(item.previous_status)
  const next = label(item.status)
  return previous && next ? copy.statusChange(previous, next) : undefined
}

function notificationRoute(item: Mention) {
  const scene = item.scene ?? 'document.comment_mentioned'
  if (scene === 'workspace.access_requested' || scene === 'workspace.access_reviewed') {
    if (item.resource_type !== 'workspace' || !item.resource_id || !/^[1-9]\d*$/.test(item.resource_id)) return undefined
    return { title: scene === 'workspace.access_requested' ? copy.accessRequested : copy.accessReviewed, action: copy.reviewAccess, href: `/workspace/inbox#workspace-requests-${item.resource_id}` }
  }
  if (scene === 'project.access_requested' || scene === 'project.access_reviewed') {
    if (item.resource_type !== 'plan_unit' || !item.resource_id || !/^[1-9]\d*\/[1-9]\d*$/.test(item.resource_id)) return undefined
    const [, unitId] = item.resource_id.split('/')
    return { title: scene === 'project.access_requested' ? copy.accessRequested : copy.accessReviewed, action: copy.reviewAccess, href: `/workspace/inbox#project-requests-${unitId}` }
  }
  if (scene === 'work_task.comment_mentioned' || scene === 'quality_bug.comment_mentioned') {
    const id = Number(item.resource_id)
    if (!Number.isSafeInteger(id) || id <= 0 || !Number.isSafeInteger(item.comment_id) || (item.comment_id ?? 0) <= 0) return undefined
    const task = scene === 'work_task.comment_mentioned'
    if (item.resource_type !== (task ? 'work_task' : 'quality_bug')) return undefined
    return {title: task ? copy.taskMention : copy.bugMention, action:copy.open,
      href: `/workspace/${task ? 'work-items' : 'quality-management/bugs'}/${id}#comment-${item.comment_id}`}
  }
  if (item.resource_type === 'bitable' && (scene === 'document.access_requested' || scene === 'document.access_reviewed')) {
    if (!item.resource_id || !/^[a-zA-Z0-9_-]{1,200}$/.test(item.resource_id)) return undefined
    return { title: scene === 'document.access_requested' ? copy.accessRequested : copy.accessReviewed,
      action: scene === 'document.access_requested' ? copy.reviewAccess : copy.viewAccess,
      href: `/workspace/bitables/${encodeURIComponent(item.resource_id)}#access-requests` }
  }
  if (item.resource_type && item.resource_type !== 'rich_text') return undefined
  if (!Number.isSafeInteger(item.document_id) || item.document_id <= 0) return undefined
  const documentPath = `/workspace/documents/${item.document_id}`
  switch (scene) {
    case 'document.access_requested':
      return { title: copy.accessRequested, action: copy.reviewAccess, href: `${documentPath}#access-requests` }
    case 'document.access_reviewed':
      return { title: copy.accessReviewed, action: copy.viewAccess, href: `${documentPath}#access-requests` }
    case 'document.comment_mentioned':
      if (!Number.isSafeInteger(item.thread_id) || item.thread_id <= 0) return undefined
      return { title: copy.mention, action: copy.open, href: `${documentPath}#comment-${item.thread_id}` }
    default:
      return undefined
  }
}

export function notificationPresentation(item: Mention) {
  const scene = item.scene ?? 'document.comment_mentioned'
  if (Object.hasOwn(copy.messageActions, scene)) {
    const href = messageRoute(item)
    if (!href) return undefined
    const actor = item.actor_name?.trim() || copy.actor(item.actor_user_id)
    const action = scene === 'quality_bug.assigned' && item.recipient_role === 'cc'
      ? copy.assignmentCopied : scene === 'plan_unit.member_assigned' && item.member_role === 'owner'
      ? copy.ownerAssigned : copy.messageActions[scene as keyof typeof copy.messageActions]
    return { href, title: copy.messageTitle(actor, action),
      resourceTitle: item.resource_title?.trim() || copy.resource(item.resource_id ?? ''),
      detail: messageDetail(item), approval: false }
  }
  const display = notificationRoute(item)
  if (!display) return undefined
  const kind = copy.types[item.resource_type ?? 'rich_text']
  const title = item.resource_title?.trim() || copy.resource(item.resource_id || item.document_id)
  const actor = item.actor_name?.trim() || copy.actor(item.actor_user_id)
  const base = { ...display, resourceTitle: undefined, detail: undefined, approval: scene.endsWith('.access_requested') || scene.endsWith('.access_reviewed') }
  if (scene.endsWith('.access_requested')) {
    const level = item.grant_level && item.grant_level in copy.levels ? copy.levels[item.grant_level as keyof typeof copy.levels] : ''
    return { ...base, title: copy.requestTitle(actor, title, kind, level) }
  }
  if (scene.endsWith('.access_reviewed')) return { ...base, title: copy.reviewedTitle(title, kind) }
  return { ...base, title: copy.mentionTitle(actor, title, kind) }
}
