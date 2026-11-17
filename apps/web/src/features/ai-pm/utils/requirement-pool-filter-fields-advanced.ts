import type { PMScopeSelection } from '@/shared/api'
import type { RequirementPoolFilters } from '../model/types'
import { requirementRoleOwners } from '../utils/requirement-role-owners'
import {
  REQUIREMENT_POOL_UNSCHEDULED,
  type RequirementPoolFilterFieldId,
  type RequirementPoolFilterValue,
  requirementPoolFilterFields,
} from './requirement-pool-filter-fields-core'

function toggleNumber(ids: readonly number[], id: number) {
  return ids.includes(id) ? ids.filter((value) => value !== id) : [...ids, id]
}

function toggleString(ids: readonly string[], id: string) {
  return ids.includes(id) ? ids.filter((value) => value !== id) : [...ids, id]
}

export function requirementPoolFilterFieldValues(
  filters: RequirementPoolFilterValue,
  fieldId: RequirementPoolFilterFieldId,
): readonly string[] | readonly number[] {
  switch (fieldId) {
    case 'scope':
      return [...filters.projectIds.map(String), ...filters.planUnitIds]
    case 'workflow':
      return filters.currentNodeIds
    case 'role-product':
      return filters.productOwnerUserIds
    case 'role-ui':
      return filters.designOwnerUserIds
    case 'role-frontend':
      return filters.frontendOwnerUserIds
    case 'role-backend':
      return filters.backendOwnerUserIds
    case 'role-tester':
      return filters.testerUserIds
  }
}

export function requirementPoolFilterFieldActive(
  filters: RequirementPoolFilterValue,
  fieldId: RequirementPoolFilterFieldId,
) {
  return requirementPoolFilterFieldValues(filters, fieldId).length > 0
}

export function availableRequirementPoolFilterFields(
  filters: RequirementPoolFilterValue,
  pendingFieldId?: RequirementPoolFilterFieldId,
) {
  return requirementPoolFilterFields.filter((field) => (
    field.id !== pendingFieldId && !requirementPoolFilterFieldActive(filters, field.id)
  ))
}

export function toggleRequirementPoolFilter(
  filters: RequirementPoolFilterValue,
  fieldId: RequirementPoolFilterFieldId,
  value: string,
): RequirementPoolFilterValue {
  const next = { ...filters }
  switch (fieldId) {
    case 'scope':
      return next
    case 'workflow':
      next.currentNodeIds = toggleString(filters.currentNodeIds, value)
      return next
    case 'role-product':
      next.productOwnerUserIds = toggleNumber(filters.productOwnerUserIds, Number(value) || 0)
      return next
    case 'role-ui':
      next.designOwnerUserIds = toggleNumber(filters.designOwnerUserIds, Number(value) || 0)
      return next
    case 'role-frontend':
      next.frontendOwnerUserIds = toggleNumber(filters.frontendOwnerUserIds, Number(value) || 0)
      return next
    case 'role-backend':
      next.backendOwnerUserIds = toggleNumber(filters.backendOwnerUserIds, Number(value) || 0)
      return next
    case 'role-tester':
      next.testerUserIds = toggleNumber(filters.testerUserIds, Number(value) || 0)
      return next
  }
}

export function clearRequirementPoolFilter(
  filters: RequirementPoolFilterValue,
  fieldId: RequirementPoolFilterFieldId,
): RequirementPoolFilterValue {
  switch (fieldId) {
    case 'scope':
      return { ...filters, projectIds: [], planUnitIds: [] }
    case 'workflow':
      return { ...filters, currentNodeIds: [] }
    case 'role-product':
      return { ...filters, productOwnerUserIds: [] }
    case 'role-ui':
      return { ...filters, designOwnerUserIds: [] }
    case 'role-frontend':
      return { ...filters, frontendOwnerUserIds: [] }
    case 'role-backend':
      return { ...filters, backendOwnerUserIds: [] }
    case 'role-tester':
      return { ...filters, testerUserIds: [] }
  }
}

export function requirementPoolFilterHasChips(filters: RequirementPoolFilterValue) {
  return requirementPoolFilterFields.some((field) => requirementPoolFilterFieldActive(filters, field.id))
}

export function requirementPoolFilterHasValue(filters: RequirementPoolFilterValue) {
  return Boolean(filters.q.trim()) || requirementPoolFilterHasChips(filters)
}

export function requirementPoolScopeSelection(
  filters: RequirementPoolFilterValue,
): PMScopeSelection {
  return {
    projectSetIds: filters.projectIds,
    projectIds: filters.planUnitIds
      .filter((id) => id !== REQUIREMENT_POOL_UNSCHEDULED)
      .map(Number)
      .filter((id) => Number.isSafeInteger(id) && id > 0),
    requirementIds: [],
  }
}

