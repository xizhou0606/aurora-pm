import { copy } from '@/shared/copy'

export type PMScopeOption = {
  id: number
  label: string
  parentId?: number
  requirementsLockedAt?: string
}

export type PMScopeOptions = {
  projectSets: PMScopeOption[]
  projects: PMScopeOption[]
  requirements: PMScopeOption[]
}

export type PMScopeSelection = {
  projectSetIds: readonly number[]
  projectIds: readonly number[]
  requirementIds: readonly number[]
}

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {}
}

function array(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

function positiveInteger(value: unknown): number | undefined {
  const number = Number(value)
  return Number.isInteger(number) && number > 0 ? number : undefined
}

function text(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function normalizeOption(value: unknown, parentKeys: readonly string[]): PMScopeOption | null {
  const source = record(value)
  const id = positiveInteger(source.id)
  if (!id) return null
  const parentId = parentKeys.map((key) => positiveInteger(source[key])).find(Boolean)
  const label = text(source.name)
    || text(source.code)
    || text(source.title)
    || copy.ui.scope.fallback(id)
  const lockedAt = typeof source.requirements_locked_at === 'string' && source.requirements_locked_at
    ? source.requirements_locked_at
    : undefined
  return { id, label, ...(parentId ? { parentId } : {}), ...(lockedAt ? { requirementsLockedAt: lockedAt } : {}) }
}

function options(value: unknown, parentKeys: readonly string[]): PMScopeOption[] {
  const normalized = array(value).flatMap((entry) => {
    const option = normalizeOption(entry, parentKeys)
    return option ? [option] : []
  })
  const counts = new Map<number, number>()
  normalized.forEach((option) => counts.set(option.id, (counts.get(option.id) ?? 0) + 1))
  return normalized.filter((option) => counts.get(option.id) === 1)
}

/**
 * Normalize the frozen PM scope contract. Invalid rows are discarded instead
 * of being widened into another workspace or parent scope.
 */
export function normalizePMScopeOptions(value: unknown): PMScopeOptions {
  const source = record(value)
  return {
    projectSets: options(source.project_sets ?? source.projectSets, []),
    projects: options(source.projects, ['project_new_id']),
    requirements: options(source.requirements, ['version_task_new_id']),
  }
}

function isCanonicalSelection(ids: readonly number[]): boolean {
  return ids.every((id) => Number.isSafeInteger(id) && id > 0)
    && new Set(ids).size === ids.length
}

/**
 * Validate both scope membership and parent ownership before a scoped query is
 * allowed to run. Child-only and cross-parent selections fail closed.
 */
export function isValidPMScopeSelection(
  selection: PMScopeSelection,
  options: PMScopeOptions,
): boolean {
  if (
    !isCanonicalSelection(selection.projectSetIds)
    || !isCanonicalSelection(selection.projectIds)
    || !isCanonicalSelection(selection.requirementIds)
  ) return false

  const projectSets = new Set(options.projectSets.map((item) => item.id))
  const selectedProjectSets = new Set(selection.projectSetIds)
  const projects = new Map(options.projects.map((item) => [item.id, item]))
  const selectedProjects = new Set(selection.projectIds)
  const requirements = new Map(options.requirements.map((item) => [item.id, item]))

  if (selection.projectSetIds.some((id) => !projectSets.has(id))) return false
  if (selection.projectIds.some((id) => {
    const project = projects.get(id)
    return !project?.parentId || !selectedProjectSets.has(project.parentId)
  })) return false
  if (selection.requirementIds.some((id) => {
    const requirement = requirements.get(id)
    return !requirement?.parentId || !selectedProjects.has(requirement.parentId)
  })) return false
  return true
}
