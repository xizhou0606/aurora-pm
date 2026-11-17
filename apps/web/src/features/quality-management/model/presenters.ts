import { QUALITY_COPY } from '@/shared/copy'
import type { PMScopeOptions } from '@/shared/api'
import type { QualityActivity, QualityBug, QualityScope, QualityUser } from './types'
import { qualityRichTextPlainText } from './rich-text'
import { safeMaterialUrl } from './values'

const EMPTY_REQUIREMENT_SELECTION: QualityScope = {
  projectSetIds: [],
  projectIds: [],
  requirementIds: [],
}

export function qualityBugDirectoryUserIds(
  bugs: readonly Pick<QualityBug, 'assigneeUserId' | 'reporterUserId' | 'reviewerUserId'>[],
  selectedIds: readonly number[] = [],
): number[] {
  const ids = new Set<number>()
  for (const bug of bugs) {
    for (const id of [bug.assigneeUserId, bug.reporterUserId, bug.reviewerUserId]) {
      if (Number.isInteger(id) && Number(id) > 0) ids.add(Number(id))
    }
  }
  for (const id of selectedIds) {
    if (Number.isInteger(id) && id > 0) ids.add(id)
  }
  return [...ids].sort((left, right) => left - right)
}

export function qualityRequirementSelection(
  options: PMScopeOptions | null,
  requirementId?: number,
): QualityScope {
  if (!options || !requirementId) return EMPTY_REQUIREMENT_SELECTION
  const requirement = options.requirements.find((entry) => entry.id === requirementId)
  const project = requirement?.parentId
    ? options.projects.find((entry) => entry.id === requirement.parentId)
    : undefined
  const projectSet = project?.parentId
    ? options.projectSets.find((entry) => entry.id === project.parentId)
    : undefined
  if (!requirement || !project || !projectSet) return EMPTY_REQUIREMENT_SELECTION
  return {
    projectSetIds: [projectSet.id],
    projectIds: [project.id],
    requirementIds: [requirement.id],
  }
}

export function qualityRequirementLabel(
  options: PMScopeOptions | null,
  requirementId?: number,
): string | undefined {
  const selection = qualityRequirementSelection(options, requirementId)
  if (!selection.requirementIds.length || !options) return undefined
  const projectSet = options.projectSets.find((entry) => entry.id === selection.projectSetIds[0])
  const project = options.projects.find((entry) => entry.id === selection.projectIds[0])
  const requirement = options.requirements.find((entry) => entry.id === selection.requirementIds[0])
  return [projectSet?.label, project?.label, requirement?.label].filter(Boolean).join(' / ')
}

export function materialDisplayName(value: unknown): string {
  const safeUrl = safeMaterialUrl(value)
  if (!safeUrl) return QUALITY_COPY.detail.materials
  try {
    const name = new URL(safeUrl, 'http://localhost').pathname.split('/').pop() || ''
    return decodeURIComponent(name) || QUALITY_COPY.detail.materials
  } catch {
    return QUALITY_COPY.detail.materials
  }
}

function normalizedIdentity(value: string | undefined): string {
  return value?.trim().toLocaleLowerCase() || ''
}

/**
 * Quality activity rows currently expose the PM-internal user id while the
 * platform directory exposes the canonical external fact id. A server-provided
 * display name is therefore the safe join key; only rows without a name may
 * fall back to the numeric id.
 */
export function qualityActivityUser(
  activity: Pick<QualityActivity, 'userId' | 'userName'>,
  users: readonly QualityUser[],
): QualityUser | undefined {
  const actorName = normalizedIdentity(activity.userName)
  if (actorName) {
    return users.find((user) => (
      normalizedIdentity(user.name) === actorName
      || normalizedIdentity(user.username) === actorName
    ))
  }
  return activity.userId
    ? users.find((user) => user.id === activity.userId)
    : undefined
}

