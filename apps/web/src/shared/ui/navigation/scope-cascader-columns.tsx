import type { PMScopeOptions } from '@/shared/api'
import { copy } from '@/shared/copy'

import {
  resolvePMScopeCascaderModel,
  scopeCheckboxState,
  type PMScopeLevel,
  type PMScopeSelectionMode,
} from './scope-cascader-model'
import {
  type PMScopeCascaderColumn,
} from './scope-cascader-tree'

export type PMScopeCascaderLevels = 'all' | 'project-requirement' | 'project-set-project'

export type PMScopeCascaderLabels = {
  label: string
  description?: string
  hierarchyHint?: string
  projectSet: string
  project: string
  requirement: string
  selectProjectSetFirst: string
  selectProjectFirst: string
  noOptions?: string
}

export type PMScopeCascaderState =
  | { status: 'loading' }
  | { status: 'error'; onRetry: () => void }
  | { status: 'ready'; options: PMScopeOptions }

export function columns(
  labels: PMScopeCascaderLabels,
  model: ReturnType<typeof resolvePMScopeCascaderModel>,
  selectionMode: PMScopeSelectionMode,
  levels: PMScopeCascaderLevels,
  navigation: { projectSetId?: number; projectId?: number },
): PMScopeCascaderColumn[] {
  const emptyLabel = labels.noOptions ?? copy.ui.scope.noOptions
  const leaf: PMScopeLevel = levels === 'project-set-project' ? 'project' : 'requirement'
  const states = (level: PMScopeLevel, ids: readonly number[]) => ({
    selectedIds: ids.filter((id) => selectionMode === 'single-requirement'
      ? model.selection[`${level}Ids`].includes(id)
      : scopeCheckboxState(model, level, id, leaf) === true),
    indeterminateIds: selectionMode === 'single-requirement'
      ? []
      : ids.filter((id) => scopeCheckboxState(model, level, id, leaf) === 'mixed'),
  })
  const navigatedProjectSetId = model.invalid ? undefined : navigation.projectSetId
  const navigatedProjectId = model.invalid ? undefined : navigation.projectId
  const projectSetId = model.allOptions.projectSets.some(({ id }) => id === navigatedProjectSetId)
    ? navigatedProjectSetId
    : model.selection.projectSetIds[0]
  const projects = levels === 'project-requirement'
    ? model.allOptions.projects
    : projectSetId
      ? model.allOptions.projects.filter(({ parentId }) => parentId === projectSetId)
      : []
  const projectId = projects.some(({ id }) => id === navigatedProjectId)
    ? navigatedProjectId
    : projects.find(({ id }) => model.selection.projectIds.includes(id))?.id
  const requirements = projectId
    ? model.allOptions.requirements.filter(({ parentId }) => parentId === projectId)
    : []
  const projectSetIds = model.allOptions.projectSets.map((option) => option.id)
  const projectIds = projects.map((option) => option.id)
  const requirementIds = requirements.map((option) => option.id)
  const result: PMScopeCascaderColumn[] = [
    {
      level: 'projectSet',
      label: labels.projectSet,
      options: model.allOptions.projectSets,
      ...states('projectSet', projectSetIds),
      emptyLabel,
      hasChildren: true,
      activeId: projectSetId,
    },
    {
      level: 'project',
      label: labels.project,
      options: projects,
      ...states('project', projectIds),
      unavailable: projectSetId ? undefined : labels.selectProjectSetFirst,
      emptyLabel,
      hasChildren: true,
      activeId: projectId,
    },
    {
      level: 'requirement',
      label: labels.requirement,
      options: requirements,
      ...states('requirement', requirementIds),
      unavailable: projectId ? undefined : labels.selectProjectFirst,
      emptyLabel,
      hasChildren: false,
    },
  ]
  const visibleColumns = levels === 'project-requirement'
    ? result.slice(1)
    : levels === 'project-set-project'
      ? result.slice(0, 2)
      : result
  return visibleColumns.map((column, index) => ({
    ...column,
    hasChildren: index < visibleColumns.length - 1,
  }))
}