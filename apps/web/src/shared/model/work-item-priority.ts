export type CanonicalWorkItemPriority = 'low' | 'normal' | 'high' | 'urgent'
export type WorkItemPriorityTone = 'neutral' | 'info' | 'attention' | 'danger'

const PRIORITY_TONES: Readonly<Record<CanonicalWorkItemPriority, WorkItemPriorityTone>> = {
  low: 'neutral',
  normal: 'info',
  high: 'attention',
  urgent: 'danger',
}

export function workItemPriorityTone(value: unknown): WorkItemPriorityTone {
  const priority = String(value || '').trim().toLocaleLowerCase() as CanonicalWorkItemPriority
  return PRIORITY_TONES[priority] || 'neutral'
}