export function applyRequirementPoolScopeSelection(
  filters: RequirementPoolFilterValue,
  selection: PMScopeSelection,
): RequirementPoolFilterValue {
  const unscheduled = filters.planUnitIds.includes(REQUIREMENT_POOL_UNSCHEDULED)
  return {
    ...filters,
    projectIds: [...selection.projectSetIds],
    planUnitIds: [
      ...(unscheduled ? [REQUIREMENT_POOL_UNSCHEDULED] : []),
      ...selection.projectIds.map(String),
    ],
  }
}

export function toggleRequirementPoolUnscheduled(
  filters: RequirementPoolFilterValue,
): RequirementPoolFilterValue {
  return {
    ...filters,
    planUnitIds: toggleString(filters.planUnitIds, REQUIREMENT_POOL_UNSCHEDULED),
  }
}

export function requirementPoolScopeQuery(
  filters: RequirementPoolFilterValue,
  planUnits: ReadonlyArray<{ id: number; parentId?: number }>,
) {
  const selectedUnits = new Set(
    filters.planUnitIds
      .filter((id) => id !== REQUIREMENT_POOL_UNSCHEDULED)
      .map(Number)
      .filter((id) => Number.isSafeInteger(id) && id > 0),
  )
  const fullSetIds = filters.projectIds.filter((setId) => {
    const children = planUnits.filter((item) => item.parentId === setId)
    return children.length === 0 || children.every((item) => selectedUnits.has(item.id))
  })
  const partialUnits = [...selectedUnits].filter((id) => {
    const parentId = planUnits.find((item) => item.id === id)?.parentId
    return !parentId || !fullSetIds.includes(parentId)
  })
  const planUnit = [
    ...partialUnits.map(String),
    ...(filters.planUnitIds.includes(REQUIREMENT_POOL_UNSCHEDULED) ? [REQUIREMENT_POOL_UNSCHEDULED] : []),
  ]
  return {
    projectId: fullSetIds.length ? fullSetIds.join(',') : undefined,
    planUnit: planUnit.length ? planUnit.join(',') : undefined,
  }
}

export function csvIds(ids: readonly number[] | readonly string[]) {
  return ids.length ? ids.join(',') : undefined
}

// Single source of truth for the pool request filters: the pool page and the
// route loader must derive identical values so React Query reuses one cache
// entry (and one request) for the same URL state.
export function requirementPoolQueryFilters(input: {
  filters: RequirementPoolFilterValue
  projects: ReadonlyArray<{ id: number; parentId?: number }>
  view: 'list' | 'cards'
  page: number
  pageSize: number
}): RequirementPoolFilters {
  const scopeQuery = requirementPoolScopeQuery(input.filters, input.projects)
  return {
    q: input.filters.q.trim() || undefined,
    projectId: scopeQuery.projectId,
    planUnit: scopeQuery.planUnit,
    currentNodeId: csvIds(input.filters.currentNodeIds),
    productOwnerUserId: csvIds(input.filters.productOwnerUserIds),
    designOwnerUserId: csvIds(input.filters.designOwnerUserIds),
    frontendOwnerUserId: csvIds(input.filters.frontendOwnerUserIds),
    backendOwnerUserId: csvIds(input.filters.backendOwnerUserIds),
    testerUserId: csvIds(input.filters.testerUserIds),
    ...(input.view === 'cards'
      ? { limit: 'all' as const, offset: 0 }
      : { limit: input.pageSize, offset: (input.page - 1) * input.pageSize }),
  }
}

function parseCsvNumbers(value?: string) {
  if (!value) return []
  return value.split(',').map(Number).filter((id) => Number.isSafeInteger(id) && id > 0)
}

function parseCsvStrings(value?: string) {
  if (!value) return []
  return value.split(',').map((item) => item.trim()).filter(Boolean)
}

export type RequirementPoolSearchState = {
  q?: string
  project_set_ids?: string
  project_ids?: string
  unscheduled?: '1'
  current_node_id?: string
  product_owner_user_id?: string
  design_owner_user_id?: string
  frontend_owner_user_id?: string
  backend_owner_user_id?: string
  tester_user_id?: string
  view?: 'list' | 'cards'
  page?: number
  page_size?: number
}

