import { PhWarning, PhCalendar, PhCheck } from '@/shared/ui/icons'

import {
  isSafeApiError,
  PM_API_ERROR_CODES,
  type PMSafeSchedule,
  type PMSafeScheduleBoundary,
  type PMSafeScheduleConflict,
  type PMSafeScheduleState,
} from '@/shared/api'
import { getPMRuntimeConfig } from '@/shared/config'
import { copy } from '@/shared/copy'
import { getRuntimeSession } from '@/shared/runtime'
import { Button, StatusPill, type StatusPillTone } from '@/shared/ui'

import type { WorkflowInstance } from '../model/types'

const text = copy.aiPM.workflowSubject

function date(value?: string | null) {
  return value?.slice(0, 10) || ''
}

export function scheduleRange(boundary?: PMSafeScheduleBoundary): string {
  const start = date(boundary?.planned_start_at)
  const end = date(boundary?.planned_end_at)
  if (start && end) return `${start} - ${end}`
  if (start) return text.nodePlanStartOnly(start)
  if (end) return text.nodePlanEndOnly(end)
  return text.scheduleUnscheduled
}

function sameScheduleRange(left?: PMSafeScheduleBoundary, right?: PMSafeScheduleBoundary) {
  return date(left?.planned_start_at) === date(right?.planned_start_at) &&
    date(left?.planned_end_at) === date(right?.planned_end_at)
}

export function workflowSchedule(instance: WorkflowInstance): PMSafeSchedule {
  if (instance.subject_schedule) return instance.subject_schedule
  return {
    state: 'conflict',
    can_confirm: false,
    actual_dates_independent: true,
    conflicts: [{ code: 'SUBJECT_SCHEDULE_UNAVAILABLE' }],
  }
}

/** True when the schedule carries nothing worth showing outside node panels. */
export function workflowScheduleIdle(schedule: PMSafeSchedule): boolean {
  const showConstraints = Boolean(
    (schedule.constraints?.planned_start_at || schedule.constraints?.planned_end_at) &&
    !sameScheduleRange(schedule.subject, schedule.constraints),
  )
  return schedule.state === 'unscheduled' && !schedule.conflicts.length && !schedule.suggested && !schedule.can_confirm && !showConstraints
}

/** Inline "排期状态 [状态]" tag for header meta rows; description moves to a tooltip. */
export function WorkflowScheduleTag({ instance }: { instance: WorkflowInstance }) {
  const schedule = workflowSchedule(instance)
  if (workflowScheduleIdle(schedule)) return null
  return (
    <span data-testid="workflow-schedule-tag" data-schedule-state={schedule.state}
      title={scheduleDescription(schedule.state)} className="inline-flex shrink-0 items-center gap-1.5">
      <PhCalendar aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="text-xs text-muted-foreground">{text.scheduleTitle}</span>
      <StatusPill tone={scheduleTone(schedule.state)}>{scheduleLabel(schedule.state)}</StatusPill>
    </span>
  )
}

function calendarDate(value?: string | null) {
  const result = date(value)
  if (!/^\d{4}-\d{2}-\d{2}$/.test(result)) return ''
  const parsed = new Date(`${result}T00:00:00Z`)
  return !Number.isNaN(parsed.getTime()) && parsed.toISOString().slice(0, 10) === result ? result : ''
}

export function workflowNodeIsOverdue(
  node: Pick<WorkflowInstance['timeline_nodes'][number], 'planned_end_at' | 'status' | 'actual_completed_at'>,
  schedule?: PMSafeSchedule,
  now = new Date(),
  timezone = getRuntimeSession().timezone || getPMRuntimeConfig().timezone,
) {
  const subjectStart = calendarDate(schedule?.subject?.planned_start_at)
  const subjectEnd = calendarDate(schedule?.subject?.planned_end_at)
  const plannedEnd = calendarDate(node.planned_end_at)
  if (
    schedule?.state === 'conflict' ||
    !subjectStart || !subjectEnd || subjectStart > subjectEnd ||
    !plannedEnd || node.status === 'completed' || node.status === 'skipped' || node.actual_completed_at
  ) return false
  const calendarDayAt = (timeZone: string) => {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).formatToParts(now)
    const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((item) => item.type === type)?.value ?? ''
    return `${part('year')}-${part('month')}-${part('day')}`
  }
  let today = ''
  try {
    today = calendarDayAt(timezone)
  } catch {
    try {
      today = calendarDayAt('UTC')
    } catch {
      return false
    }
  }
  return plannedEnd < today
}

