import type { InboxRouteState, InboxScope } from './types'

const EMPTY_SCOPE: InboxScope = Object.freeze({
  projectSetIds: [],
  projectIds: [],
  requirementIds: [],
})

function stringValue(value: unknown): string {
  if (typeof value === 'string') return value.trim()
  return typeof value === 'number' || typeof value === 'boolean' ? String(value) : ''
}

function trueFlag(value: unknown): boolean {
  return value === true || value === 1 || value === '1' || value === 'true'
}

function idList(value: unknown): { ids: number[]; valid: boolean } {
  if (value === undefined || value === null || value === '') {
    return { ids: [], valid: true }
  }
  const sources = Array.isArray(value) ? value : [value]
  let canonical: number[] | undefined
  for (const source of sources) {
    const raw = String(source).trim()
    if (!raw) return { ids: [], valid: false }
    const parts = raw.split(',').map((part) => part.trim())
    if (parts.some((part) => !/^[1-9]\d*$/.test(part))) {
      return { ids: [], valid: false }
    }
    const ids = [...new Set(parts.map(Number))]
    if (ids.some((id) => !Number.isSafeInteger(id) || id <= 0)) {
      return { ids: [], valid: false }
    }
    if (canonical && (
      canonical.length !== ids.length
      || canonical.some((id, index) => id !== ids[index])
    )) {
      return { ids: [], valid: false }
    }
    canonical = ids
  }
  return { ids: canonical || [], valid: true }
}

function pageValue(value: unknown): number {
  const page = Number(value)
  return Number.isSafeInteger(page) && page > 0 ? page : 1
}

export function parseInboxRoute(
  search: Readonly<Record<string, unknown>>,
): InboxRouteState {
  const projectSets = idList(search.project_set_ids)
  const projects = idList(search.project_ids)
  const requirements = idList(search.requirement_ids)
  const invalidLegacyScope = [
    'project_set_id',
    'project_id',
    'project_new_ids',
    'requirement_id',
    'team_id',
  ].some((key) => search[key] !== undefined)
  return {
    projectSetIds: projectSets.ids,
    projectIds: projects.ids,
    requirementIds: requirements.ids,
    search: stringValue(search.q).slice(0, 120),
    workflowPage: pageValue(search.workflow_page),
    taskPage: pageValue(search.task_page),
    bugPage: pageValue(search.bug_page),
    mentionPage: pageValue(search.mention_page),
    invalidLegacyScope,
    invalidScope: invalidLegacyScope
      || trueFlag(search.invalid_scope)
      || !projectSets.valid
      || !projects.valid
      || !requirements.valid,
  }
}

export function requestScope(
  route: InboxRouteState,
): Readonly<Record<string, unknown>> {
  return {
    ...(route.projectSetIds.length
      ? { project_set_ids: route.projectSetIds.join(',') }
      : {}),
    ...(route.projectIds.length ? { project_ids: route.projectIds.join(',') } : {}),
    ...(route.requirementIds.length
      ? { requirement_ids: route.requirementIds.join(',') }
      : {}),
    ...(route.search ? { q: route.search } : {}),
  }
}

export function inboxPath(route: InboxRouteState): string {
  const query = new URLSearchParams()
  if (route.projectSetIds.length) {
    query.set('project_set_ids', route.projectSetIds.join(','))
  }
  if (route.projectIds.length) query.set('project_ids', route.projectIds.join(','))
  if (route.requirementIds.length) {
    query.set('requirement_ids', route.requirementIds.join(','))
  }
  if (route.search) query.set('q', route.search)
  if (route.workflowPage > 1) query.set('workflow_page', String(route.workflowPage))
  if (route.taskPage > 1) query.set('task_page', String(route.taskPage))
  if (route.bugPage > 1) query.set('bug_page', String(route.bugPage))
  if (route.mentionPage > 1) query.set('mention_page', String(route.mentionPage))
  const suffix = query.toString()
  return `/workspace/inbox${suffix ? `?${suffix}` : ''}`
}

export function resetFeedPages(route: InboxRouteState): InboxRouteState {
  return { ...route, workflowPage: 1, taskPage: 1, bugPage: 1, mentionPage: 1 }
}

export function emptyScope(): InboxScope {
  return { ...EMPTY_SCOPE }
}

export function hasInboxScope(scope: InboxScope): boolean {
  return Boolean(
    scope.projectSetIds.length || scope.projectIds.length || scope.requirementIds.length,
  )
}
