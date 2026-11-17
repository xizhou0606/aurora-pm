import { keepPreviousData, queryOptions, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { pmKeys } from '@/shared/query'
import {
  isSafeApiError,
  normalizePlatformUsers,
  normalizePMScopeOptions,
  PM_API_ERROR_CODES,
  PMSafeApiError,
  platformApi,
  requestPMScopeOptions,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
} from '@/shared/api'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'
import { aipmClient } from '../api/client'
import { useAIPMClient } from '../components/environment'
import { useRequirementsOperationAccess } from '../hooks/useOperationAccess'
import type { ProjectOption, Requirement, RequirementPatch, RequirementPoolFilters, RequirementPoolPage, RequirementPoolSummary, WorkflowAssignee } from '../model/types'

export const keys = {
  requirements: (workspaceId: number, filters: Record<string, unknown>) => ['ai-pm', workspaceId, 'requirements', filters] as const,
  requirement: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'requirement', id] as const,
  requirementScope: (workspaceId: number) => ['ai-pm', workspaceId, 'requirement-scope'] as const,
  projects: (workspaceId: number) => ['ai-pm', workspaceId, 'projects'] as const,
  units: (workspaceId: number, projectId: number) => ['ai-pm', workspaceId, 'units', projectId] as const,
}

async function safe<T>(work: () => Promise<T>, code: string): Promise<T> {
  try { return await work() } catch (error) { throw toSafeApiError(error, code) }
}

function requiredData<T>(response: unknown): T {
  const value = unwrapApiData<T>(response)
  if (value === undefined) throw new Error('PM API returned no data')
  return value
}

function requireOperation(allowed: boolean): void {
  if (!allowed) throw new PMSafeApiError('ACTION_PERMISSION_DENIED', 403)
}

function requirementPatchAssignsOwners(body: Omit<RequirementPatch, 'id'>): boolean {
  return Object.keys(body).some((key) => key.endsWith('_owner_user_id') || key.endsWith('_owner_user_ids') || key === 'tester_user_ids' || key === 'developer_user_ids')
}

function requirementPatchOnlyAssignsOwners(body: Omit<RequirementPatch, 'id'>): boolean {
  const keys = Object.keys(body).filter((key) => key !== 'expected_version')
  return keys.length > 0 && keys.every((key) => key.endsWith('_owner_user_id') || key.endsWith('_owner_user_ids') || key === 'tester_user_ids' || key === 'developer_user_ids')
}

// Shared with the route loader: prefetching must populate the exact cache
// entry the pool page reads on mount.
export function requirementPoolQueryOptions(workspaceId: number, filters: RequirementPoolFilters) {
  return queryOptions({
    queryKey: keys.requirements(workspaceId, filters),
    staleTime: 15_000,
    queryFn: ({ signal }) => safe(async () => {
      const response = await platformApi.get('/requirements', {
        params: {
          q: filters.q || undefined,
          project_new_id: filters.projectId || undefined,
          version_task_new_id: filters.planUnit || undefined,
          status: filters.status || undefined,
          priority: filters.priority || undefined,
          current_node_id: filters.currentNodeId || undefined,
          product_owner_user_id: filters.productOwnerUserId || undefined,
          design_owner_user_id: filters.designOwnerUserId || undefined,
          frontend_owner_user_id: filters.frontendOwnerUserId || undefined,
          backend_owner_user_id: filters.backendOwnerUserId || undefined,
          tester_user_id: filters.testerUserId || undefined,
          limit: filters.limit,
          offset: filters.limit === 'all' ? undefined : filters.offset,
        },
        signal,
      })
      const envelope = response.data as {
        summary?: Partial<RequirementPoolSummary>
        users?: unknown
      }
      const total = unwrapApiTotal(response)
      return {
        rows: unwrapApiRows<Requirement>(response),
        users: normalizePlatformUsers(envelope.users) as WorkflowAssignee[],
        total,
        summary: {
          total: Number(envelope.summary?.total ?? total),
          unscheduled: Number(envelope.summary?.unscheduled ?? 0),
          delivery: Number(envelope.summary?.delivery ?? 0),
        },
      }
    }, 'REQUIREMENT_POOL_FAILED'),
  })
}