function scheduleLabel(state: PMSafeScheduleState) {
  if (state === 'scheduled') return text.scheduleScheduled
  if (state === 'partial') return text.schedulePartial
  if (state === 'conflict') return text.scheduleConflict
  return text.scheduleUnscheduled
}

function scheduleTone(state: PMSafeScheduleState): StatusPillTone {
  if (state === 'scheduled') return 'success'
  if (state === 'conflict') return 'danger'
  if (state === 'partial') return 'warning'
  return 'neutral'
}

function scheduleDescription(state: PMSafeScheduleState) {
  if (state === 'scheduled') return text.scheduleScheduledDescription
  if (state === 'partial') return text.schedulePartialDescription
  if (state === 'conflict') return text.scheduleConflictDescription
  return text.scheduleUnscheduledDescription
}

export function scheduleConflictMessage(
  conflict: PMSafeScheduleConflict,
  resolveNodeName: (id: string) => string | undefined = () => undefined,
  resolveSubjectName: (type: PMSafeScheduleConflict['subject_type'], id?: number) => string | undefined = () => undefined,
) {
  const subject = resolveSubjectName(conflict.subject_type, conflict.subject_id) || (
    conflict.subject_type === 'project_set' ? text.scheduleProjectSetFallback
      : conflict.subject_type === 'plan_unit' ? text.schedulePlanUnitFallback
        : conflict.subject_type === 'requirement' ? conflict.subject_id
          ? text.scheduleRequirementReference(conflict.subject_id)
          : text.scheduleRequirementFallback
          : text.scheduleSubjectFallback
  )
  const name = conflict.node_id
    ? resolveNodeName(conflict.node_id) || text.scheduleNodeReference(subject, conflict.node_id)
    : copy.aiPM.workflow.nodeGeneric
  const related = conflict.related_node_id
    ? resolveNodeName(conflict.related_node_id) || text.scheduleNodeReference(subject, conflict.related_node_id)
    : copy.aiPM.workflow.nodeGeneric
  const value = date(conflict.value)
  switch (conflict.code) {
    case 'SUBJECT_START_AFTER_END': return text.scheduleSubjectOrderConflict(subject)
    case 'NODE_START_AFTER_END': return text.scheduleNodeOrderConflict(name)
    case 'NODE_BEFORE_SUBJECT_START': return text.scheduleNodeBeforeStart(name, value)
    case 'NODE_AFTER_SUBJECT_END': return text.scheduleNodeAfterEnd(name, value)
    case 'SERIAL_NODE_ORDER': return text.scheduleSerialConflict(name, related)
    case 'SUBJECT_PLAN_UNIT_REQUIRED': return text.schedulePlanUnitRequired
    case 'SUBJECT_SCHEDULE_UNAVAILABLE': return text.scheduleUnavailable
    case 'SCHEDULE_CONFIRMATION_NOT_AVAILABLE': return text.scheduleSuggestionUnavailable
    case 'PROJECT_START_AFTER_END': return text.scheduleProjectOrderConflict
    case 'PLAN_UNIT_BEFORE_PROJECT_START': return text.schedulePlanUnitBeforeProject(subject)
    case 'PLAN_UNIT_AFTER_PROJECT_END': return text.schedulePlanUnitAfterProject(subject)
    case 'PLAN_UNIT_START_AFTER_END': return text.schedulePlanUnitOrderConflict(subject)
    case PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE: return text.scheduleStale
    default: return text.scheduleConflictUnknown
  }
}

