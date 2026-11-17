import {
  coreApi,
  platformApi,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
  type PlatformUserPage,
  type PlatformUserQuery,
} from '@/shared/api'
import type {
  AICapability,
  AIRun,
  PlanUnitOption,
  Requirement,
  WorkflowAssignee,
  WorkflowInstance,
  WorkflowNodeArtifact,
  WorkflowNodeOpinion,
  WorkflowTemplate,
} from '../model/types'

export type AIPMClient = {
  requirement(id: number, signal?: AbortSignal): Promise<Requirement>
  planUnits(projectId: number, signal?: AbortSignal): Promise<PlanUnitOption[]>
  workflowTemplate(id: number, signal?: AbortSignal): Promise<WorkflowTemplate>
  aiCapabilities(signal?: AbortSignal): Promise<{ enabled: boolean; model: string; capabilities: AICapability[] }>
  aiRuns(subjectType?: string, subjectId?: number, signal?: AbortSignal): Promise<{ rows: AIRun[]; total: number }>
  workflowInstance(id: number, signal?: AbortSignal): Promise<WorkflowInstance>
  workflowInstances(
    subjectType: 'requirement' | 'plan_unit',
    subjectId: number,
    signal?: AbortSignal,
  ): Promise<WorkflowInstance[]>
  workflowAssignees(query: PlatformUserQuery, signal?: AbortSignal): Promise<PlatformUserPage>
  testCaseArtifacts(instanceId: number, nodeId: string, signal?: AbortSignal): Promise<WorkflowNodeArtifact[]>
  testReportArtifacts(instanceId: number, nodeId: string, signal?: AbortSignal): Promise<WorkflowNodeArtifact[]>
  nodeOpinions(instanceId: number, nodeId: string, signal?: AbortSignal): Promise<WorkflowNodeOpinion[]>
}

async function safe<T>(work: () => Promise<T>, code: string): Promise<T> {
  try {
    return await work()
  } catch (error) {
    throw toSafeApiError(error, code)
  }
}

function requiredData<T>(response: unknown): T {
  const value = unwrapApiData<T>(response)
  if (value === undefined) throw new Error('PM API returned no data')
  return value
}

export const aipmClient: AIPMClient = {
  requirement: (id, signal) => safe(async () => requiredData<Requirement>(
    await platformApi.get(`/requirements/${id}`, { signal }),
  ), 'REQUIREMENT_FAILED'),
  planUnits: (projectId, signal) => safe(async () => unwrapApiRows<PlanUnitOption>(
    await platformApi.get('/project-new', {
      params: { project_new_id: projectId, limit: 200 },
      signal,
    }),
  ), 'PLAN_UNITS_FAILED'),
  workflowTemplate: (id, signal) => safe(async () => requiredData<WorkflowTemplate>(
    await platformApi.get(`/pm/workflows/templates/${id}`, { signal }),
  ), 'WORKFLOW_FAILED'),
  aiCapabilities: (signal) => safe(async () => requiredData<{ enabled: boolean; model: string; capabilities: AICapability[] }>(
    await platformApi.get('/ai/capabilities', { signal }),
  ), 'AI_CAPABILITIES_FAILED'),
  aiRuns: async (subjectType, subjectId, signal) => safe(async () => {
    const response = await platformApi.get('/ai/runs', {
      params: { subject_type: subjectType || undefined, subject_id: subjectId || undefined, limit: 100 },
      signal,
    })
    return { rows: unwrapApiRows<AIRun>(response), total: unwrapApiTotal(response) }
  }, 'AI_RUNS_FAILED'),
  workflowInstance: (id, signal) => safe(async () => requiredData<WorkflowInstance>(
    await platformApi.get(`/pm/workflows/instances/${id}`, { signal }),
  ), 'WORKFLOW_INSTANCE_FAILED'),
  workflowInstances: (subjectType, subjectId, signal) => safe(async () => unwrapApiRows<WorkflowInstance>(
    await platformApi.get('/pm/workflows/instances', {
      params: { subject_type: subjectType, subject_id: subjectId },
      signal,
    }),
  ), 'WORKFLOW_INSTANCES_FAILED'),
  workflowAssignees: (query, signal) => safe(async () => {
    const response = await coreApi.get('/directory/users', {
      params: {
        ...(query.q?.trim() ? { q: query.q.trim() } : {}),
        ...(query.ids?.length ? { ids: query.ids.join(',') } : {}),
        limit: query.limit ?? 20,
        offset: query.offset ?? 0,
      },
      signal,
    })
    const items = unwrapApiRows<WorkflowAssignee>(response)
      .filter((user) => Number.isInteger(Number(user.id)) && Number(user.id) > 0)
    return { items, total: unwrapApiTotal(response) || items.length }
  }, 'WORKFLOW_ASSIGNEES_FAILED'),
  testCaseArtifacts: (instanceId, nodeId, signal) => safe(async () => unwrapApiRows<WorkflowNodeArtifact>(
    await platformApi.get(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/test-case-artifacts`, { signal }),
  ), 'WORKFLOW_TEST_CASE_ARTIFACTS_FAILED'),
  testReportArtifacts: (instanceId, nodeId, signal) => safe(async () => unwrapApiRows<WorkflowNodeArtifact>(
    await platformApi.get(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/test-report-artifacts`, { signal }),
  ), 'WORKFLOW_TEST_REPORT_ARTIFACTS_FAILED'),
  nodeOpinions: (instanceId, nodeId, signal) => safe(async () => unwrapApiRows<WorkflowNodeOpinion>(
    await platformApi.get(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/opinions`, { signal }),
  ), 'WORKFLOW_NODE_OPINIONS_FAILED'),
}
