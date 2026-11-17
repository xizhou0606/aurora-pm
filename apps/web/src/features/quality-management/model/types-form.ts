import type { QualityDefectDomain, QualityMaterial } from './types-entity'
import type { QualityMutationState } from './types-mutation'

export type QualityBugDraft = {
  requirementId?: number
  title: string
  status: string
  priority: string
  bugType: string
  defectDomain: QualityDefectDomain
  dueDate: string
  severity: string
  foundEnvironment: string
  operatingSystem: string
  browser: string
  assigneeUserId?: number
  reviewerUserId?: number
  resolutionReason: string
  resolutionNote: string
  reproductionSteps: string
  expectedResult: string
  actualResult: string
  reproductionMaterials: QualityMaterial[]
  sourceExecutionId?: number
  sourceTestId?: number
}

export type QualitySceneOptionKind = 'environment' | 'operating_system' | 'browser'

export type QualitySceneOption = {
  id?: number
  kind?: QualitySceneOptionKind
  name: string
  builtIn: boolean
  legacy: boolean
  usageCount?: number
}

export type QualitySceneOptionsState =
  | { status: 'loading'; rows: readonly QualitySceneOption[] }
  | { status: 'error'; rows: readonly QualitySceneOption[]; retry: () => Promise<void> }
  | { status: 'ready'; rows: readonly QualitySceneOption[] }

export type QualitySceneOptionCatalogs = Readonly<Record<
  QualitySceneOptionKind,
  QualitySceneOptionsState
>>

export type QualitySceneOptionMutations = {
  canCreate: boolean
  canDelete: (option: QualitySceneOption) => boolean
  createState: QualityMutationState
  deleteState: QualityMutationState
  create: (input: { kind: QualitySceneOptionKind; name: string }) => Promise<QualitySceneOption>
  remove: (input: { kind: QualitySceneOptionKind; option: QualitySceneOption }) => Promise<{
    kind: QualitySceneOptionKind
    id: number
  }>
  reset: () => void
}

export type QualityEnvironmentOption = QualitySceneOption
export type QualityEnvironmentOptionsState = QualitySceneOptionsState

export type QualityCreatePayload = {
  requirement_id: number
  title: string
  status: string
  priority: string
  priority_key: string
  bug_type: string
  defect_domain: QualityDefectDomain
  due_date: string | null
  severity_key: string
  found_environment: string
  operating_system: string
  browser: string
  assignee_user_id: number | null
  reviewer_user_id: number | null
  resolution_reason: string
  resolution_note: string
  reproduction_steps: string
  expected_result: string
  actual_result: string
  reproduction_materials: QualityMaterial[]
  source_execution_id: number | null
  source_test_id: number | null
}

export type QualityEditPayload = Omit<
  QualityCreatePayload,
  'source_execution_id' | 'source_test_id'
>

export type QualityAssignPayload = {
  assignee_user_id: number
  defect_owner_user_id: number
  cc_user_ids: number[]
  copy_user_ids: number[]
  assignment_note: string
  assignment_materials: QualityMaterial[]
  remark: string
  note: string
}

export type QualityResolvePayload = {
  status: 'review' | 'done'
  resolution_reason: string
  resolution_note: string
  remark: string
  note: string
  resolution_materials: QualityMaterial[]
  reviewer_user_id: number | null
  verified_by_user_id: number | null
}

export type QualityReturnPayload = {
  status: 'reopened'
  regression_note: string
  regression_materials: QualityMaterial[]
}