export function requirementScopeQueryOptions(workspaceId: number) {
  return queryOptions({
    queryKey: keys.requirementScope(workspaceId),
    staleTime: 30_000,
    queryFn: ({ signal }) => safe(async () => (
      normalizePMScopeOptions(await requestPMScopeOptions<unknown>(signal))
    ), 'REQUIREMENT_SCOPE_FAILED'),
  })
}

export function useRequirementPool(workspaceId: number, filters: RequirementPoolFilters) {
  const access = useRequirementsOperationAccess()
  return useQuery({
    ...requirementPoolQueryOptions(workspaceId, filters),
    enabled: workspaceId > 0 && access.canViewPool(),
    placeholderData: keepPreviousData,
    refetchInterval: (query) => query.state.data?.rows.some(
      (row) => row.deliverable_overview?.requirement_cover?.status === 'pending',
    ) ? 4000 : false,
    refetchIntervalInBackground: false,
  })
}

export function useRequirementCover(
  workspaceId: number,
  requirementId: number,
  generation: number | undefined,
  enabled = true,
) {
  const access = useRequirementsOperationAccess()
  return useQuery<Blob | null>({
    queryKey: ['requirement-cover', workspaceId, requirementId, generation ?? 0],
    enabled: enabled && workspaceId > 0 && requirementId > 0 && Boolean(generation) && access.canViewPool(),
    retry: false,
    staleTime: 10 * 60_000,
    gcTime: 10 * 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    queryFn: async ({ signal }) => {
      try {
        const response = await platformApi.get<Blob>(`/requirements/${requirementId}/cover`, { responseType: 'blob', signal })
        return response.data instanceof Blob && response.data.type === 'image/jpeg' ? response.data : null
      } catch (error) {
        if (toSafeApiError(error, 'REQUIREMENT_COVER_FAILED').status === 404) return null
        throw error
      }
    },
  })
}

export function useRequirementScope(workspaceId: number) {
  return useQuery({
    ...requirementScopeQueryOptions(workspaceId),
    enabled: workspaceId > 0,
  })
}

export function useRequirement(workspaceId: number, id: number) {
  const api = useAIPMClient()
  const access = useRequirementsOperationAccess()
  return useQuery<Requirement>({ queryKey: keys.requirement(workspaceId, id), enabled: workspaceId > 0 && id > 0 && access.canViewPool(), queryFn: ({ signal }) => api.requirement(id, signal) })
}

export function useProjects(workspaceId: number) {
  return useQuery({ queryKey: keys.projects(workspaceId), enabled: workspaceId > 0, queryFn: ({ signal }) => safe(async () => unwrapApiRows<ProjectOption>(await platformApi.get('/project-sets', { params: { limit: 200 }, signal })), 'PROJECTS_FAILED') })
}

export function usePlanUnits(workspaceId: number, projectId: number) {
  const api = useAIPMClient()
  return useQuery({ queryKey: keys.units(workspaceId, projectId), enabled: workspaceId > 0 && projectId > 0, queryFn: ({ signal }) => api.planUnits(projectId, signal) })
}

export function prefetchRequirement(client: QueryClient, workspaceId: number, id: number) {
  if (workspaceId <= 0 || id <= 0) return
  void client.prefetchQuery({
    queryKey: keys.requirement(workspaceId, id),
    queryFn: ({ signal }) => aipmClient.requirement(id, signal),
  })
}

