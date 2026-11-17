import type {
  QualityDefectDomainFilter,
  QualityFilters,
  QualityRelationFilter,
  QualityRouteState,
  QualityScope,
} from './types'
import { optionalNumber, optionalString } from './values'

const PAGE_SIZES = [10, 20, 50, 100] as const

function first(value: unknown): unknown {
  return Array.isArray(value) ? value[0] : value
}

function searchString(value: unknown): string | undefined {
  const candidate = first(value)
  if (typeof candidate === 'string') return candidate.trim() || undefined
  return typeof candidate === 'number' || typeof candidate === 'boolean'
    ? String(candidate)
    : undefined
}

function searchStrings(value: unknown): string[] {
  const candidate = first(value)
  if (candidate === undefined || candidate === null) return []
  const values = String(candidate)
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  return [...new Set(values)]
}

function trueFlag(value: unknown): boolean {
  const candidate = first(value)
  return candidate === true || candidate === 1
    || candidate === '1' || candidate === 'true'
}

function pageSize(value: unknown): QualityRouteState['pageSize'] {
  const parsed = optionalNumber(first(value))
  return PAGE_SIZES.includes(parsed as QualityRouteState['pageSize'])
    ? parsed as QualityRouteState['pageSize']
    : 50
}

function strictIds(value: unknown): { values: number[]; valid: boolean } {
  if (value === undefined) return { values: [], valid: true }
  if (Array.isArray(value) && value.length !== 1) return { values: [], valid: false }
  const parts = String(Array.isArray(value) ? value[0] : value).split(',').map((item) => item.trim())
  if (
    !parts.length
    || parts.some((item) => !/^[1-9]\d*$/.test(item))
    || new Set(parts).size !== parts.length
  ) return { values: [], valid: false }
  const values = parts.map(Number)
  return values.every((id) => Number.isSafeInteger(id) && id > 0)
    ? { values, valid: true }
    : { values: [], valid: false }
}

function scope(search: Readonly<Record<string, unknown>>): {
  value: QualityScope
  valid: boolean
} {
  const projectSets = strictIds(search.project_set_ids)
  const projects = strictIds(search.project_ids)
  const requirements = strictIds(search.requirement_ids)
  return {
    value: {
      projectSetIds: projectSets.values,
      projectIds: projects.values,
      requirementIds: requirements.values,
    },
    valid: projectSets.valid && projects.valid && requirements.valid,
  }
}

function filters(search: Readonly<Record<string, unknown>>): QualityFilters {
  const sortBy = searchString(search.sort_by)
  const sortOrder = searchString(search.sort_order)
  const requestedRelation = searchString(search.relation)
  const requestedDefectDomain = searchString(search.defect_domain)
  const relation: QualityRelationFilter = ['reported', 'assigned', 'responsible', 'resolved'].includes(
    requestedRelation || '',
  )
    ? requestedRelation as QualityRelationFilter
    : trueFlag(search.responsible_by_me)
      ? 'responsible'
      : trueFlag(search.resolved_by_me)
      ? 'resolved'
      : trueFlag(search.assigned_to_me)
        ? 'assigned'
        : trueFlag(search.reported_by_me)
          ? 'reported'
          : ''
  return {
    q: searchString(search.q) || '',
    status: searchString(search.status) || '',
    severities: searchStrings(search.severity_key),
    environments: searchStrings(search.found_environment),
    assigneeUserIds: strictIds(search.assignee_user_id).values,
    ownerUserIds: strictIds(search.current_owner_user_id).values,
    reporterUserIds: strictIds(search.reporter_user_id).values,
    relation,
    defectDomain: ['functional', 'ui'].includes(requestedDefectDomain || '')
      ? requestedDefectDomain as QualityDefectDomainFilter
      : '',
    createdFrom: searchString(search.created_from) || '',
    createdTo: searchString(search.created_to) || '',
    sortBy: ['priority', 'severity', 'status', 'created_at', 'status_changed_at'].includes(sortBy || '')
      ? sortBy as QualityFilters['sortBy']
      : 'created_at',
    sortOrder: sortOrder === 'asc' ? 'asc' : 'desc',
  }
}

