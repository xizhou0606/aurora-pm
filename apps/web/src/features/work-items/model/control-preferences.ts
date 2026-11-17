import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { TableColumnOption } from '@/shared/model/table-column-preferences'

export type WorkItemsViewControlId = 'view' | 'layout'

export type WorkItemsFilterControlId =
  | 'search' | 'status' | 'priority' | 'client' | 'assignee' | 'group' | 'mine' | 'overdue'

export const WORK_ITEMS_VIEW_CONTROL_OPTIONS: readonly TableColumnOption<WorkItemsViewControlId>[] = [
  { id: 'view', label: WORK_ITEMS_COPY.filterSettings.viewControl, required: true },
  { id: 'layout', label: WORK_ITEMS_COPY.filterSettings.layoutControl },
]

export const WORK_ITEMS_FILTER_CONTROL_OPTIONS: readonly TableColumnOption<WorkItemsFilterControlId>[] = [
  { id: 'search', label: WORK_ITEMS_COPY.filter.searchLabel, required: true, fixed: true },
  { id: 'status', label: WORK_ITEMS_COPY.filter.status },
  { id: 'priority', label: WORK_ITEMS_COPY.filter.priority },
  { id: 'client', label: WORK_ITEMS_COPY.filter.client },
  { id: 'assignee', label: WORK_ITEMS_COPY.filter.assignee },
  { id: 'group', label: WORK_ITEMS_COPY.filter.groupBy },
  { id: 'mine', label: WORK_ITEMS_COPY.filter.mine },
  { id: 'overdue', label: WORK_ITEMS_COPY.filter.overdue },
]
