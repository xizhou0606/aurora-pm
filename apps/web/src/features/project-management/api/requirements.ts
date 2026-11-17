import {
  platformApi,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
} from '@/shared/api'
import type {
  CreateScrumProductRequirementPayload,
  CreateScrumProductRequirementResult,
  DeleteScrumProductRequirementResult,
  ListScrumRequirementsInput,
  PatchScrumProductRequirementResult,
  ScrumPagedRows,
  ScrumProductRequirementNew,
  ScrumProductRequirementPatch,
} from './contracts'
import {
  requireResourceMutationResult,
  resolveDeleteMutationResult,
  resolvePatchMutationResult,
} from './mutation-response'
import { clearProjectRequirementScopeOptionsCache } from './scope'
import { listResponseUsers } from './users'

export async function listScrumProductRequirementsNew(
  input: ListScrumRequirementsInput | number,
  signal?: AbortSignal,
): Promise<ScrumPagedRows<ScrumProductRequirementNew>> {
  const normalized: ListScrumRequirementsInput = typeof input === 'number'
    ? { versionTaskNewId: input }
    : input
  if (!normalized.projectNewId && !normalized.versionTaskNewId) return { rows: [], total: 0, users: [] }
  const params: Record<string, string> = {
    limit: String(normalized.limit ?? 50),
    offset: String(normalized.offset ?? 0),
  }
  if (normalized.projectNewId) params.project_new_id = String(normalized.projectNewId)
  if (normalized.versionTaskNewId) params.version_task_new_id = String(normalized.versionTaskNewId)
  if ('q' in normalized && normalized.q?.trim()) params.q = normalized.q.trim()
  if ('status' in normalized && normalized.status) params.status = normalized.status
  if ('priority' in normalized && normalized.priority) params.priority = normalized.priority
  try {
    const response = await platformApi.get('/product-requirements-new', { params, signal })
    const rows = unwrapApiRows<ScrumProductRequirementNew>(response)
    return {
      rows: rows.filter((row) => Number(row.id) > 0),
      total: unwrapApiTotal(response),
      users: listResponseUsers(response),
    }
  } catch (error) {
    throw toSafeApiError(error, 'REQUIREMENTS_FAILED')
  }
}

export async function createScrumProductRequirementNew(
  body: CreateScrumProductRequirementPayload,
): Promise<CreateScrumProductRequirementResult> {
  try {
    const response = await platformApi.post('/product-requirements-new', body)
    const result = requireResourceMutationResult(
      unwrapApiData<CreateScrumProductRequirementResult>(response),
      'REQUIREMENT_CREATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'REQUIREMENT_CREATE_FAILED')
  }
}

export async function patchScrumProductRequirementNew(
  id: number,
  body: ScrumProductRequirementPatch,
): Promise<PatchScrumProductRequirementResult> {
  try {
    const response = await platformApi.patch(`/product-requirements-new/${id}`, body)
    const { expected_version: expectedVersion, ...resourcePatch } = body
    const result = resolvePatchMutationResult(
      unwrapApiData<PatchScrumProductRequirementResult>(response),
      {
        id,
        ...resourcePatch,
        version: expectedVersion + 1,
      } as PatchScrumProductRequirementResult,
      'REQUIREMENT_UPDATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'REQUIREMENT_UPDATE_FAILED')
  }
}

export async function deleteScrumProductRequirementNew(
  id: number,
): Promise<DeleteScrumProductRequirementResult> {
  try {
    const response = await platformApi.delete(`/product-requirements-new/${id}`)
    const result = resolveDeleteMutationResult(
      unwrapApiData<DeleteScrumProductRequirementResult>(response),
      'REQUIREMENT_DELETE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'REQUIREMENT_DELETE_FAILED')
  }
}