export function requirementPoolFiltersFromSearch(
  search: RequirementPoolSearchState,
): RequirementPoolFilterValue {
  return {
    q: search.q ?? '',
    projectIds: parseCsvNumbers(search.project_set_ids),
    planUnitIds: [
      ...(search.unscheduled === '1' ? [REQUIREMENT_POOL_UNSCHEDULED] : []),
      ...parseCsvNumbers(search.project_ids).map(String),
    ],
    currentNodeIds: parseCsvStrings(search.current_node_id),
    productOwnerUserIds: parseCsvNumbers(search.product_owner_user_id),
    designOwnerUserIds: parseCsvNumbers(search.design_owner_user_id),
    frontendOwnerUserIds: parseCsvNumbers(search.frontend_owner_user_id),
    backendOwnerUserIds: parseCsvNumbers(search.backend_owner_user_id),
    testerUserIds: parseCsvNumbers(search.tester_user_id),
  }
}

export function requirementPoolSearchFromFilters(
  filters: RequirementPoolFilterValue,
  extras: { page?: number; pageSize?: number; view?: 'list' | 'cards' } = {},
): RequirementPoolSearchState {
  const planUnits = filters.planUnitIds.filter((id) => id !== REQUIREMENT_POOL_UNSCHEDULED)
  return {
    ...(filters.q.trim() ? { q: filters.q.trim().slice(0, 120) } : {}),
    ...(filters.projectIds.length ? { project_set_ids: filters.projectIds.join(',') } : {}),
    ...(planUnits.length ? { project_ids: planUnits.join(',') } : {}),
    ...(filters.planUnitIds.includes(REQUIREMENT_POOL_UNSCHEDULED) ? { unscheduled: '1' as const } : {}),
    ...(filters.currentNodeIds.length ? { current_node_id: filters.currentNodeIds.join(',') } : {}),
    ...(filters.productOwnerUserIds.length ? { product_owner_user_id: filters.productOwnerUserIds.join(',') } : {}),
    ...(filters.designOwnerUserIds.length ? { design_owner_user_id: filters.designOwnerUserIds.join(',') } : {}),
    ...(filters.frontendOwnerUserIds.length ? { frontend_owner_user_id: filters.frontendOwnerUserIds.join(',') } : {}),
    ...(filters.backendOwnerUserIds.length ? { backend_owner_user_id: filters.backendOwnerUserIds.join(',') } : {}),
    ...(filters.testerUserIds.length ? { tester_user_id: filters.testerUserIds.join(',') } : {}),
    ...(extras.view === 'cards' ? { view: 'cards' as const } : {}),
    ...(extras.page && extras.page > 1 ? { page: extras.page } : {}),
    ...(extras.pageSize && extras.pageSize !== 20 ? { page_size: extras.pageSize } : {}),
  }
}

export function requirementPoolFilterChipsEqual(
  left: RequirementPoolFilterValue,
  right: RequirementPoolFilterValue,
) {
  return JSON.stringify(requirementPoolSearchFromFilters({ ...left, q: '' }))
    === JSON.stringify(requirementPoolSearchFromFilters({ ...right, q: '' }))
}

export function requirementMatchesSearchFilter(
  requirement: { id: number; title: string },
  q: string,
) {
  const search = q.trim().toLowerCase()
  if (!search) return true
  return String(requirement.id).includes(search) || requirement.title.toLowerCase().includes(search)
}

export function requirementMatchesScopeFilter(
  requirement: { project_new_id: number; version_task_new_id?: number | null },
  filters: RequirementPoolFilterValue,
  planUnits: ReadonlyArray<{ id: number; parentId?: number }>,
) {
  const query = requirementPoolScopeQuery(filters, planUnits)
  if (!query.projectId && !query.planUnit) return true
  const fullSets = new Set((query.projectId ?? '').split(',').filter(Boolean).map(Number))
  const units = (query.planUnit ?? '').split(',').filter(Boolean)
  const unscheduled = units.includes(REQUIREMENT_POOL_UNSCHEDULED)
  const unitIds = new Set(units.filter((id) => id !== REQUIREMENT_POOL_UNSCHEDULED).map(Number))
  if (fullSets.has(requirement.project_new_id)) return true
  const versionId = Number(requirement.version_task_new_id ?? 0)
  if (versionId > 0 && unitIds.has(versionId)) return true
  return unscheduled && versionId <= 0
}

export function requirementMatchesOwnerFilter(
  requirement: Parameters<typeof requirementRoleOwners>[0],
  filters: RequirementPoolFilterValue,
) {
  const owners = requirementRoleOwners(requirement)
  const selected: Array<[readonly number[], readonly number[]]> = [
    [filters.productOwnerUserIds, owners.product_owner_user_ids],
    [filters.designOwnerUserIds, owners.design_owner_user_ids],
    [filters.frontendOwnerUserIds, owners.frontend_owner_user_ids],
    [filters.backendOwnerUserIds, owners.backend_owner_user_ids],
    [filters.testerUserIds, owners.tester_user_ids],
  ]
  return selected.every(([want, have]) => want.length === 0 || want.some((id) => have.includes(id)))
}