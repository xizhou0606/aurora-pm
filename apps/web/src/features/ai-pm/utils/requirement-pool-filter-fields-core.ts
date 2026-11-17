import { copy } from '@/shared/copy'
import type { TableColumnOption } from '@/shared/model/table-column-preferences'
import { requirementRoleLanes } from '../utils/requirement-role-owners'

const poolCopy = copy.aiPM.requirementPool

export const REQUIREMENT_POOL_NODE_IDLE = '__idle__'
export const REQUIREMENT_POOL_NODE_COMPLETED = '__completed__'
export const REQUIREMENT_POOL_UNSCHEDULED = 'none'

export type RequirementPoolFilterFieldId =
  | 'scope'
  | 'workflow'
  | 'role-product'
  | 'role-ui'
  | 'role-frontend'
  | 'role-backend'
  | 'role-tester'

export type RequirementPoolFilterFieldKind = 'scope' | 'node' | 'person'

export type RequirementPoolFilterField = {
  id: RequirementPoolFilterFieldId
  columnId: string
  kind: RequirementPoolFilterFieldKind
  label: string
}

export type RequirementPoolFilterValue = {
  q: string
  projectIds: number[]
  planUnitIds: string[]
  currentNodeIds: string[]
  productOwnerUserIds: number[]
  designOwnerUserIds: number[]
  frontendOwnerUserIds: number[]
  backendOwnerUserIds: number[]
  testerUserIds: number[]
}

const roleQueryByLane = {
  product_owner_user_ids: 'product_owner_user_id',
  design_owner_user_ids: 'design_owner_user_id',
  frontend_owner_user_ids: 'frontend_owner_user_id',
  backend_owner_user_ids: 'backend_owner_user_id',
  tester_user_ids: 'tester_user_id',
} as const

export const emptyRequirementPoolFilterValue: RequirementPoolFilterValue = {
  q: '',
  projectIds: [],
  planUnitIds: [],
  currentNodeIds: [],
  productOwnerUserIds: [],
  designOwnerUserIds: [],
  frontendOwnerUserIds: [],
  backendOwnerUserIds: [],
  testerUserIds: [],
}

export const requirementPoolFilterFields: readonly RequirementPoolFilterField[] = [
  { id: 'scope', columnId: 'scope', kind: 'scope', label: poolCopy.scopeLevel },
  { id: 'workflow', columnId: 'workflow', kind: 'node', label: poolCopy.workflowOverview },
  ...requirementRoleLanes.map((lane) => ({
    id: lane.id,
    columnId: lane.id,
    kind: 'person' as const,
    label: lane.label,
  })),
]

export type RequirementPoolFilterControlId = 'search' | RequirementPoolFilterFieldId

export const REQUIREMENT_POOL_FILTER_CONTROLS_NAMESPACE = 'requirement-pool-filter-controls'

export const REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS: readonly TableColumnOption<RequirementPoolFilterControlId>[] = [
  { id: 'search', label: poolCopy.search, required: true },
  ...requirementPoolFilterFields.filter((field) => field.id !== 'scope').map((field) => ({ id: field.id, label: field.label })),
]

export function requirementPoolRoleQueryKey(fieldId: RequirementPoolFilterFieldId) {
  const lane = requirementRoleLanes.find((item) => item.id === fieldId)
  return lane ? roleQueryByLane[lane.key] : undefined
}