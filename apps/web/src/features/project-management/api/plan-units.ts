import {
  platformApi,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
} from '@/shared/api'
import type {
  CreateScrumVersionTaskNewPayload,
  CreateScrumVersionTaskNewResult,
  DeleteScrumVersionTaskNewPayload,
  DeleteScrumVersionTaskNewResult,
  PatchScrumVersionTaskNewPayload,
  PatchScrumVersionTaskNewResult,
  ScrumProjectDeliveryNode,
  ScrumProjectDeliveryNodeLabel,
  ScrumVersionTaskNew,
  PublishedPlanUnitWorkflowTemplate,
  PublishedWorkflowTemplate,
} from './contracts'
import {
  requireResourceMutationResult,
  resolveDeleteMutationResult,
  resolvePatchMutationResult,
} from './mutation-response'
import { clearProjectRequirementScopeOptionsCache } from './scope'

type LegacyCreateScrumVersionTaskNewPayload = {
  project_new_id: number
  title: string
  type?: string
  owner_user_id?: number | null
  planned_start_at?: string | null
  planned_end_at?: string | null
  design_freeze_due_at?: string | null
  development_complete_due_at?: string | null
  test_acceptance_due_at?: string | null
  delivery_node_labels?: ScrumProjectDeliveryNodeLabel[]
  delivery_nodes?: ScrumProjectDeliveryNode[]
  actual_start_at?: string | null
  actual_completed_at?: string | null
  risk_level?: string
  goal?: string
}

type LegacyPatchScrumVersionTaskNewPayload = Partial<{
  title: string
  type: string
  owner_user_id: number | null
  planned_start_at: string | null
  expected_planned_start_at: string | null
  planned_end_at: string | null
  expected_planned_end_at: string | null
  design_freeze_due_at: string | null
  expected_design_freeze_due_at: string | null
  development_complete_due_at: string | null
  expected_development_complete_due_at: string | null
  test_acceptance_due_at: string | null
  expected_test_acceptance_due_at: string | null
  delivery_nodes: ScrumProjectDeliveryNode[]
  actual_start_at: string | null
  expected_actual_start_at: string | null
  actual_completed_at: string | null
  expected_actual_completed_at: string | null
  risk_level: string
  goal: string
}>

export async function listScrumVersionTasksNew(projectNewId: number) {
  if (!projectNewId) return [] as ScrumVersionTaskNew[]
  const response = await platformApi.get('/project-new', {
    params: { project_new_id: String(projectNewId) },
  })
  return unwrapApiRows<ScrumVersionTaskNew>(response).filter(
    (row) => Number(row.id) > 0,
  )
}

export async function listPublishedWorkflowTemplates(
  subjectType: 'requirement' | 'plan_unit',
  signal?: AbortSignal,
) {
  const response = await platformApi.get('/pm/workflows/templates', { params: { subject_type: subjectType }, signal })
  return unwrapApiRows<PublishedWorkflowTemplate>(response).filter((row) => (
    Number(row.id) > 0 && Number(row.published_version) > 0
  ))
}

export async function listPublishedPlanUnitWorkflowTemplates(signal?: AbortSignal) {
  return listPublishedWorkflowTemplates('plan_unit', signal) as Promise<PublishedPlanUnitWorkflowTemplate[]>
}

export async function listScrumProjectDeliveryNodeLabelsNew(planUnitId: number, signal?: AbortSignal) {
  if (!planUnitId) return [] as ScrumProjectDeliveryNodeLabel[]
  try {
    const path = `/version-tasks-new/${planUnitId}/delivery-node-labels`
    const response = signal
      ? await platformApi.get(path, { signal })
      : await platformApi.get(path)
    return unwrapApiRows<ScrumProjectDeliveryNodeLabel>(response)
  } catch (error) {
    throw toSafeApiError(error, 'DELIVERY_LABELS_FAILED')
  }
}

