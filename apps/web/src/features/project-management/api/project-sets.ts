import {
  platformApi,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
} from '@/shared/api'
import type {
  CreateScrumProjectNewPayload,
  CreateScrumProjectNewResult,
  DeleteScrumProjectNewPayload,
  DeleteScrumProjectNewResult,
  ListScrumProjectsNewInput,
  PatchScrumProjectNewPayload,
  PatchScrumProjectNewResult,
  ProjectMembersPage,
  PlanUnitMembersSnapshot,
  PutMembersPayload,
  RemovalImpact,
  ScrumProjectAuditLogNew,
  ScrumProjectNewsNew,
  ScrumProjectNew,
} from './contracts'
import {
  requireResourceMutationResult,
  resolveDeleteMutationResult,
  resolvePatchMutationResult,
} from './mutation-response'
import { clearProjectRequirementScopeOptionsCache } from './scope'

type LegacyCreateScrumProjectNewPayload = {
  team_id: number
  name: string
  owner_user_id?: number | null
  planned_start_at?: string | null
  planned_end_at?: string | null
  description?: string
  status?: string
}

type LegacyPatchScrumProjectNewPayload = Partial<{
  team_id: number
  name: string
  planned_start_at: string | null
  expected_planned_start_at: string | null
  planned_end_at: string | null
  expected_planned_end_at: string | null
  description: string
  status: string
}>

export async function listScrumProjectsNew(
  input: ListScrumProjectsNewInput,
  signal?: AbortSignal,
) {
  const params: Record<string, string> = {
    limit: String(input.limit ?? 20),
    offset: String(input.offset ?? 0),
    sort: 'updated_at_desc',
  }
  if (input.teamId) params.team_id = String(input.teamId)
  if (input.q?.trim()) params.q = input.q.trim()
  try {
    const response = await platformApi.get('/project-sets', { params, signal })
    const rows = unwrapApiRows<ScrumProjectNew>(response)
    return {
      rows: rows.filter((row) => Number(row.id) > 0),
      total: unwrapApiTotal(response),
    }
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_SETS_FAILED')
  }
}

export async function listScrumProjectAuditLogsNew(
  projectNewId: number,
  input: { limit?: number; offset?: number } = {},
  signal?: AbortSignal,
) {
  try {
    const response = await platformApi.get(`/project-sets/${projectNewId}/audit-logs`, {
      params: {
        limit: String(input.limit ?? 30),
        offset: String(input.offset ?? 0),
      },
      signal,
    })
    return {
      rows: unwrapApiRows<ScrumProjectAuditLogNew>(response),
      total: unwrapApiTotal(response),
    }
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_AUDIT_FAILED')
  }
}

export async function listScrumProjectNewsNew(
  projectNewId: number,
  signal?: AbortSignal,
) {
  try {
    const response = await platformApi.get(`/project-sets/${projectNewId}/news`, {
      params: { limit: '1' },
      signal,
    })
    return unwrapApiRows<ScrumProjectNewsNew>(response).filter((row) => (
      Number(row.id) > 0 && Number(row.project_set_id) === projectNewId
    ))
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_NEWS_FAILED')
  }
}

export function createScrumProjectNew(
  body: CreateScrumProjectNewPayload,
): Promise<CreateScrumProjectNewResult>
export function createScrumProjectNew(
  body: LegacyCreateScrumProjectNewPayload,
): Promise<CreateScrumProjectNewResult>
export async function createScrumProjectNew(
  body: CreateScrumProjectNewPayload | LegacyCreateScrumProjectNewPayload,
): Promise<CreateScrumProjectNewResult> {
  try {
    const response = await platformApi.post('/project-sets', body)
    const result = requireResourceMutationResult(
      unwrapApiData<CreateScrumProjectNewResult>(response),
      'PROJECT_CREATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_CREATE_FAILED')
  }
}

export async function patchScrumProjectNew(
  id: number,
  body: PatchScrumProjectNewPayload | LegacyPatchScrumProjectNewPayload,
): Promise<PatchScrumProjectNewResult> {
  const {
    team_id: _legacyTeamId,
    owner_user_id: _legacyOwnerUserId,
    ...patch
  } = body as LegacyPatchScrumProjectNewPayload & {
    owner_user_id?: number | null
  }
  try {
    const response = await platformApi.patch(`/project-sets/${id}`, patch)
    const result = resolvePatchMutationResult(
      unwrapApiData<PatchScrumProjectNewResult>(response),
      { id, ...patch } as PatchScrumProjectNewResult,
      'PROJECT_UPDATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_UPDATE_FAILED')
  }
}

export async function deleteScrumProjectNew(
  id: DeleteScrumProjectNewPayload['id'],
): Promise<DeleteScrumProjectNewResult> {
  try {
    const response = await platformApi.delete(`/project-sets/${id}`)
    const result = resolveDeleteMutationResult(
      unwrapApiData<DeleteScrumProjectNewResult>(response),
      'PROJECT_DELETE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_DELETE_FAILED')
  }
}

export async function transferScrumProjectOwnerNew(
  id: number,
  body: {
    new_owner_user_id: number
    keep_previous_owner_as_sub_admin: boolean
    transfer_reason?: string
  },
) {
  try {
    const response = await platformApi.post(`/project-sets/${id}/transfer-owner-new`, body)
    const result = requireResourceMutationResult(
      unwrapApiData<ScrumProjectNew>(response),
      'PROJECT_OWNER_TRANSFER_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_OWNER_TRANSFER_FAILED')
  }
}

// Plan unit (project/version item) members API: owner + participants are
// version-level facts under /project-new.
export async function getProjectMembers(
  planUnitId: number,
  params?: { q?: string; page?: number; page_size?: number },
  signal?: AbortSignal,
): Promise<ProjectMembersPage> {
  try {
    const response = await platformApi.get(`/project-new/${planUnitId}/members`, {
      params: {
        q: params?.q ?? '',
        page: String(params?.page ?? 1),
        page_size: String(params?.page_size ?? 50),
      },
      signal,
    })
    const result = unwrapApiData<ProjectMembersPage>(response)
    if (!result || !Array.isArray(result.items)) throw new Error('invalid members response')
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_MEMBERS_FAILED')
  }
}

export async function putProjectMembers(
  planUnitId: number,
  body: PutMembersPayload,
): Promise<PlanUnitMembersSnapshot> {
  try {
    const response = await platformApi.put(`/project-new/${planUnitId}/members`, body)
    const result = unwrapApiData<PlanUnitMembersSnapshot>(response)
    if (!result || result.plan_unit_id == null) throw new Error('invalid members response')
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_MEMBERS_UPDATE_FAILED')
  }
}

export async function getProjectMemberRemovalImpact(
  planUnitId: number,
  userId: number,
  signal?: AbortSignal,
): Promise<RemovalImpact> {
  try {
    const response = await platformApi.get(
      `/project-new/${planUnitId}/members/${userId}/removal-impact`,
      { signal },
    )
    const result = unwrapApiData<RemovalImpact>(response)
    if (!result || !Array.isArray(result.responsible_items)) throw new Error('invalid impact response')
    return result
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_MEMBER_IMPACT_FAILED')
  }
}
