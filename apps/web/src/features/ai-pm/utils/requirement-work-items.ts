import { navigateToWorkItems } from '@/shared/runtime'

export type RequirementWorkItemsScope = {
  id: number
  project_new_id: number
  version_task_new_id?: number | null
}

export function requirementWorkItemsQuery(
  requirement: RequirementWorkItemsScope,
): Record<string, string> {
  const planUnitId = Number(requirement.version_task_new_id)
  return {
    view: 'list',
    project_set_ids: String(requirement.project_new_id),
    ...(Number.isSafeInteger(planUnitId) && planUnitId > 0
      ? { project_ids: String(planUnitId) }
      : {}),
    requirement_ids: String(requirement.id),
  }
}

export function requirementWorkItemsPath(requirement: RequirementWorkItemsScope): string {
  const query = new URLSearchParams(requirementWorkItemsQuery(requirement)).toString()
  return `/workspace/work-items?${query}`
}

export function openRequirementWorkItems(
  requirement: RequirementWorkItemsScope,
  fallbackNavigate: (path: string) => void,
) {
  const query = requirementWorkItemsQuery(requirement)
  if (navigateToWorkItems(query)) return
  fallbackNavigate(requirementWorkItemsPath(requirement))
}