export function useRequirementMutations(workspaceId: number) {
  const client = useQueryClient()
  const access = useRequirementsOperationAccess()
  const invalidateRequirements = () => client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirements'] })
  const invalidateAfterLocationChange = () => {
    void invalidateRequirements()
    void client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirement'] })
    void client.invalidateQueries({ queryKey: keys.requirementScope(workspaceId) })
    if (workspaceId > 0) {
      void client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'requirements') })
      void client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'planUnits') })
    }
  }
  return {
    create: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.create), mutationFn: (body: { project_new_id: number; version_task_new_id: number; workflow_template_id: number; internal_document?: { resource_id: string; expected_policy_revision: number; operation_id: string }; title: string; priority?: string; description?: string; product_doc_url?: string; product_owner_user_ids?: number[]; design_owner_user_ids?: number[]; frontend_owner_user_ids?: number[]; backend_owner_user_ids?: number[]; tester_user_ids?: number[] }) => safe(async () => { requireOperation(access.canCreateRequirement()); return requiredData<Requirement>(await platformApi.post('/requirements', body)) }, 'REQUIREMENT_CREATE_FAILED'), onSuccess: invalidateRequirements }),
    patch: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.save), mutationFn: ({ id, ...body }: RequirementPatch) => safe(async () => {
      const assignsOwners = requirementPatchAssignsOwners(body)
      if (assignsOwners) requireOperation(access.canAssignRoleOwners())
      if (!requirementPatchOnlyAssignsOwners(body)) requireOperation(access.canUpdateRequirement())
      const endpoint = requirementPatchOnlyAssignsOwners(body)
        ? `/requirements/${id}/role-owners`
        : `/requirements/${id}`
      return requiredData<Requirement>(await platformApi.patch(endpoint, body))
    }, 'REQUIREMENT_UPDATE_FAILED'), onSuccess: invalidateRequirements }),
    move: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.move),
      mutationFn: ({ id, version_task_new_id, expected_version, confirm_subject_schedule }: {
        id: number
        version_task_new_id: number
        expected_version: number
        confirm_subject_schedule?: true
      }) => safe(async () => { requireOperation(access.canBatchMoveRequirement()); return requiredData<{ moved_count: number; batch_id: string }>(await platformApi.post('/requirements/batch-move', {
        items: [{ requirement_id: id, expected_version }],
        version_task_new_id,
        ...(confirm_subject_schedule ? { confirm_subject_schedule } : {}),
      })) }, 'REQUIREMENT_MOVE_FAILED'),
      onSuccess: invalidateAfterLocationChange,
    }),
    remove: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.remove),
      mutationFn: (id: number) => safe(async () => {
        requireOperation(access.canDeleteRequirement())
        await platformApi.delete(`/requirements/${id}`)
      }, 'REQUIREMENT_DELETE_FAILED'),
      onSuccess: invalidateAfterLocationChange,
    }),
    schedule: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.schedule),
      mutationFn: ({ id, version_task_new_id, expected_version, confirm_subject_schedule }: { id: number; version_task_new_id: number; expected_version: number; confirm_subject_schedule?: true }) => safe(async () => { requireOperation(access.canScheduleRequirement()); return requiredData<Requirement>(await platformApi.post(`/requirements/${id}/schedule`, { version_task_new_id, expected_version, ...(confirm_subject_schedule ? { confirm_subject_schedule } : {}) })) }, 'REQUIREMENT_SCHEDULE_FAILED'),
      onSuccess: invalidateRequirements,
      onError: async (error, { id }) => {
        if (!isSafeApiError(error) || error.code !== PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE) return
        await client.refetchQueries({ queryKey: keys.requirement(workspaceId, id), type: 'active' })
        await client.invalidateQueries({
          queryKey: keys.requirementScope(workspaceId),
          refetchType: 'active',
        })
      },
    }),
    validateFeishu: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.validateDocument), mutationFn: ({ id, url }: { id: number; url: string }) => safe(async () => { requireOperation(access.canValidateDocument()); return requiredData<{ valid: boolean; fetch_enabled: boolean }>(await platformApi.post(`/requirements/${id}/feishu-document/validate`, { url })) }, 'FEISHU_VALIDATE_FAILED') }),
  }
}

// keep PreviousData re-export for backward compat
export { keepPreviousData }
