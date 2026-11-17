import { QUALITY_COPY } from '@/shared/copy'
import { qualityReviewerOwnsStatus } from './filters'

import type {
  QualityActivity,
  QualityActivityPage,
  QualityBug,
  QualityFilterOption,
  QualityLifecycleAction,
  QualityLifecycleModel,
  QualityMutationState,
} from './types'

function statusLabel(status: string | undefined): string {
  return status
    ? QUALITY_COPY.enums.status[status] || status
    : QUALITY_COPY.list.unknown
}

const RETIRED_QUALITY_STATUS_VALUES = new Set(['canceled', 'cancelled'])

export function qualityStatusFilterOptions(selectedValue = ''): QualityFilterOption[] {
  const byLabel = new Map<string, QualityFilterOption>()
  for (const [value, label] of Object.entries(QUALITY_COPY.enums.status)) {
    if (RETIRED_QUALITY_STATUS_VALUES.has(value)) continue
    const current = byLabel.get(label)
    if (!current || value === selectedValue) byLabel.set(label, { value, label })
  }
  return [...byLabel.values()]
}

export function qualityAllowedTransitionOptions(bug: QualityBug): QualityFilterOption[] {
  if (!bug.capabilities.canTransition && !bug.capabilities.canReturn) return []
  const labels = new Set<string>()
  const transitions = [
    ...bug.capabilities.allowedTransitions,
    ...(bug.capabilities.canReturn ? ['reopened'] : []),
  ]
  return transitions.flatMap((value) => {
    if (!value || value === bug.status || RETIRED_QUALITY_STATUS_VALUES.has(value)) return []
    const label = statusLabel(value)
    if (labels.has(label)) return []
    labels.add(label)
    return [{ value, label }]
  })
}

export function qualityCurrentOwnerUserId(bug: QualityBug): number | undefined {
  return qualityReviewerOwnsStatus(bug.status)
    ? bug.reviewerUserId || bug.reporterUserId
    : bug.assigneeUserId
}

function transitionLabel(status: string): string {
  if (status === 'in_progress') return QUALITY_COPY.lifecycle.confirm
  if (status === 'done' || status === 'closed') return QUALITY_COPY.lifecycle.pass
  if (['todo', 'triage'].includes(status)) return QUALITY_COPY.lifecycle.restore
  return QUALITY_COPY.lifecycle.transitionTo(statusLabel(status))
}

function transitionActions(
  bug: QualityBug,
  transition: QualityMutationState,
): QualityLifecycleAction[] {
  if (!bug.capabilities.canTransition) return []
  const labels = new Set<string>()
  return bug.capabilities.allowedTransitions.flatMap((status) => {
    if (!status || status === bug.status) return []
    const label = transitionLabel(status)
    if (labels.has(label)) return []
    labels.add(label)
    return [{
      id: `transition:${status}`,
      kind: 'transition' as const,
      status,
      label,
      variant: status === 'in_progress' ? 'default' as const : 'outline' as const,
      disabled: transition.status === 'pending',
    }]
  })
}

export function qualityLifecycleModel(
  bug: QualityBug,
  transition: QualityMutationState,
): QualityLifecycleModel {
  const transitions = transitionActions(bug, transition)
  const byStatus = new Map(
    transitions.map((action) => [action.kind === 'transition' ? action.status : '', action]),
  )
  const used = new Set<string>()
  const actions: QualityLifecycleAction[] = []
  const appendTransition = (status: string) => {
    const action = byStatus.get(status)
    if (!action || used.has(action.id)) return
    used.add(action.id)
    actions.push(action)
  }
  appendTransition('in_progress')
  if (bug.capabilities.canAssign) actions.push({
    id: 'assign', kind: 'assign', label: QUALITY_COPY.lifecycle.assign,
    variant: 'outline', disabled: false,
  })
  if (bug.capabilities.canResolve) actions.push({
    id: 'resolve', kind: 'resolve', label: QUALITY_COPY.lifecycle.resolve,
    variant: 'outline', disabled: false,
  })
  appendTransition('done')
  appendTransition('closed')
  if (bug.capabilities.canReturn) actions.push({
    id: 'return', kind: 'return', label: QUALITY_COPY.lifecycle.return,
    variant: 'outline', disabled: false,
  })
  appendTransition('triage')
  appendTransition('todo')
  for (const action of transitions) {
    if (!used.has(action.id)) actions.push(action)
  }

  return {
    currentLabel: QUALITY_COPY.lifecycle.current(statusLabel(bug.status)),
    actions,
    transitionError: transition.status === 'error',
  }
}

export function mergeQualityActivityPages(
  pages: readonly QualityActivityPage[] | undefined,
): QualityActivity[] {
  const seen = new Set<number>()
  return (pages || []).flatMap((page) => page.rows.flatMap((row) => {
    if (seen.has(row.id)) return []
    seen.add(row.id)
    return [row]
  }))
}
