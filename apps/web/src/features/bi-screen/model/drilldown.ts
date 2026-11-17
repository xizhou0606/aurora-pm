import type { Drilldown, RouteState } from '../types'
import { normalizedScope } from './route'

export function drilldownPath(route: RouteState, target: Drilldown): string {
  const scope = normalizedScope({ ...route.scope, ...target.scope })
  const query = new URLSearchParams(target.query)
  if (scope.projectSetIds.length) query.set('project_set_ids', scope.projectSetIds.join(','))
  if (scope.projectIds.length) query.set('project_ids', scope.projectIds.join(','))
  if (scope.requirementIds.length) query.set('requirement_ids', scope.requirementIds.join(','))
  const pathname = target.domain === 'tasks'
    ? '/workspace/work-items'
    : '/workspace/quality-management'
  if (target.domain === 'tasks') query.set('view', 'list')
  const suffix = query.toString()
  return `${pathname}${suffix ? `?${suffix}` : ''}`
}
