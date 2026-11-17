import { ActivityDateValue, ActivityChange, StatusPill, UserMention } from '@/shared/ui'
import { PhPlus, PhChatCenteredText, PhArrowsClockwise, PhUser, PhCalendar, PhPencil } from '@/shared/ui/icons'
import { QUALITY_COPY } from '@/shared/copy'
import { qualityActivityNote, formatQualityActivityDateTime } from '../../model/presenters'
import { qualityStatusTone, qualitySeverityTone } from '../../model/status-tone'
import type { QualityActivity, QualityUser } from '../../model/types'
import { ActivitySummary } from './ActivitySummary'

export function qualityActivityIcon(activity: QualityActivity) {
  if (activity.fieldKey === 'status' || ['status_transition', 'resolved', 'returned'].includes(activity.action || '')) return <PhArrowsClockwise />
  if (activity.action === 'created') return <PhPlus />
  if (activity.action === 'comment') return <PhChatCenteredText />
  if (activity.action === 'assigned' || ['assignee_user_id','reporter_user_id','reviewer_user_id','verified_by_user_id'].includes(activity.fieldKey || '')) return <PhUser />
  if (activity.fieldKey === 'due_date') return <PhCalendar />
  return <PhPencil />
}

export function QualityActivityChange({activity, fallback, onPreview, users = []}: {
  activity: QualityActivity
  users?: readonly QualityUser[]
  fallback: string
  onPreview: (url: string, trigger: HTMLElement) => void
}) {
  if (activity.fieldKey === 'due_date') {
    const note = qualityActivityNote(activity.summary)
    return <span className="grid gap-2"><span className="flex flex-wrap items-center gap-2"><span className="text-muted-foreground">{QUALITY_COPY.activity.changedField.dueDate}</span><ActivityChange before={<ActivityDateValue value={formatQualityActivityDateTime(activity.oldValue)} />} after={<ActivityDateValue value={formatQualityActivityDateTime(activity.newValue)} />} /></span>{note ? <ActivitySummary value={note} onPreview={onPreview} /> : null}</span>
  }
  if (['assignee_user_id','reporter_user_id','reviewer_user_id','verified_by_user_id'].includes(activity.fieldKey || '')) {
    const person = (raw?: string) => {
      const id = Number(raw)
      if (!Number.isSafeInteger(id) || id <= 0) return <span className="text-muted-foreground">{QUALITY_COPY.activity.emptyValue}</span>
      const user = users.find(user=>user.id === id)
      return <UserMention id={id} name={user?.name || raw || ''} username={user?.username} />
    }
    const note = qualityActivityNote(activity.summary)
    return <span className="grid gap-2"><span className="flex flex-wrap items-center gap-2"><span className="text-muted-foreground">{QUALITY_COPY.activity.assigned}</span><ActivityChange before={person(activity.oldValue)} after={person(activity.newValue)} /></span>{note ? <ActivitySummary value={note} onPreview={onPreview} /> : null}</span>
  }
  const fields = {
    status: {label: QUALITY_COPY.activity.statusChanged, labels: QUALITY_COPY.enums.status, tone: qualityStatusTone},
    severity_key: {label: QUALITY_COPY.form.severity, labels: QUALITY_COPY.enums.severity, tone: qualitySeverityTone},
    priority: {label: QUALITY_COPY.form.priority, labels: QUALITY_COPY.enums.priority, tone: () => 'neutral' as const},
  }
  const field = fields[activity.fieldKey as keyof typeof fields]
  if (!field || (!activity.oldValue && !activity.newValue)) return <ActivitySummary value={fallback} onPreview={onPreview} />
  const pill = (value?: string) => <StatusPill tone={field.tone(value)}>{value ? field.labels[value] || value : QUALITY_COPY.activity.emptyValue}</StatusPill>
  const note = qualityActivityNote(activity.summary)
  return <span className="grid min-w-0 gap-2">
    <span className="flex flex-wrap items-center gap-2">
      <span className="text-muted-foreground">{field.label}</span>
      <ActivityChange before={pill(activity.oldValue)} after={pill(activity.newValue)} />
    </span>
    {note ? <ActivitySummary value={note} onPreview={onPreview} /> : null}
  </span>
}
