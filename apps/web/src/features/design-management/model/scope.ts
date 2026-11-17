import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'

import type { RequirementOption } from './types'

export function requirementScopeOptions(items: readonly RequirementOption[]): PMScopeOptions {
  return {
    projectSets: [...new Map(items.map((item) => [item.project_set_id, {
      id: item.project_set_id,
      label: item.project_set_name,
    }])).values()],
    projects: [...new Map(items.map((item) => [item.plan_unit_id, {
      id: item.plan_unit_id,
      label: item.plan_unit_title,
      parentId: item.project_set_id,
    }])).values()],
    requirements: items.map((item) => ({
      id: item.id,
      label: item.title,
      parentId: item.plan_unit_id,
    })),
  }
}

export function requirementScopeSelection(
  requirementId: number | undefined,
  items: readonly RequirementOption[],
): PMScopeSelection {
  const item = items.find((candidate) => candidate.id === requirementId)
  return item ? {
    projectSetIds: [item.project_set_id],
    projectIds: [item.plan_unit_id],
    requirementIds: [item.id],
  } : { projectSetIds: [], projectIds: [], requirementIds: [] }
}
