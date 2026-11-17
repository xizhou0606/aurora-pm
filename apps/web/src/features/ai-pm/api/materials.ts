import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { pmKeys } from '@/shared/query'
import { platformApi, toSafeApiError, unwrapApiData, unwrapApiRows } from '@/shared/api'
import { PMSafeApiError } from '@/shared/api'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'
import { useRequirementsOperationAccess, useTestingOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'
import type { RequirementMaterial, TestingLibraryItem } from '../model/types'
import { keys as poolKeys } from './pool'

export const keys = {
  requirementMaterials: (workspaceId: number, requirementId: number, kind?: string) => ['ai-pm', workspaceId, 'requirement-materials', requirementId, kind ?? 'all'] as const,
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

export function downloadWorkflowNodeArtifact(artifactId: number) {
  return safe(async () => {
    const response = await platformApi.get<Blob>(`/pm/workflow-node-artifacts/${artifactId}/download`, {
      responseType: 'blob',
    })
    return response.data
  }, 'WORKFLOW_NODE_ARTIFACT_DOWNLOAD_FAILED')
}

export function downloadRequirementMaterial(requirementId: number, materialId: number) {
  return safe(async () => {
    const response = await platformApi.get<Blob>(`/requirements/${requirementId}/materials/${materialId}/download`, {
      responseType: 'blob',
    })
    return response.data
  }, 'FILE_DOWNLOAD_FAILED')
}

export function downloadPlatformFile(url: string) {
  return safe(async () => {
    if (/^https?:\/\//i.test(url) && !url.includes('/api/')) {
      window.open(url, '_blank', 'noopener,noreferrer')
      return null
    }
    const path = url.replace(/^https?:\/\/[^/]+/, '').replace(/^\/api\/v1\/platform/, '')
    const response = await platformApi.get<Blob>(path, { responseType: 'blob' })
    return response.data
  }, 'FILE_DOWNLOAD_FAILED')
}

export function useRequirementMaterials(workspaceId: number, requirementId: number, kind?: 'design' | 'test_case') {
  const access = useRequirementsOperationAccess()
  return useQuery<RequirementMaterial[]>({
    queryKey: keys.requirementMaterials(workspaceId, requirementId, kind),
    enabled: workspaceId > 0 && requirementId > 0 && access.canViewMaterials(),
    queryFn: ({ signal }) => safe(async () => unwrapApiRows<RequirementMaterial>(await platformApi.get(`/requirements/${requirementId}/materials`, {
      params: { kind },
      signal,
    })), 'REQUIREMENT_MATERIALS_FAILED'),
  })
}

export function useWorkflowMaterialProjections(workspaceId: number, instanceId: number, kind?: 'design' | 'test_case') {
  const access = useWorkflowOperationAccess()
  return useQuery<RequirementMaterial[]>({
    queryKey: ['ai-pm', workspaceId, 'workflow-instance', instanceId, 'material-projections', kind ?? 'all'],
    enabled: workspaceId > 0 && instanceId > 0 && access.canViewMaterialProjections(),
    queryFn: ({ signal }) => safe(async () => unwrapApiRows<RequirementMaterial>(await platformApi.get(`/pm/workflows/instances/${instanceId}/material-projections`, {
      params: { kind },
      signal,
    })), 'WORKFLOW_MATERIAL_PROJECTIONS_FAILED'),
  })
}

export function useRequirementMaterialMutations(workspaceId: number) {
  const client = useQueryClient()
  const access = useRequirementsOperationAccess()
  const invalidate = (requirementId: number) => Promise.all([
    client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirement-materials', requirementId] }),
    client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'testing-library'] }),
    client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirements'] }),
    client.invalidateQueries({ queryKey: poolKeys.requirement(workspaceId, requirementId) }),
    ...(workspaceId > 0 ? [
      client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'requirements') }),
      client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'workflow-instance'] }),
    ] : []),
  ])
  return {
    upload: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.uploadMaterial),
      mutationFn: ({ requirementId, kind, file, url, source }: { requirementId: number; kind: 'design' | 'test_case'; file?: File; url?: string; source?: string }) => safe(async () => {
        requireOperation(access.canUploadMaterial())
        const formData = new FormData()
        formData.append('kind', kind)
        if (source) formData.append('source', source)
        if (url) formData.append('url', url)
        if (file) formData.append('file', file)
        return requiredData<RequirementMaterial>(await platformApi.post(`/requirements/${requirementId}/materials`, formData))
      }, 'REQUIREMENT_MATERIAL_UPLOAD_FAILED'),
      onSuccess: (row) => invalidate(row.requirement_id),
    }),
  }
}

export function useTestingLibraryMutations(workspaceId: number) {
  const client = useQueryClient()
  const access = useTestingOperationAccess()
  const invalidate = () => Promise.all([
    client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'testing-library'] }),
    client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirement-materials'] }),
    client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirements'] }),
  ])
  return {
    create: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.testingLibrary.upload),
      mutationFn: ({ kind, requirementId, planUnitId, file, url }: { kind: 'test_case' | 'test_report'; requirementId?: number; planUnitId?: number; file?: File; url?: string }) => safe(async () => {
        if (kind === 'test_case') requireOperation(access.canUploadLibraryItem())
        else if (url) requireOperation(access.canLinkVersionReport())
        else requireOperation(access.canUploadVersionReport())
        const formData = new FormData()
        formData.append('kind', kind)
        if (requirementId) formData.append('requirement_id', String(requirementId))
        if (planUnitId) formData.append('plan_unit_id', String(planUnitId))
        if (url) formData.append('url', url)
        if (file) formData.append('file', file)
        const endpoint = kind === 'test_case'
          ? '/pm/testing/items'
          : url ? '/pm/testing/version-reports/link' : '/pm/testing/version-reports/upload'
        return requiredData<TestingLibraryItem | RequirementMaterial>(await platformApi.post(endpoint, formData))
      }, 'TESTING_LIBRARY_UPLOAD_FAILED'),
      onSuccess: invalidate,
    }),
  }
}