export function ScheduleConflictList({
  schedule,
  resolveNodeName,
  resolveSubjectName,
}: {
  schedule?: PMSafeSchedule
  resolveNodeName?: (id: string) => string | undefined
  resolveSubjectName?: (type: PMSafeScheduleConflict['subject_type'], id?: number) => string | undefined
}) {
  if (!schedule?.conflicts.length) return null
  return <div className="grid min-w-0 gap-1.5">
    <strong className="text-xs text-red-900">{text.scheduleConflictTitle}</strong>
    <ul className="m-0 grid min-w-0 gap-1 pl-4 text-xs leading-5 text-red-800">
      {schedule.conflicts.map((conflict, index) => (
        <li key={`${conflict.code}:${conflict.node_id || ''}:${conflict.related_node_id || ''}:${index}`}>
          {scheduleConflictMessage(conflict, resolveNodeName, resolveSubjectName)}
        </li>
      ))}
    </ul>
  </div>
}

export function ScheduleMutationNotice({
  error,
  code: safeCode,
  schedule: safeSchedule,
  instance,
  pending,
  onConfirm,
  onAdjust,
  resolveSubjectName,
}: {
  error?: unknown
  code?: string
  schedule?: PMSafeSchedule
  instance?: WorkflowInstance
  pending?: boolean
  onConfirm?: () => void
  onAdjust?: () => void
  resolveSubjectName?: (type: PMSafeScheduleConflict['subject_type'], id?: number) => string | undefined
}) {
  const normalized = error === undefined
    ? { code: safeCode, schedule: safeSchedule }
    : isSafeApiError(error)
      ? { code: error.code, schedule: error.schedule }
      : undefined
  if (!normalized) return null
  const schedule = normalized.schedule
  const stale = normalized.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  const confirmation = normalized.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED && schedule?.can_confirm && schedule.suggested
  const conflict = (
    normalized.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFLICT ||
    normalized.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  ) && schedule
  if (!confirmation && !conflict) return null
  const nodeNames = new Map((instance?.timeline_nodes ?? []).map((node) => [node.id, node.name]))
  const resolveNodeName = (id: string) => nodeNames.get(id)
  if (confirmation) return <div role="alert" data-testid="workflow-schedule-confirmation" className="grid min-w-0 gap-2.5 rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/40 p-3 text-amber-950">
    <div className="flex min-w-0 items-start gap-2">
      <PhCalendar aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0">
        <strong className="block text-sm">{text.scheduleSuggestedTitle}</strong>
        <p className="mt-1 mb-0 text-xs leading-5 text-amber-800">{text.scheduleSuggestedDescription}</p>
        <p className="mt-1 mb-0 break-words text-sm font-semibold">{scheduleRange(confirmation)}</p>
      </div>
    </div>
    <div className="flex flex-wrap justify-end gap-2">
      {onAdjust ? <Button type="button" size="sm" variant="outline" disabled={pending} onClick={onAdjust}>{text.scheduleAdjust}</Button> : null}
      {onConfirm ? <Button type="button" size="sm" disabled={pending} onClick={onConfirm}>{pending ? <PhCalendar className="animate-pulse" /> : <PhCheck />}{pending ? text.scheduleConfirming : text.scheduleConfirmSuggested}</Button> : null}
    </div>
  </div>
  return <div role="alert" data-testid="workflow-schedule-conflict" className="grid min-w-0 gap-2.5 rounded-md border border-red-200 dark:border-red-800 bg-red-50 p-3 text-red-950">
    <div className="flex min-w-0 items-start gap-2">
      <PhWarning aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0"><strong className="block text-sm">{text.scheduleConflict}</strong><p className="mt-1 mb-0 text-xs leading-5 text-red-800">{text.scheduleConflictDescription}</p></div>
    </div>
    <ScheduleConflictList schedule={schedule} resolveNodeName={resolveNodeName} resolveSubjectName={resolveSubjectName} />
    {onAdjust ? <Button type="button" size="sm" variant="outline" className="justify-self-end" disabled={pending} onClick={onAdjust}>{stale ? text.scheduleReloadLatest : text.scheduleAdjust}</Button> : null}
  </div>
}

