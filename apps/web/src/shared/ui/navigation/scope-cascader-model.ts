import type { PMScopeOption, PMScopeOptions, PMScopeSelection } from '@/shared/api'

export type PMScopeLevel = 'projectSet' | 'project' | 'requirement'
export type PMScopeCheckboxState = false | true | 'mixed'
export type PMScopeSelectionMode = 'multiple' | 'single-requirement'

export type PMScopeCascaderModel = {
  options: PMScopeOptions
  allOptions: PMScopeOptions
  selection: PMScopeSelection
  invalid: boolean
}

const EMPTY_SELECTION: PMScopeSelection = {
  projectSetIds: [],
  projectIds: [],
  requirementIds: [],
}

function uniqueOptions(options: readonly PMScopeOption[]): PMScopeOption[] {
  const counts = new Map<number, number>()
  options.forEach(({ id }) => {
    if (!Number.isSafeInteger(id) || id <= 0) return
    counts.set(id, (counts.get(id) ?? 0) + 1)
  })
  return options.filter(({ id }) => (
    Number.isSafeInteger(id) && id > 0 && counts.get(id) === 1
  ))
}

function uniqueChildren(
  options: readonly PMScopeOption[],
  parents: readonly PMScopeOption[],
): PMScopeOption[] {
  const parentIds = new Set(parents.map(({ id }) => id))
  return uniqueOptions(options).filter(({ parentId }) => (
    Number.isSafeInteger(parentId)
    && Boolean(parentId && parentIds.has(parentId))
  ))
}

function canonicalIds(ids: readonly number[]): number[] | null {
  if (
    ids.some((id) => !Number.isSafeInteger(id) || id <= 0)
    || new Set(ids).size !== ids.length
  ) return null
  return [...ids]
}

function hasEvery(ids: readonly number[], options: readonly PMScopeOption[]): boolean {
  const optionIds = new Set(options.map(({ id }) => id))
  return ids.every((id) => optionIds.has(id))
}

function hasSelectedParents(
  ids: readonly number[],
  options: readonly PMScopeOption[],
  parentIds: readonly number[],
): boolean {
  const byId = new Map(options.map((option) => [option.id, option]))
  const selectedParents = new Set(parentIds)
  return ids.every((id) => {
    const parentId = byId.get(id)?.parentId
    return Boolean(parentId && selectedParents.has(parentId))
  })
}

export function resolvePMScopeCascaderModel(
  options: PMScopeOptions,
  value: PMScopeSelection,
): PMScopeCascaderModel {
  const projectSets = uniqueOptions(options.projectSets)
  const allProjects = uniqueChildren(options.projects, projectSets)
  const allRequirements = uniqueChildren(options.requirements, allProjects)
  const projectSetIds = canonicalIds(value.projectSetIds)
  const projectIds = canonicalIds(value.projectIds)
  const requirementIds = canonicalIds(value.requirementIds)
  const valid = Boolean(
    projectSetIds && projectIds && requirementIds
    && hasEvery(projectSetIds, projectSets)
    && hasEvery(projectIds, allProjects)
    && hasSelectedParents(projectIds, allProjects, projectSetIds)
    && hasEvery(requirementIds, allRequirements)
    && hasSelectedParents(requirementIds, allRequirements, projectIds),
  )
  const selection = valid
    ? { projectSetIds: projectSetIds!, projectIds: projectIds!, requirementIds: requirementIds! }
    : EMPTY_SELECTION
  const selectedProjectSets = new Set(selection.projectSetIds)
  const selectedProjects = new Set(selection.projectIds)

  return {
    invalid: !valid,
    selection,
    allOptions: {
      projectSets,
      projects: allProjects,
      requirements: allRequirements,
    },
    options: {
      projectSets,
      projects: allProjects.filter(({ parentId }) => Boolean(
        parentId && selectedProjectSets.has(parentId),
      )),
      requirements: allRequirements.filter(({ parentId }) => Boolean(
        parentId && selectedProjects.has(parentId),
      )),
    },
  }
}

function merged(ids: readonly number[], additions: readonly number[]): number[] {
  return [...new Set([...ids, ...additions])]
}

function without(ids: readonly number[], removals: ReadonlySet<number>): number[] {
  return ids.filter((id) => !removals.has(id))
}

function children(options: readonly PMScopeOption[], parentId: number): PMScopeOption[] {
  return options.filter((option) => option.parentId === parentId)
}

export function scopeCheckboxState(
  model: PMScopeCascaderModel,
  level: PMScopeLevel,
  id: number,
  leaf: PMScopeLevel = 'requirement',
): PMScopeCheckboxState {
  const { selection, allOptions } = model
  if (level === 'requirement') return selection.requirementIds.includes(id)
  if (level === 'project') {
    if (leaf === 'project') return selection.projectIds.includes(id)
    if (!selection.projectIds.includes(id)) return false
    const requirements = children(allOptions.requirements, id)
    return requirements.every((option) => selection.requirementIds.includes(option.id))
      ? true : 'mixed'
  }
  if (!selection.projectSetIds.includes(id)) return false
  const projects = children(allOptions.projects, id)
  return projects.every((project) => scopeCheckboxState(model, 'project', project.id, leaf) === true)
    ? true : 'mixed'
}

