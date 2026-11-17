import { ActivityDateValue, ActivityChange, StatusPill, UserMention } from '@/shared/ui'
import { PhCalendar, PhChatCenteredText, PhGitBranch, PhPencil, PhPlus, PhUser, PhArrowsClockwise } from '@/shared/ui/icons'
import { workItemStatusTone } from '@/shared/model'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { DetailResourceRow, WorkItemUserOption } from '../../model/types'
import { formatActivityDateTime } from '../../model/presenters'
import { statusLabel } from '../../model/utils'

export function StatusChange({ from, to }: { from: string; to: string }) {
  return <ActivityChange
    before={<StatusPill tone={workItemStatusTone(from)}>{statusLabel(from)}</StatusPill>}
    after={<StatusPill tone={workItemStatusTone(to)}>{statusLabel(to)}</StatusPill>}
  />
}

export function activityIcon(row: DetailResourceRow) {
  const activity = row.activity
  if (!activity) return <PhGitBranch />
  if (activity.fieldKey === 'status' || ['status_change', 'auto_complete'].includes(activity.action || '')) return <PhArrowsClockwise />
  if (activity.action === 'comment') return <PhChatCenteredText />
  if (['starts_at', 'due_at'].includes(activity.fieldKey || '')) return <PhCalendar />
  if (['assignee_id', 'reporter_user_id'].includes(activity.fieldKey || '')) return <PhUser />
  if (activity.action === 'create') return <PhPlus />
  return <PhPencil />
}

export function ActivitySummary({ row, users = [] }: { row: DetailResourceRow; users?: readonly WorkItemUserOption[] }) {
  const activity = row.activity
  if (activity && ['starts_at','due_at'].includes(activity.fieldKey || '')) {
    return <span className="flex flex-wrap items-center gap-2"><span className="text-muted-foreground">{activity.fieldKey === 'starts_at' ? WORK_ITEMS_COPY.detail.startsAt : WORK_ITEMS_COPY.detail.dueAt}</span><ActivityChange before={<ActivityDateValue value={formatActivityDateTime(activity.oldValue)} />} after={<ActivityDateValue value={formatActivityDateTime(activity.newValue)} />} /></span>
  }
  if (activity && ['assignee_id','reporter_user_id'].includes(activity.fieldKey || '')) {
    const person = (raw?: string) => {
      const id = Number(raw)
      if (!Number.isSafeInteger(id) || id <= 0) return <span className="text-muted-foreground">{WORK_ITEMS_COPY.list.unassigned}</span>
      const user = users.find(user=>user.id === id)
      return <UserMention id={id} name={user?.label || raw || ''} username={user?.username} />
    }
    return <span className="flex flex-wrap items-center gap-2"><span className="text-muted-foreground">{activity.fieldKey === 'assignee_id' ? WORK_ITEMS_COPY.detail.assignee : WORK_ITEMS_COPY.detail.reporter}</span><ActivityChange before={person(activity.oldValue)} after={person(activity.newValue)} /></span>
  }
  if (activity && (activity.fieldKey === 'status' || ['status_change', 'auto_complete'].includes(activity.action || ''))
    && (activity.oldValue !== undefined || activity.newValue !== undefined)) {
    return <span className="flex flex-wrap items-center gap-2">
      <span className="text-muted-foreground">{WORK_ITEMS_COPY.detail.status}</span>
      <StatusChange from={activity.oldValue || ''} to={activity.newValue || ''} />
    </span>
  }
  return <span className="whitespace-pre-wrap break-words">{row.primary}</span>
}
