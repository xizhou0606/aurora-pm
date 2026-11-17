import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
export type ProjectGuideTarget = { kind: 'task' | 'quality_bug' | 'plan_unit'; id: number }
export type GuideMembership = { kind: 'workspace' | 'plan_unit'; id: number; status: 'joined' | 'pending' | 'requestable' | 'workspace_required' | 'unavailable' }
export type ProjectGuide = { memberships: GuideMembership[]; workspaceId: number; workspaceName: string; projectSetName: string; projectName: string; ownerName: string }
export async function getProjectGuide(target: ProjectGuideTarget, signal?: AbortSignal): Promise<ProjectGuide> {
  try {
    const dto = unwrapApiData(await platformApi.get(`/project-access-guide/${target.kind}/${target.id}`, { signal })) as Record<string, unknown>
    if (!Number.isSafeInteger(dto.workspace_id) || Number(dto.workspace_id) <= 0 || typeof dto.workspace_name !== 'string' || typeof dto.project_name !== 'string' || typeof dto.owner_name !== 'string') throw new Error('invalid_project_guide')
    const memberships = dto.memberships as GuideMembership[]
    if (!Array.isArray(memberships) || memberships.length !== 2 || new Set(memberships.map(item => item.kind)).size !== 2 || memberships.some(item => !['workspace', 'plan_unit'].includes(item.kind) || !Number.isSafeInteger(item.id) || item.id <= 0 || !['joined', 'pending', 'requestable', 'workspace_required', 'unavailable'].includes(item.status))) throw new Error('invalid_membership_states')
    return { memberships, workspaceId: Number(dto.workspace_id), workspaceName: dto.workspace_name, projectSetName: typeof dto.project_set_name === 'string' ? dto.project_set_name : '', projectName: dto.project_name, ownerName: dto.owner_name }
  } catch (error) { throw toSafeApiError(error, 'PROJECT_ACCESS_GUIDE_FAILED') }
}