export function togglePMScopeSelection(
  model: PMScopeCascaderModel,
  level: PMScopeLevel,
  id: number,
  leaf: PMScopeLevel = 'requirement',
): PMScopeSelection {
  const { selection } = model
  if (!model.options.projectSets.some((option) => option.id === id) && level === 'projectSet') {
    return selection
  }
  if (!model.allOptions.projects.some((option) => option.id === id) && level === 'project') {
    return selection
  }
  if (!model.options.requirements.some((option) => option.id === id) && level === 'requirement') {
    return selection
  }
  if (level === 'projectSet') {
    const projects = children(model.allOptions.projects, id)
    const projectIds = projects.map((option) => option.id)
    const projectIdSet = new Set(projectIds)
    const requirementIds = leaf === 'project'
      ? []
      : model.allOptions.requirements
        .filter((option) => Boolean(option.parentId && projectIdSet.has(option.parentId)))
        .map((option) => option.id)
    if (scopeCheckboxState(model, level, id, leaf) === true) return {
      projectSetIds: selection.projectSetIds.filter((value) => value !== id),
      projectIds: without(selection.projectIds, projectIdSet),
      requirementIds: without(selection.requirementIds, new Set(requirementIds)),
    }
    return {
      projectSetIds: merged(selection.projectSetIds, [id]),
      projectIds: merged(selection.projectIds, projectIds),
      requirementIds: merged(selection.requirementIds, requirementIds),
    }
  }
  if (level === 'project') {
    const project = model.allOptions.projects.find((option) => option.id === id)
    if (!project?.parentId) return selection
    const requirementIds = leaf === 'project'
      ? []
      : children(model.allOptions.requirements, id).map((option) => option.id)
    if (scopeCheckboxState(model, level, id, leaf) === true) return {
      projectSetIds: selection.projectSetIds,
      projectIds: selection.projectIds.filter((value) => value !== id),
      requirementIds: without(selection.requirementIds, new Set(requirementIds)),
    }
    return {
      projectSetIds: merged(selection.projectSetIds, [project.parentId]),
      projectIds: merged(selection.projectIds, [id]),
      requirementIds: merged(selection.requirementIds, requirementIds),
    }
  }
  return {
    projectSetIds: selection.projectSetIds,
    projectIds: selection.projectIds,
    requirementIds: selection.requirementIds.includes(id)
      ? selection.requirementIds.filter((value) => value !== id)
      : [...selection.requirementIds, id],
  }
}

/**
 * Cascades may navigate through parent rows without selecting them. Selecting
 * a visible requirement must therefore establish its legal parent path instead
 * of coupling navigation state to the controlled selection.
 */
export function togglePMScopeRequirementPath(
  model: PMScopeCascaderModel,
  id: number,
): PMScopeSelection {
  const requirement = model.allOptions.requirements.find((option) => option.id === id)
  if (!requirement?.parentId) return model.selection
  const project = model.allOptions.projects.find((option) => option.id === requirement.parentId)
  if (!project?.parentId) return model.selection
  if (!model.selection.requirementIds.includes(id)) return {
    projectSetIds: merged(model.selection.projectSetIds, [project.parentId]),
    projectIds: merged(model.selection.projectIds, [project.id]),
    requirementIds: merged(model.selection.requirementIds, [id]),
  }
  const remainingRequirements = model.selection.requirementIds.filter((value) => value !== id)
  const remainingForProject = remainingRequirements.some((value) => (
    model.allOptions.requirements.find((option) => option.id === value)?.parentId === project.id
  ))
  if (remainingForProject || scopeCheckboxState(model, 'project', project.id) === true) {
    return { ...model.selection, requirementIds: remainingRequirements }
  }
  const projectIds = model.selection.projectIds.filter((value) => value !== project.id)
  const keepsProjectSet = projectIds.some((value) => (
    model.allOptions.projects.find((option) => option.id === value)?.parentId === project.parentId
  ))
  return {
    projectSetIds: keepsProjectSet
      ? model.selection.projectSetIds
      : model.selection.projectSetIds.filter((value) => value !== project.parentId),
    projectIds,
    requirementIds: remainingRequirements,
  }
}

/** Selects one active hierarchy path without selecting sibling descendants. */
export function selectSinglePMScopePath(
  model: PMScopeCascaderModel,
  level: PMScopeLevel,
  id: number,
): PMScopeSelection {
  if (level === 'projectSet') {
    if (!model.options.projectSets.some((option) => option.id === id)) return model.selection
    return { projectSetIds: [id], projectIds: [], requirementIds: [] }
  }
  if (level === 'project') {
    const project = model.allOptions.projects.find((option) => option.id === id)
    if (!project?.parentId) return model.selection
    return { projectSetIds: [project.parentId], projectIds: [id], requirementIds: [] }
  }
  const requirement = model.allOptions.requirements.find((option) => option.id === id)
  if (!requirement?.parentId) return model.selection
  const project = model.allOptions.projects.find((option) => option.id === requirement.parentId)
  if (!project?.parentId) return model.selection
  return {
    projectSetIds: [project.parentId],
    projectIds: [project.id],
    requirementIds: [requirement.id],
  }
}
