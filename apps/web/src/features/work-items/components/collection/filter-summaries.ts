import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { ActiveFilter } from '@/shared/ui'
import type { UserOptionsState, WorkItemsRouteState, WorkItemStatusOption, WorkItemClientOption } from '../../model/types'
import { WORK_ITEM_PRIORITY_OPTIONS } from '../../model/utils'

export function taskFilterSummaries(
  route: WorkItemsRouteState,
  statuses: readonly WorkItemStatusOption[],
  users: UserOptionsState,
  clients: readonly WorkItemClientOption[],
  onChange: (patch: Partial<WorkItemsRouteState>) => void,
): ActiveFilter[] {
  const result: ActiveFilter[] = []
  const add = (id: string, label: string, value: string, patch: Partial<WorkItemsRouteState>) => {
    if (value) result.push({ id, label: `${label}：${value}`, onRemove: () => onChange(patch) })
  }
  add('kinds', WORK_ITEMS_COPY.filter.kinds, route.filterKinds?.join('、') ?? '', { filterKinds: [] })
  add('search', WORK_ITEMS_COPY.filter.searchLabel, route.search, { search: '' })
  add('status', WORK_ITEMS_COPY.filter.status,
    route.statuses.map((id) => statuses.find((item) => item.value === id)?.label ?? id).join('、'), { statuses: [] })
  add('priority', WORK_ITEMS_COPY.filter.priority,
    route.priorities.map((id) => WORK_ITEM_PRIORITY_OPTIONS.find((item) => item.value === id)?.label ?? id).join('、'), { priorities: [] })
  add('client', WORK_ITEMS_COPY.filter.client,
    route.taskClientIds.map((id) => clients.find((item) => item.id === id)?.name ?? String(id)).join('、'), { taskClientIds: [] })
  add('assignee', WORK_ITEMS_COPY.filter.assignee,
    route.assigneeIds.map((id) => users.status === 'ready' ? users.data.find((item) => item.id === id)?.label ?? String(id) : String(id)).join('、'), { assigneeIds: [] })
  if (route.overdue) result.push({ id: 'overdue', label: WORK_ITEMS_COPY.filter.overdue, onRemove: () => onChange({ overdue: false }) })
  if (route.groupBy !== 'none') {
    const labels = { requirement_id: WORK_ITEMS_COPY.filter.requirement, assignee_id: WORK_ITEMS_COPY.filter.assigneeGroup, priority: WORK_ITEMS_COPY.filter.priorityGroup }
    add('group', WORK_ITEMS_COPY.filter.groupBy, labels[route.groupBy], { groupBy: 'none' })
  }
  return result
}