export function parseQualityRoute(
  pathname: string,
  search: Readonly<Record<string, unknown>>,
): QualityRouteState {
  const detailMatch = pathname.match(/^\/workspace\/quality-management\/bugs\/(\d+)$/)
  const legacyCreate = pathname === '/workspace/quality-management/bugs/new'
  const queryBugId = optionalNumber(first(search.bug_id))
  const bugId = detailMatch ? optionalNumber(detailMatch[1]) : queryBugId
  const isCreate = legacyCreate || trueFlag(search.create)
  const parsedScope = scope(search)
  const legacyScope = [
    'project_set_id', 'project_id', 'project_new_ids', 'requirement_id', 'team_id',
  ].some((key) => search[key] !== undefined)
  return {
    kind: isCreate ? 'create' : bugId ? 'detail' : 'list',
    view: searchString(search.view) === 'dashboard' ? 'dashboard' : 'list',
    bugId,
    mode: searchString(search.mode) === 'edit' || trueFlag(search.edit)
      ? 'edit'
      : 'view',
    from: searchString(search.from),
    scope: parsedScope.value,
    filters: filters(search),
    page: optionalNumber(first(search.page)) || 1,
    pageSize: pageSize(search.page_size),
    sourceExecutionId: optionalNumber(first(search.source_execution_id)),
    sourceTestId: optionalNumber(first(search.source_test_id)),
    actualResult: searchString(search.actual_result),
    invalidScope: trueFlag(search.invalid_scope) || legacyScope || !parsedScope.valid,
    needsRouteNormalization: legacyCreate || Boolean(detailMatch),
  }
}

function appendIds(params: URLSearchParams, key: string, ids: readonly number[]) {
  if (ids.length) params.set(key, ids.join(','))
}

function appendScope(params: URLSearchParams, value: QualityScope) {
  appendIds(params, 'project_set_ids', value.projectSetIds)
  appendIds(params, 'project_ids', value.projectIds)
  appendIds(params, 'requirement_ids', value.requirementIds)
}

function appendListState(params: URLSearchParams, route: QualityRouteState) {
  appendScope(params, route.scope)
  if (route.view === 'dashboard') params.set('view', 'dashboard')
  if (route.filters.q) params.set('q', route.filters.q)
  if (route.filters.status) params.set('status', route.filters.status)
  if (route.filters.severities.length) params.set('severity_key', route.filters.severities.join(','))
  if (route.filters.environments.length) params.set('found_environment', route.filters.environments.join(','))
  if (route.filters.assigneeUserIds.length) params.set('assignee_user_id', route.filters.assigneeUserIds.join(','))
  if (route.filters.ownerUserIds.length) params.set('current_owner_user_id', route.filters.ownerUserIds.join(','))
  if (route.filters.reporterUserIds.length) params.set('reporter_user_id', route.filters.reporterUserIds.join(','))
  if (route.filters.relation) params.set('relation', route.filters.relation)
  if (route.filters.defectDomain) params.set('defect_domain', route.filters.defectDomain)
  if (route.filters.createdFrom) params.set('created_from', route.filters.createdFrom)
  if (route.filters.createdTo) params.set('created_to', route.filters.createdTo)
  if (route.filters.sortBy !== 'created_at') params.set('sort_by', route.filters.sortBy)
  if (route.filters.sortOrder !== 'desc') params.set('sort_order', route.filters.sortOrder)
  if (route.page > 1) params.set('page', String(route.page))
  if (route.pageSize !== 50) params.set('page_size', String(route.pageSize))
}

export function qualityListPath(route: QualityRouteState): string {
  const params = new URLSearchParams()
  appendListState(params, route)
  const query = params.toString()
  return `/workspace/quality-management${query ? `?${query}` : ''}`
}

export function qualityCreatePath(route: QualityRouteState): string {
  const params = new URLSearchParams()
  appendListState(params, route)
  params.set('create', '1')
  if (route.sourceExecutionId) params.set('source_execution_id', String(route.sourceExecutionId))
  if (route.sourceTestId) params.set('source_test_id', String(route.sourceTestId))
  if (route.actualResult) params.set('actual_result', route.actualResult)
  const query = params.toString()
  return `/workspace/quality-management${query ? `?${query}` : ''}`
}

export function qualityDetailPath(
  route: QualityRouteState,
  id: number,
  mode: 'view' | 'edit' = 'view',
): string {
  const params = new URLSearchParams()
  appendListState(params, route)
  params.set('bug_id', String(id))
  if (mode === 'edit') params.set('mode', 'edit')
  if (route.from) params.set('from', route.from)
  const query = params.toString()
  return `/workspace/quality-management${query ? `?${query}` : ''}`
}