export function qualityActivitySummary(
  activity: Pick<QualityActivity, 'action' | 'fieldKey' | 'oldValue' | 'newValue' | 'summary'>,
  fallback: string,
  users: readonly QualityUser[] = [],
  scopeOptions: PMScopeOptions | null = null,
): string {
  if (activity.action === 'comment') {
    return activity.summary?.trim()
      ? `${QUALITY_COPY.activity.commented}\n${qualityActivityRichText(activity.summary)}`
      : QUALITY_COPY.activity.commented
  }
  if (activity.fieldKey === 'status' && activity.newValue) {
    const previous = activity.oldValue ? QUALITY_COPY.enums.status[activity.oldValue] || activity.oldValue : QUALITY_COPY.activity.emptyValue
    const next = QUALITY_COPY.enums.status[activity.newValue] || activity.newValue
    return activitySummaryWithNote(
      `${QUALITY_COPY.activity.statusChanged}：${previous} → ${next}`,
      activity.summary,
    )
  }
  if ((activity.fieldKey === 'priority' || activity.fieldKey === 'severity_key') && activity.newValue) {
    const dictionary = activity.fieldKey === 'priority' ? QUALITY_COPY.enums.priority : QUALITY_COPY.enums.severity
    const previous = activity.oldValue ? dictionary[activity.oldValue] || activity.oldValue : QUALITY_COPY.activity.emptyValue
    const next = dictionary[activity.newValue] || activity.newValue
    return activitySummaryWithNote(
      `${QUALITY_COPY.activity.fieldChanged}：${previous} → ${next}`,
      activity.summary,
    )
  }
  if (['assignee_user_id', 'reporter_user_id', 'reviewer_user_id', 'verified_by_user_id'].includes(activity.fieldKey || '')) {
    const user = users.find((entry) => String(entry.id) === activity.newValue)
    return activitySummaryWithNote(
      user ? `${QUALITY_COPY.activity.assigned}：${user.name}` : QUALITY_COPY.activity.assigned,
      activity.summary,
    )
  }
  if (activity.fieldKey === 'due_date') {
    return activitySummaryWithNote(activityChangeSummary(
      QUALITY_COPY.activity.changedField.dueDate,
      formatQualityActivityDateTime(activity.oldValue),
      formatQualityActivityDateTime(activity.newValue),
    ), activity.summary)
  }
  if (activity.fieldKey === 'requirement_id') {
    return activitySummaryWithNote(activityChangeSummary(
      QUALITY_COPY.activity.changedField.requirement,
      qualityActivityRequirementName(activity.oldValue, scopeOptions),
      qualityActivityRequirementName(activity.newValue, scopeOptions),
    ), activity.summary)
  }
  if (activity.fieldKey !== 'reproduction_steps') {
    return formatQualityActivityChangeSummary(activity.summary || fallback, scopeOptions)
  }
  const previous = qualityActivityRichText(activity.oldValue)
  const next = qualityActivityRichText(activity.newValue)
  return QUALITY_COPY.activity.richTextChanged(
    QUALITY_COPY.detail.reproduction,
    previous,
    next,
  )
}

function activityChangeSummary(field: string, previous: string, next: string): string {
  return `${QUALITY_COPY.activity.fieldChanged}：${field}：${previous} → ${next}`
}

const QUALITY_ACTIVITY_NOTE_PREFIX = '备注：'

function splitQualityActivityNote(summary: string): { summary: string; note?: string } {
  const segmentPrefix = `；${QUALITY_ACTIVITY_NOTE_PREFIX}`
  const segmentIndex = summary.lastIndexOf(segmentPrefix)
  if (segmentIndex >= 0) {
    return {
      summary: summary.slice(0, segmentIndex),
      note: summary.slice(segmentIndex + segmentPrefix.length),
    }
  }
  if (summary.startsWith(QUALITY_ACTIVITY_NOTE_PREFIX)) {
    return { summary: '', note: summary.slice(QUALITY_ACTIVITY_NOTE_PREFIX.length) }
  }
  return { summary }
}

