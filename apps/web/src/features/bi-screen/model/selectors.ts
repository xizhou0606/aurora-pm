import type { PMScopeOptions } from '@/shared/api'

import type { DashboardState, Scope } from '../types'

export function selectedScopeCount(scope: Scope): number {
  return scope.requirementIds.length || scope.projectIds.length || scope.projectSetIds.length
}

export function selectedScopeLabel(scope: Scope, options: PMScopeOptions): string | undefined {
  const selected = [
    ...scope.projectSetIds.map((id) => options.projectSets.find((item) => item.id === id)?.label),
    ...scope.projectIds.map((id) => options.projects.find((item) => item.id === id)?.label),
    ...scope.requirementIds.map((id) => options.requirements.find((item) => item.id === id)?.label),
  ].filter((value): value is string => Boolean(value))
  return selected.length === 1 ? selected[0] : undefined
}

export function readyDashboardData(state: DashboardState) {
  return state.status === 'ready' ? state.data : undefined
}