export async function createScrumProjectDeliveryNodeLabelNew(planUnitId: number, name: string) {
  try {
    const response = await platformApi.post(
      `/version-tasks-new/${planUnitId}/delivery-node-labels`,
      { name },
    )
    return requireResourceMutationResult(
      unwrapApiData<ScrumProjectDeliveryNodeLabel & { id: number }>(response),
      'DELIVERY_LABEL_SAVE_FAILED',
    )
  } catch (error) {
    throw toSafeApiError(error, 'DELIVERY_LABEL_SAVE_FAILED')
  }
}

export async function patchScrumProjectDeliveryNodeLabelNew(id: number, name: string) {
  try {
    const response = await platformApi.patch(`/project-delivery-node-labels/${id}`, {
      name,
    })
    return requireResourceMutationResult(
      unwrapApiData<ScrumProjectDeliveryNodeLabel & { id: number }>(response),
      'DELIVERY_LABEL_SAVE_FAILED',
    )
  } catch (error) {
    throw toSafeApiError(error, 'DELIVERY_LABEL_SAVE_FAILED')
  }
}

export async function deleteScrumProjectDeliveryNodeLabelNew(id: number) {
  try {
    const response = await platformApi.delete(`/project-delivery-node-labels/${id}`)
    return resolveDeleteMutationResult(
      unwrapApiData<{ deleted: true }>(response),
      'DELIVERY_LABEL_DELETE_FAILED',
    )
  } catch (error) {
    throw toSafeApiError(error, 'DELIVERY_LABEL_DELETE_FAILED')
  }
}

export function createScrumVersionTaskNew(
  body: CreateScrumVersionTaskNewPayload,
): Promise<CreateScrumVersionTaskNewResult>
export function createScrumVersionTaskNew(
  body: LegacyCreateScrumVersionTaskNewPayload,
): Promise<CreateScrumVersionTaskNewResult>
export async function createScrumVersionTaskNew(
  body: CreateScrumVersionTaskNewPayload | LegacyCreateScrumVersionTaskNewPayload,
): Promise<CreateScrumVersionTaskNewResult> {
  try {
    const response = await platformApi.post('/project-new', body)
    const result = requireResourceMutationResult(
      unwrapApiData<CreateScrumVersionTaskNewResult>(response),
      'PLAN_UNIT_CREATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PLAN_UNIT_CREATE_FAILED')
  }
}

export async function patchScrumVersionTaskNew(
  id: number,
  body: PatchScrumVersionTaskNewPayload | LegacyPatchScrumVersionTaskNewPayload,
): Promise<PatchScrumVersionTaskNewResult> {
  try {
    const response = await platformApi.patch(`/project-new/${id}`, body)
    const result = resolvePatchMutationResult(
      unwrapApiData<PatchScrumVersionTaskNewResult>(response),
      { id, ...body } as PatchScrumVersionTaskNewResult,
      'PLAN_UNIT_UPDATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PLAN_UNIT_UPDATE_FAILED')
  }
}

export async function deleteScrumVersionTaskNew(
  id: DeleteScrumVersionTaskNewPayload['id'],
): Promise<DeleteScrumVersionTaskNewResult> {
  try {
    const response = await platformApi.delete(`/project-new/${id}`)
    const result = resolveDeleteMutationResult(
      unwrapApiData<DeleteScrumVersionTaskNewResult>(response),
      'PLAN_UNIT_DELETE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PLAN_UNIT_DELETE_FAILED')
  }
}

export type LockPlanUnitRequirementsResult = {
  id: number
  requirements_locked: boolean
  requirements_locked_at?: string | null
  requirements_locked_by_user_id?: number | null
}

// 纯锁版：已过需求录入阶段但仍未锁版的存量项目/版本直接落锁版事实，
// 工作流状态不动。空范围或并发冲突由后端 409 返回。
export async function lockPlanUnitRequirements(
  id: number,
): Promise<LockPlanUnitRequirementsResult> {
  try {
    const response = await platformApi.post(`/project-new/${id}/requirements-lock`)
    return requireResourceMutationResult(
      unwrapApiData<LockPlanUnitRequirementsResult>(response),
      'PLAN_UNIT_REQUIREMENTS_LOCK_FAILED',
    )
  } catch (error) {
    throw toSafeApiError(error, 'PLAN_UNIT_REQUIREMENTS_LOCK_FAILED')
  }
}
