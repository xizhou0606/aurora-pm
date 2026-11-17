import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { platformApi, toSafeApiError, unwrapApiData, unwrapApiRows } from '@/shared/api'
import { PMSafeApiError } from '@/shared/api'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'
import { useAIPMClient } from '../components/environment'
import { useAIRunsOperationAccess } from '../hooks/useOperationAccess'
import type { AICapability, AIRun, WorkflowDefinition } from '../model/types'

export const keys = {
  capabilities: (workspaceId: number) => ['ai-pm', workspaceId, 'capabilities'] as const,
  runs: (workspaceId: number, subjectType?: string, subjectId?: number) => ['ai-pm', workspaceId, 'runs', subjectType ?? 'all', subjectId ?? 'all'] as const,
  run: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'run', id] as const,
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

export function useAICapabilities(workspaceId: number) {
  const api = useAIPMClient()
  const access = useAIRunsOperationAccess()
  return useQuery<{ enabled: boolean; model: string; capabilities: AICapability[] }>({ queryKey: keys.capabilities(workspaceId), enabled: workspaceId > 0 && access.canViewCapabilities(), queryFn: ({ signal }) => api.aiCapabilities(signal) })
}

export function useAIRuns(workspaceId: number, subjectType?: string, subjectId?: number, options: { pollActive?: boolean } = {}) {
  const api = useAIPMClient()
  const access = useAIRunsOperationAccess()
  return useQuery<{ rows: AIRun[]; total: number }>({ queryKey: keys.runs(workspaceId, subjectType, subjectId), enabled: workspaceId > 0 && access.canViewRuns(), refetchInterval: options.pollActive === false ? false : (query) => (query.state.data?.rows.some((run) => ['queued', 'running'].includes(run.status)) ? 2000 : false), queryFn: ({ signal }) => api.aiRuns(subjectType, subjectId, signal) })
}

export function useAIRun(workspaceId: number, id: number) {
  const access = useAIRunsOperationAccess()
  return useQuery<AIRun>({
    queryKey: keys.run(workspaceId, id),
    enabled: workspaceId > 0 && id > 0 && access.canViewRuns(),
    refetchInterval: (query) => !query.state.data || ['queued', 'running'].includes(query.state.data.status) ? 1500 : false,
    queryFn: ({ signal }) => safe(async () => requiredData<AIRun>(await platformApi.get(`/ai/runs/${id}`, { signal })), 'AI_RUN_FAILED'),
  })
}

export function useAIMutations(workspaceId: number) {
  const client = useQueryClient()
  const access = useAIRunsOperationAccess()
  const invalidate = () => client.invalidateQueries({ queryKey: ['ai-pm', workspaceId] })
  return {
    create: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.ai.submit), mutationFn: (body: { capability: string; subject_type: string; subject_id: number; user_prompt?: string; workflow_definition?: WorkflowDefinition; test_case_delivery?: { instance_id: number; node_id: string } }) => safe(async () => { requireOperation(access.canCreateRun()); return requiredData<AIRun>(await platformApi.post('/ai/runs', body)) }, 'AI_RUN_CREATE_FAILED'), onSuccess: invalidate }),
    apply: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.ai.apply), mutationFn: (id: number) => safe(async () => { requireOperation(access.canApplyRun()); return requiredData<AIRun>(await platformApi.post(`/ai/runs/${id}/apply`)) }, 'AI_APPLY_FAILED'), onSuccess: invalidate }),
    retry: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.ai.retry), mutationFn: (id: number) => safe(async () => { requireOperation(access.canRetryRun()); return requiredData<AIRun>(await platformApi.post(`/ai/runs/${id}/retry`)) }, 'AI_RETRY_FAILED'), onSuccess: invalidate }),
    cancel: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.ai.cancel), mutationFn: (id: number) => safe(async () => { requireOperation(access.canCancelRun()); return requiredData<AIRun>(await platformApi.post(`/ai/runs/${id}/cancel`)) }, 'AI_CANCEL_FAILED'), onSuccess: invalidate }),
    feedback: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.ai.feedback), mutationFn: ({ id, rating, comment }: { id: number; rating: 'up' | 'down'; comment?: string }) => safe(async () => { requireOperation(access.canSubmitFeedback()); return unwrapApiData(await platformApi.post(`/ai/runs/${id}/feedback`, { rating, comment })) }, 'AI_FEEDBACK_FAILED') }),
  }
}

export function useCancelAIRun(workspaceId: number) {
  const client = useQueryClient()
  const access = useAIRunsOperationAccess()
  return useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.ai.cancel),
    mutationFn: (id: number) => safe(async () => { requireOperation(access.canCancelRun()); return requiredData<AIRun>(await platformApi.post(`/ai/runs/${id}/cancel`)) }, 'AI_CANCEL_FAILED'),
    onSuccess: () => client.invalidateQueries({ queryKey: ['ai-pm', workspaceId] }),
  })
}