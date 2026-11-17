import { WORK_ITEMS_COPY } from '@/shared/copy'
import { normalizeWorkItemStatus } from '@/shared/model'
import type {
  WorkItemsGroupBy,
  WorkItemsRouteState,
  WorkItemsScope,
} from './types'
import { normalizeWorkItemPriority } from './presenters'

export const DEFAULT_PAGE_SIZE = 50 as const
export const PAGE_SIZES = [10, 20, 50, 100] as const

function stringValue(value: unknown): string {
  if (typeof value === 'string') return value.trim()
  return typeof value === 'number' || typeof value === 'boolean' ? String(value) : ''
}

function trueFlag(value: unknown): boolean {
  return value === true || value === 1 || value === '1' || value === 'true'
}

function positiveId(value: unknown): number | undefined {
  const parsed = Number(Array.isArray(value) ? value[0] : value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
}

function idList(value: unknown): { values: number[]; valid: boolean } {
  if (value === undefined) return { values: [], valid: true }
  const sources = Array.isArray(value) ? value.map(String) : [String(value)]
  let canonical: number[] | undefined
  for (const source of sources) {
    const parts = source.split(',').map((part) => part.trim())
    if (!parts.length || parts.some((part) => !/^[1-9]\d*$/.test(part))) {
      return { values: [], valid: false }
    }
    const values = parts.map(Number)
    if (values.some((id) => !Number.isSafeInteger(id) || id <= 0)
      || new Set(values).size !== values.length) {
      return { values: [], valid: false }
    }
    if (canonical && (canonical.length !== values.length
      || canonical.some((id, index) => id !== values[index]))) {
      return { values: [], valid: false }
    }
    canonical = values
  }
  return { values: canonical || [], valid: Boolean(canonical?.length) }
}

function stringList(value: unknown): string[] {
  const source = Array.isArray(value) ? value.join(',') : stringValue(value)
  const seen = new Set<string>()
  return source.split(',').flatMap((part) => {
    const item = part.trim().toLowerCase().slice(0, 64)
    if (!item || seen.has(item)) return []
    seen.add(item)
    return [item]
  })
}

function canonicalEnumList(
  value: unknown,
  normalize: (entry: unknown) => string,
  allowed: Readonly<Record<string, string>>,
): string[] {
  const seen = new Set<string>()
  return stringList(value).flatMap((entry) => {
    const normalized = normalize(entry)
    if (!Object.hasOwn(allowed, normalized) || seen.has(normalized)) return []
    seen.add(normalized)
    return [normalized]
  })
}

function pageSize(value: unknown): (typeof PAGE_SIZES)[number] {
  const parsed = Number(value)
  return PAGE_SIZES.includes(parsed as (typeof PAGE_SIZES)[number])
    ? parsed as (typeof PAGE_SIZES)[number]
    : DEFAULT_PAGE_SIZE
}

function detailIdFromPath(pathname: string): number | undefined {
  const match = pathname.match(/^\/workspace\/work-items\/(\d+)$/)
  return match ? positiveId(match[1]) : undefined
}

export function parseWorkItemsRoute(
  pathname: string,
  search: Readonly<Record<string, unknown>>,
): WorkItemsRouteState {
  const projectSet = idList(search.project_set_ids)
  const project = idList(search.project_ids)
  const requirement = idList(search.requirement_ids)
  const assignee = idList(search.assignee_ids)
  const clients = idList(search.task_client_ids)
  const pmAssignee = idList(search.pm_assignee_ids)
  const one2allAssignee = idList(search.one2all_assignee_ids)
  const legacyAssignees = pmAssignee.values.length ? pmAssignee : one2allAssignee
  const resolvedAssignees = search.assignee_ids !== undefined ? assignee : legacyAssignees
  const singularPresent = [
    'project_set_id', 'project_id', 'project_new_ids', 'requirement_id',
    'team_id', 'team_ids', 'classic_project_id', 'classic_project_ids',
  ].some((key) => search[key] !== undefined)
  const hasInvalidIDs = !projectSet.valid || !project.valid || !requirement.valid
    || !assignee.valid || !clients.valid || !pmAssignee.valid || !one2allAssignee.valid
  const conflictingLegacyAssignees = pmAssignee.values.length > 0
    && one2allAssignee.values.length > 0
    && (pmAssignee.values.length !== one2allAssignee.values.length
      || pmAssignee.values.some((id, index) => id !== one2allAssignee.values[index]))
  const invalidScope = hasInvalidIDs || singularPresent || conflictingLegacyAssignees
  const scope: WorkItemsScope = {
    projectSetIds: projectSet.values,
    projectIds: project.values,
    requirementIds: requirement.values,
  }
  const view = search.view === 'board' ? 'board' : 'list'
  const groupByValues: WorkItemsGroupBy[] = ['requirement_id', 'assignee_id', 'priority']
  const parsedGroup = stringValue(search.group_by) as WorkItemsGroupBy
  const rawStatuses = stringList(search.statuses)
  const rawPriorities = stringList(search.priorities)
  const statuses = canonicalEnumList(
    search.statuses,
    normalizeWorkItemStatus,
    WORK_ITEMS_COPY.enums.statuses,
  )
  const priorities = canonicalEnumList(
    search.priorities,
    normalizeWorkItemPriority,
    WORK_ITEMS_COPY.enums.priorities,
  )
  const enumRouteChanged = rawStatuses.join(',') !== statuses.join(',')
    || rawPriorities.join(',') !== priorities.join(',')
  const legacyDetail = Boolean(detailIdFromPath(pathname))
  return {
    scope,
    view,
    layout: view === 'list' && search.layout === 'gantt' ? 'gantt' : 'table',
    search: stringValue(search.q).slice(0, 120),
    overdue: trueFlag(search.overdue),
    assigneeIds: resolvedAssignees.values,
    statuses,
    priorities,
    taskClientIds: clients.values,
    filterKinds: stringList(search.filter_kinds),
    groupBy: groupByValues.includes(parsedGroup) ? parsedGroup : 'none',
    sort: ['created_at_desc', 'created_at_asc', 'updated_at_desc', 'updated_at_asc']
      .includes(stringValue(search.sort))
      ? stringValue(search.sort) as WorkItemsRouteState['sort']
      : 'created_at_desc',
    page: positiveId(search.page) || 1,
    pageSize: pageSize(search.page_size),
    detailId: detailIdFromPath(pathname) || positiveId(search.task_id ?? search.detail_id),
    mode: search.mode === 'edit' ? 'edit' : 'view',
    from: search.from === 'inbox' ? 'inbox' : undefined,
    invalidLegacyScope: invalidScope || trueFlag(search.invalid_scope),
    needsRouteNormalization: legacyDetail || enumRouteChanged || [
      'source', 'authority', 'pm_assignee_ids', 'one2all_assignee_ids', 'detail_id',
    ].some((key) => search[key] !== undefined),
  }
}

export function workItemsPath(
  route: WorkItemsRouteState,
  options: { detailId?: number; omitDetail?: boolean } = {},
): string {
  const query = new URLSearchParams()
  if (route.view !== 'list') query.set('view', route.view)
  if (route.view === 'list' && route.layout !== 'table') query.set('layout', route.layout)
  if (route.scope.projectSetIds.length) query.set('project_set_ids', route.scope.projectSetIds.join(','))
  if (route.scope.projectIds.length) query.set('project_ids', route.scope.projectIds.join(','))
  if (route.scope.requirementIds.length) query.set('requirement_ids', route.scope.requirementIds.join(','))
  if (route.search) query.set('q', route.search)
  if (route.overdue) query.set('overdue', '1')
  if (route.assigneeIds.length) query.set('assignee_ids', route.assigneeIds.join(','))
  if (route.statuses.length) query.set('statuses', route.statuses.join(','))
  if (route.priorities.length) query.set('priorities', route.priorities.join(','))
  if (route.taskClientIds.length) query.set('task_client_ids', route.taskClientIds.join(','))
  if (route.filterKinds?.length) query.set('filter_kinds', route.filterKinds.join(','))
  if (route.groupBy !== 'none') query.set('group_by', route.groupBy)
  if (route.sort !== 'created_at_desc') query.set('sort', route.sort)
  if (route.page > 1) query.set('page', String(route.page))
  if (route.pageSize !== DEFAULT_PAGE_SIZE) query.set('page_size', String(route.pageSize))
  if (route.from) query.set('from', route.from)
  const detailId = options.omitDetail ? undefined : options.detailId || route.detailId
  if (detailId) {
    query.set('task_id', String(detailId))
    if (route.mode === 'edit') query.set('mode', 'edit')
  }
  const pathname = '/workspace/work-items'
  const suffix = query.toString()
  return `${pathname}${suffix ? `?${suffix}` : ''}`
}

export function requestScope(route: WorkItemsRouteState): Readonly<Record<string, unknown>> {
  return {
    ...(route.scope.projectSetIds.length
      ? { project_set_ids: route.scope.projectSetIds.join(',') } : {}),
    ...(route.scope.projectIds.length
      ? { project_ids: route.scope.projectIds.join(',') } : {}),
    ...(route.scope.requirementIds.length
      ? { requirement_ids: route.scope.requirementIds.join(',') } : {}),
    ...(route.search ? { q: route.search } : {}),
    ...(route.overdue ? { overdue: 1 } : {}),
    ...(route.assigneeIds.length ? { assignee_ids: route.assigneeIds.join(',') } : {}),
    ...(route.statuses.length ? { statuses: route.statuses.join(',') } : {}),
    ...(route.priorities.length ? { priorities: route.priorities.join(',') } : {}),
    ...(route.taskClientIds.length ? { task_client_ids: route.taskClientIds.join(',') } : {}),
    ...(route.view === 'list' && route.groupBy !== 'none' ? { group_by: route.groupBy } : {}),
    sort: route.sort,
  }
}

export function resetPage(route: WorkItemsRouteState): WorkItemsRouteState {
  return { ...route, page: 1 }
}

export function emptyScope(): WorkItemsScope {
  return { projectSetIds: [], projectIds: [], requirementIds: [] }
}

export function scopeSummary(scope: WorkItemsScope): string {
  const count = scope.requirementIds.length || scope.projectIds.length || scope.projectSetIds.length
  return count ? WORK_ITEMS_COPY.scope.summarySelected(count) : WORK_ITEMS_COPY.scope.summaryAll
}