export function WorkflowScheduleSummary({
  instance,
  pending,
  compact,
  hideWhenIdle,
  onConfirmSuggestion,
}: {
  instance: WorkflowInstance
  pending?: boolean
  compact?: boolean
  hideWhenIdle?: boolean
  onConfirmSuggestion?: () => void
}) {
  const schedule = workflowSchedule(instance)
  const canConfirm = Boolean(schedule.can_confirm && schedule.suggested && onConfirmSuggestion && schedule.state !== 'scheduled' && schedule.state !== 'conflict')
  const showConstraints = Boolean(
    (schedule.constraints?.planned_start_at || schedule.constraints?.planned_end_at) &&
    !sameScheduleRange(schedule.subject, schedule.constraints),
  )
  const compactHasSubjectRange = Boolean(date(schedule.subject?.planned_start_at) || date(schedule.subject?.planned_end_at))
  if (compact) {
    if (hideWhenIdle && !canConfirm && !schedule.conflicts.length) return null
    return <section data-testid="workflow-schedule-summary" data-schedule-state={schedule.state} data-density="compact" className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
      <PhCalendar aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
      <strong className="text-xs font-medium text-muted-foreground">{text.scheduleTitle}</strong>
      <StatusPill tone={scheduleTone(schedule.state)}>{scheduleLabel(schedule.state)}</StatusPill>
      {compactHasSubjectRange ? <span className="min-w-0 truncate text-xs text-foreground">{scheduleRange(schedule.subject)}</span> : null}
      {canConfirm ? <Button type="button" size="xs" variant="outline" className="ml-auto" disabled={pending} onClick={onConfirmSuggestion}>{pending ? text.scheduleConfirming : text.scheduleConfirmSuggested}</Button> : null}
      {schedule.conflicts.length ? <div className="basis-full"><ScheduleConflictList schedule={schedule} resolveNodeName={(id) => instance.timeline_nodes.find((node) => node.id === id)?.name} /></div> : null}
    </section>
  }
  if (schedule.state === 'unscheduled' && !schedule.conflicts.length && !canConfirm && !schedule.suggested && !showConstraints) return null
  return <section data-testid="workflow-schedule-summary" data-schedule-state={schedule.state} className="grid min-w-0 gap-2 border-y border-border bg-muted/70 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
    <div className="flex min-w-0 items-start gap-2">
      <PhCalendar aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-1.5"><strong className="text-xs text-foreground">{text.scheduleTitle}</strong><StatusPill tone={scheduleTone(schedule.state)}>{scheduleLabel(schedule.state)}</StatusPill></div>
        <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{scheduleDescription(schedule.state)}</p>
        {schedule.subject ? <p className="mt-1 mb-0 break-words text-xs font-medium text-foreground">{text.scheduleMeta(text.scheduleSubjectRange, scheduleRange(schedule.subject))}</p> : null}
        {showConstraints ? <p className="mt-1 mb-0 break-words text-xs font-medium text-foreground">{text.scheduleMeta(text.scheduleConstraintRange, scheduleRange(schedule.constraints))}</p> : null}
        {schedule.suggested ? <p className="mt-1 mb-0 break-words text-xs font-medium text-amber-800">{text.scheduleMeta(text.scheduleSuggestedTitle, scheduleRange(schedule.suggested))}</p> : null}
        {schedule.actual_dates_independent ? <p className="mt-1 mb-0 text-xs leading-4 text-muted-foreground">{text.scheduleActualIndependent}</p> : null}
        <ScheduleConflictList schedule={schedule} resolveNodeName={(id) => instance.timeline_nodes.find((node) => node.id === id)?.name} />
      </div>
    </div>
    {canConfirm ? <Button type="button" size="sm" variant="outline" className="w-full sm:w-auto" disabled={pending} onClick={onConfirmSuggestion}>{pending ? text.scheduleConfirming : text.scheduleConfirmSuggested}</Button> : null}
  </section>
}