function activitySummaryWithNote(summary: string, source?: string): string {
  const note = qualityActivityNote(source)
  return note ? `${summary}\n${note}` : summary
}

export function qualityActivityNote(source?: string): string {
  const note = normalizedQualityActivityNote(splitQualityActivityNote(source || '').note)
  return note ? `${QUALITY_COPY.activity.note}：${qualityActivityRichText(note)}` : ''
}

function normalizedQualityActivityNote(source?: string): string {
  const note = source?.trim() || ''
  const values = activityChangeValues(note)
  if (!values) return note
  const previous = values.previous.trim()
  const next = values.next.trim()
  const serializedChange = previous === QUALITY_COPY.activity.emptyValue
    || previous.startsWith('{"type":"doc"')
    || next.startsWith('{"type":"doc"')
  return serializedChange ? next : note
}

function qualityActivityRequirementName(
  raw: string | undefined,
  options: PMScopeOptions | null,
): string {
  const value = raw?.trim() || ''
  if (!value || value === '0') return QUALITY_COPY.activity.emptyValue
  const requirementId = Number(value)
  if (!Number.isSafeInteger(requirementId) || requirementId <= 0) return value
  return options?.requirements.find((requirement) => requirement.id === requirementId)?.label || value
}

export function formatQualityActivityDateTime(raw: string | undefined): string {
  const value = raw?.trim() || ''
  if (!value) return QUALITY_COPY.activity.emptyValue
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const parts = Object.fromEntries(new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(date).map((part) => [part.type, part.value]))
  return `${parts.year}/${parts.month}/${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`
}

function formatQualityActivityChangeSummary(
  summary: string,
  options: PMScopeOptions | null,
): string {
  const activity = splitQualityActivityNote(summary)
  const dueDatePrefix = `${QUALITY_COPY.activity.changedField.dueDate}：`
  const requirementPrefix = `${QUALITY_COPY.activity.changedField.requirement}：`
  const reproductionPrefix = `${QUALITY_COPY.detail.reproduction}：`
  const formatted = activity.summary.split('；').map((segment) => {
    if (segment.startsWith(dueDatePrefix)) {
      const [previous, next] = segment.slice(dueDatePrefix.length).split(' → ')
      if (next === undefined) return segment
      return `${dueDatePrefix}${formatQualityActivityDateTime(previous)} → ${formatQualityActivityDateTime(next)}`
    }
    if (segment.startsWith(requirementPrefix)) {
      const [previous, next] = segment.slice(requirementPrefix.length).split(' → ')
      if (next === undefined) return segment
      return `${requirementPrefix}${qualityActivityRequirementName(previous, options)} → ${qualityActivityRequirementName(next, options)}`
    }
    if (segment.startsWith(reproductionPrefix)) {
      const values = activityChangeValues(segment.slice(reproductionPrefix.length))
      if (!values) return segment
      return `${reproductionPrefix}${qualityActivityRichText(values.previous)} → ${qualityActivityRichText(values.next)}`
    }
    return segment
  }).join('；')
  const note = normalizedQualityActivityNote(activity.note)
  return note
    ? `${formatted}\n${QUALITY_COPY.activity.note}：${qualityActivityRichText(note)}`
    : formatted
}

function activityChangeValues(value: string): { previous: string; next: string } | null {
  const separatorIndex = value.indexOf(' → ')
  if (separatorIndex < 0) return null
  return {
    previous: value.slice(0, separatorIndex),
    next: value.slice(separatorIndex + 3),
  }
}

function qualityActivityRichText(value: string | undefined): string {
  return qualityRichTextPlainText(value, {
    imageLabel: QUALITY_COPY.activity.image,
    imageText: (url) => url
      ? `${QUALITY_COPY.activity.image} ${url}`
      : QUALITY_COPY.activity.image,
  })
    .replace(/\n{2,}/g, '\n')
    || QUALITY_COPY.activity.emptyValue
}
