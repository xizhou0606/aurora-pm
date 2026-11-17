import { QUALITY_COPY } from '@/shared/copy'
import type { TableColumnOption } from '@/shared/model/table-column-preferences'

import { QUALITY_UNCLOSED_STATUS_FILTER } from './filters'

export type QualityStatusTabId =
  | 'all' | 'unclosed' | 'todo' | 'triage' | 'in_progress'
  | 'reopened' | 'review' | 'done'

export type QualityFilterControlId =
  | 'search' | 'severity' | 'environment' | 'assignee' | 'owner' | 'reporter'
  | 'created-range'

export const QUALITY_STATUS_TAB_OPTIONS: readonly TableColumnOption<QualityStatusTabId>[] = [
  { id: 'all', label: QUALITY_COPY.tabs.all, required: true },
  { id: 'unclosed', label: QUALITY_COPY.tabs.unclosed },
  { id: 'todo', label: QUALITY_COPY.tabs.todo },
  { id: 'triage', label: QUALITY_COPY.tabs.triage },
  { id: 'in_progress', label: QUALITY_COPY.tabs.handling },
  { id: 'reopened', label: QUALITY_COPY.tabs.reopened },
  { id: 'review', label: QUALITY_COPY.tabs.review },
  { id: 'done', label: QUALITY_COPY.tabs.done },
]

export const QUALITY_STATUS_TAB_VALUES: Readonly<Record<QualityStatusTabId, string>> = {
  all: '',
  unclosed: QUALITY_UNCLOSED_STATUS_FILTER,
  todo: 'todo',
  triage: 'triage',
  in_progress: 'in_progress',
  reopened: 'reopened',
  review: 'review',
  done: 'done',
}

export const QUALITY_FILTER_CONTROL_OPTIONS: readonly TableColumnOption<QualityFilterControlId>[] = [
  { id: 'search', label: QUALITY_COPY.filters.search },
  { id: 'severity', label: QUALITY_COPY.filters.severity },
  { id: 'environment', label: QUALITY_COPY.filters.environment },
  { id: 'assignee', label: QUALITY_COPY.filters.assignee },
  { id: 'owner', label: QUALITY_COPY.filters.owner },
  { id: 'reporter', label: QUALITY_COPY.filters.reporter },
  { id: 'created-range', label: QUALITY_COPY.filters.createdRange },
]

export const QUALITY_RELATION_TAB_OPTIONS = [
  { id: '', label: QUALITY_COPY.relations.all },
  { id: 'reported', label: QUALITY_COPY.relations.reported },
  { id: 'assigned', label: QUALITY_COPY.relations.assigned },
  { id: 'responsible', label: QUALITY_COPY.relations.responsible },
  { id: 'resolved', label: QUALITY_COPY.relations.resolved },
] as const
