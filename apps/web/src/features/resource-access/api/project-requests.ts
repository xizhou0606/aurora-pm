import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import type { ProjectGuideTarget } from './project-guide'
export type JoinTarget = ProjectGuideTarget | { kind: 'workspace'; id: number }
export type ProjectJoinRequest = { id: number; applicant_user_id: number; applicant_name: string; reason: string; status: 'pending' | 'approved' | 'rejected' | 'expired' | 'revoked' }
export type ProjectJoinContext = { workspace_id: number; plan_unit_id: number; project_set_id: number; can_review: boolean; can_request: boolean; has_access: boolean; workspace_member: boolean; has_more: boolean; items: ProjectJoinRequest[] }
const root = (target: JoinTarget) => target.kind === 'workspace' ? `/workspace-access-requests/${target.id}` : `/project-access-guide/${target.kind}/${target.id}/requests`
export async function getProjectRequests(target: JoinTarget, signal?: AbortSignal): Promise<ProjectJoinContext> {
  try {
    const data = unwrapApiData<ProjectJoinContext>(await platformApi.get(root(target), { signal }))
    if (!data || !Number.isSafeInteger(data.workspace_id) || data.workspace_id <= 0 || (target.kind !== 'workspace' && (!Number.isSafeInteger(data.plan_unit_id) || data.plan_unit_id <= 0)) || !Array.isArray(data.items) || ['can_review', 'can_request', 'has_access', 'workspace_member'].some(key => typeof data[key as keyof ProjectJoinContext] !== 'boolean')) throw new Error('invalid_project_requests')
    if (data.items.some(item => !Number.isSafeInteger(item.id) || !['pending','approved','rejected','expired','revoked'].includes(item.status) || typeof item.reason !== 'string' || typeof item.applicant_name !== 'string')) throw new Error('invalid_project_request')
    return data
  } catch (error) { throw toSafeApiError(error, 'PROJECT_ACCESS_REQUEST_FAILED') }
}
export async function mutateProjectRequest(target: JoinTarget, input: { reason: string } | { id: number; decision: 'approved' | 'rejected' }): Promise<void> {
  try {
    if ('reason' in input) await platformApi.post(root(target), input)
    else await platformApi.patch(`${root(target)}/${input.id}`, { decision: input.decision })
  } catch (error) { throw toSafeApiError(error, 'PROJECT_ACCESS_REQUEST_FAILED') }
}
