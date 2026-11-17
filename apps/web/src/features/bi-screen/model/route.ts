import type {
  RouteState,
  Scope,
  ScreenTab,
  TeamDimension,
} from '../types'

const EMPTY_SCOPE: Scope = Object.freeze({
  projectSetIds: [],
  projectIds: [],
  requirementIds: [],
})

function trueFlag(value: unknown): boolean {
  return value === true || value === 1 || value === '1' || value === 'true'
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

function teamDimension(value: unknown): TeamDimension {
  return value === 'month' || value === 'week' ? value : 'version'
}

function validISOWeekKey(value: string): boolean {
  const match = value.match(/^(\d{4})-W(\d{2})$/)
  if (!match) return false
  const year = Number(match[1])
  const week = Number(match[2])
  if (year < 1 || week < 1 || week > 53) return false
  const december28 = new Date(Date.UTC(year, 11, 28))
  const weekday = december28.getUTCDay() || 7
  december28.setUTCDate(december28.getUTCDate() + 4 - weekday)
  const yearStart = new Date(Date.UTC(december28.getUTCFullYear(), 0, 1))
  const finalISOWeek = Math.ceil(
    ((december28.getTime() - yearStart.getTime()) / 86_400_000 + 1) / 7,
  )
  return week <= finalISOWeek
}

function teamDimensionKey(value: unknown, dimension: TeamDimension): string | undefined {
  if (typeof value !== 'string') return undefined
  const normalized = value.trim()
  if (dimension === 'version') return /^[1-9]\d*$/.test(normalized) ? normalized : undefined
  if (dimension === 'week') return validISOWeekKey(normalized) ? normalized : undefined
  return /^\d{4}-(0[1-9]|1[0-2])$/.test(normalized) ? normalized : undefined
}

export function parseRoute(search: Readonly<Record<string, unknown>>): RouteState {
  const projectSets = idList(search.project_set_ids)
  const projects = idList(search.project_ids)
  const requirements = idList(search.requirement_ids)
  const legacyScope = [
    'project_set_id', 'project_id', 'project_new_ids', 'requirement_id', 'team_id',
  ].some((key) => search[key] !== undefined)
  const dimension = teamDimension(search.team_dimension)
  return {
    scope: {
      projectSetIds: projectSets.values,
      projectIds: projects.values,
      requirementIds: requirements.values,
    },
    period: search.period === 'month' ? 'month' : 'week',
    tab: search.bi_view === 'team' ? 'team' : 'overview',
    teamDimension: dimension,
    teamDimensionKey: teamDimensionKey(search.team_dimension_key, dimension),
		// `team_view` remains accepted in historical links, but the current screen
		// always renders the combined table and chart layout.
		teamView: 'data',
    invalidScope: trueFlag(search.invalid_scope)
      || legacyScope
      || !projectSets.valid
      || !projects.valid
      || !requirements.valid,
  }
}

export function routePath(route: RouteState): string {
  const query = new URLSearchParams()
  if (route.scope.projectSetIds.length) {
    query.set('project_set_ids', route.scope.projectSetIds.join(','))
  }
  if (route.scope.projectIds.length) query.set('project_ids', route.scope.projectIds.join(','))
  if (route.scope.requirementIds.length) {
    query.set('requirement_ids', route.scope.requirementIds.join(','))
  }
  if (route.period === 'month') query.set('period', route.period)
  if (route.tab === 'team') query.set('bi_view', route.tab)
  if (route.teamDimension !== 'version') query.set('team_dimension', route.teamDimension)
  if (route.teamDimensionKey) query.set('team_dimension_key', route.teamDimensionKey)
  const suffix = query.toString()
  return `/workspace/bi-screen${suffix ? `?${suffix}` : ''}`
}

export function sameRouteTarget(left: RouteState, right: RouteState): boolean {
  return routePath(left) === routePath(right)
}

export function routeWithTab(route: RouteState, tab: ScreenTab): RouteState {
  return { ...route, tab }
}

export function routeWithTeamDimension(
  route: RouteState,
  dimension: TeamDimension,
): RouteState {
  return { ...route, teamDimension: dimension, teamDimensionKey: undefined }
}

export function routeWithTeamDimensionKey(route: RouteState, key?: string): RouteState {
  return { ...route, teamDimensionKey: key || undefined }
}

export function normalizedScope(scope: Partial<Scope> | undefined): Scope {
  const ids = (values: readonly number[] | undefined) => (
    [...new Set(values ?? [])].filter((id) => Number.isSafeInteger(id) && id > 0)
  )
  return {
    projectSetIds: ids(scope?.projectSetIds),
    projectIds: ids(scope?.projectIds),
    requirementIds: ids(scope?.requirementIds),
  }
}

export function hasScope(scope: Scope): boolean {
  return Boolean(scope.projectSetIds.length || scope.projectIds.length || scope.requirementIds.length)
}

export function resetScope(): Scope {
  return EMPTY_SCOPE
}
